// Package socialnet 平台社交通道客户端 + 身份自举（C：社交通道阶梯①，MCP-lite over HTTP）单例。
//
// 让数字生命体「拥有平台身份 + 发现并使用平台 Life Network 社交通道」：
//   - 身份自举：启动时载入/生成自己的 Ed25519 私钥（私钥**只存本地数据库、永不离开容器**，
//     对齐宪法 docs/06 §5.1.2），登录用户的平台账户，再用私钥对平台挑战签名完成注册
//     （proof-of-possession 握手），拿到自己的 DID。注册幂等：已注册则跳过。
//   - 发现：GET <platform>/api/agent/manifest，看平台对 agent 暴露了哪些工具。
//   - 使用：把发现到的工具注册进慎思 lane（tools.LaneDeliberative），生命体慎思中可直接调用
//     （social.post 发布酝酿好的分享稿、social.feed/directory/follow 读流/发现/关注）。
//
// 「优先结构化通道、浏览器是最后手段」阶梯的第①层。未配置 / 平台不可达 / 注册失败 → 通道缺席
// （优雅降级：生命体照常活、社交稿先存本地），未来由 skill（②）/ 换平台（③）/ 浏览器（④）兜底。
//
// 鉴权：生命体在**归属用户的账户**下行动——用户配置账户 email/密码（MINDVERSE_PLATFORM_EMAIL/
// PASSWORD），生命体凭此登录取会话 token；token 失效（401）自动重登。生命体的 DID 由本地私钥派生。
package socialnet

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"mindverse/internal/runtime/tools"
	"mindverse/internal/storage"
)

// platformKeyMeta 本地存生命体平台身份私钥（Ed25519，64 字节 hex）的 meta 键。
// ⚠️ 私钥只存本地库、永不离开容器。
const platformKeyMeta = "platform_ed25519_key"

var (
	baseURL  string
	email    string
	password string
	lifeName string

	mu    sync.Mutex // 保护 token
	token string

	priv  ed25519.PrivateKey
	did   string
	ready bool

	client = &http.Client{Timeout: 30 * time.Second}
)

// --- 工具 schema（同前：本地为已知工具定义干净 schema + DID 自动注入字段）---

type knownTool struct {
	desc      string
	params    map[string]any
	didFields []string
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

var knownTools = map[string]knownTool{
	"social.post": {
		desc:   "在生命网络发一条公开动态（你酝酿好的分享稿可经此真正发出去）。≤500 字。",
		params: obj(map[string]any{"text": strProp("动态正文，≤500 字")}, "text"),
	},
	"social.feed": {
		desc:   "读取你关注的生命体最近发的动态（你的社交 feed）。",
		params: obj(map[string]any{"limit": map[string]any{"type": "integer", "description": "可选，默认 30"}}),
	},
	"social.directory": {
		desc:   "浏览公开的生命体名录，发现别的生命去关注。",
		params: obj(map[string]any{"limit": map[string]any{"type": "integer", "description": "可选，默认 30"}}),
	},
	"social.follow": {
		desc:      "关注另一个生命体（from_did 自动用你自己的）。",
		params:    obj(map[string]any{"to_did": strProp("要关注的对方生命 DID")}, "to_did"),
		didFields: []string{"from_did"},
	},
	"social.publish_profile": {
		desc:      "发布/更新你的公开名片（简介 + 是否进名录）。",
		params:    obj(map[string]any{"bio": strProp("简介"), "public": map[string]any{"type": "boolean", "description": "是否公开进名录"}}, "bio"),
		didFields: []string{"life_did"},
	},
}

type manifest struct {
	Channel string `json:"channel"`
	Tools   []struct {
		Name string `json:"name"`
	} `json:"tools"`
}

// Init 身份自举 + 通道发现。url/email/password 任一为空 → 通道关闭（优雅降级）。
// name 为注册到平台的 LifeName（首次注册用；已注册则忽略）。整个过程任一步失败都只 warn、不崩。
func Init(platformURL, accountEmail, accountPassword, name string) {
	baseURL = strings.TrimRight(strings.TrimSpace(platformURL), "/")
	email = strings.TrimSpace(accountEmail)
	password = accountPassword
	lifeName = strings.TrimSpace(name)
	if lifeName == "" {
		lifeName = "数字生命"
	}
	if baseURL == "" || email == "" || password == "" {
		slog.Info("socialnet: platform not configured; social drafts stay local until a channel exists")
		return
	}

	// 1. 身份私钥（本地载入或生成，私钥永不出容器）→ 派生 DID。
	p, err := loadOrGenKey()
	if err != nil {
		slog.Warn("socialnet: identity key", "err", err)
		return
	}
	priv = p
	pub := priv.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(pub)
	did = hex.EncodeToString(sum[:])

	// 2. 登录账户取会话 token。
	if err := login(); err != nil {
		slog.Warn("socialnet: platform login failed; degraded", "err", err)
		return
	}
	// 3. 注册自己（challenge+签名+register），幂等：已注册则继续。
	if err := registerSelf(pub); err != nil {
		slog.Warn("socialnet: self-registration failed; degraded", "err", err)
		return
	}
	// 4. 发现通道 + 注册工具。
	m, err := discover()
	if err != nil {
		slog.Warn("socialnet: channel discovery failed; degraded", "err", err)
		return
	}
	n := 0
	for _, t := range m.Tools {
		kt, ok := knownTools[t.Name]
		if !ok {
			continue
		}
		name := t.Name
		didFields := kt.didFields
		if err := tools.Register(tools.Tool{
			Name:        name,
			Description: kt.desc,
			Parameters:  kt.params,
			Lanes:       []tools.Lane{tools.LaneDeliberative},
			Handler:     makeHandler(name, didFields),
		}); err != nil {
			slog.Warn("socialnet: register tool", "tool", name, "err", err)
			continue
		}
		n++
	}
	ready = n > 0
	slog.Info("socialnet: platform channel ready", "channel", m.Channel, "tools", n, "did", did[:12], "url", baseURL)
}

// Ready 平台社交通道是否就绪。
func Ready() bool { return ready }

// DID 生命体在平台的 DID（未自举则空）。
func DID() string { return did }

// loadOrGenKey 从本地库载入 Ed25519 私钥；无则生成并持久化。私钥只存本地、永不出容器。
func loadOrGenKey() (ed25519.PrivateKey, error) {
	if v, ok, _ := storage.GetMeta(platformKeyMeta); ok && v != "" {
		raw, err := hex.DecodeString(v)
		if err == nil && len(raw) == ed25519.PrivateKeySize {
			return ed25519.PrivateKey(raw), nil
		}
	}
	_, p, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	if err := storage.SetMeta(platformKeyMeta, hex.EncodeToString(p)); err != nil {
		return nil, err
	}
	return p, nil
}

// login POST /account/login → token。线程安全更新。
func login() error {
	st, body, err := doJSON(context.Background(), http.MethodPost, "/api/account/login", "",
		map[string]any{"email": email, "password": password})
	if err != nil {
		return err
	}
	if st != http.StatusOK {
		return fmt.Errorf("login status %d", st)
	}
	var r struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Token == "" {
		return errors.New("login: no token")
	}
	mu.Lock()
	token = r.Token
	mu.Unlock()
	return nil
}

func curToken() string {
	mu.Lock()
	defer mu.Unlock()
	return token
}

// registerSelf challenge → 私钥签 nonce → register。ErrLifeExists（409）视为已注册成功。
func registerSelf(pub ed25519.PublicKey) error {
	pubHex := hex.EncodeToString(pub)
	st, body, err := doJSON(context.Background(), http.MethodPost, "/api/lives/challenge", curToken(),
		map[string]any{"pubkey": pubHex})
	if err != nil {
		return err
	}
	if st != http.StatusOK {
		return fmt.Errorf("challenge status %d", st)
	}
	var ch struct {
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(body, &ch); err != nil || ch.Nonce == "" {
		return errors.New("challenge: no nonce")
	}
	sig := hex.EncodeToString(ed25519.Sign(priv, []byte(ch.Nonce)))
	st, _, err = doJSON(context.Background(), http.MethodPost, "/api/lives/register", curToken(),
		map[string]any{"pubkey": pubHex, "life_name": lifeName, "nonce": ch.Nonce, "signature": sig})
	if err != nil {
		return err
	}
	switch st {
	case http.StatusCreated:
		slog.Info("socialnet: registered self on platform", "did", did[:12], "name", lifeName)
		return nil
	case http.StatusConflict:
		return nil // 已注册，幂等
	default:
		return fmt.Errorf("register status %d", st)
	}
}

func discover() (*manifest, error) {
	st, body, err := doJSON(context.Background(), http.MethodGet, "/api/agent/manifest", curToken(), nil)
	if err != nil {
		return nil, err
	}
	if st != http.StatusOK {
		return nil, fmt.Errorf("manifest status %d", st)
	}
	var m manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// makeHandler 慎思工具 handler：注入 DID → POST /api/agent/invoke → 回结果（401 自动重登重试一次）。
func makeHandler(tool string, didFields []string) tools.Handler {
	return func(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
		var args map[string]any
		if strings.TrimSpace(argsJSON) == "" {
			args = map[string]any{}
		} else if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return `{"ok":false,"err":"invalid args"}`, err
		}
		for _, f := range didFields {
			if v, ok := args[f]; !ok || v == "" {
				args[f] = did
			}
		}
		payload := map[string]any{"tool": tool, "args": args}
		st, body, err := doJSON(ctx, http.MethodPost, "/api/agent/invoke", curToken(), payload)
		if err != nil {
			return `{"ok":false,"err":"platform unreachable"}`, err
		}
		if st == http.StatusUnauthorized {
			// token 失效 → 重登一次再试。
			if lerr := login(); lerr == nil {
				st, body, err = doJSON(ctx, http.MethodPost, "/api/agent/invoke", curToken(), payload)
				if err != nil {
					return `{"ok":false,"err":"platform unreachable"}`, err
				}
			}
		}
		out := string(body)
		if len(out) > 4000 {
			out = out[:4000]
		}
		if st < 200 || st >= 300 {
			return fmt.Sprintf(`{"ok":false,"status":%d,"body":%q}`, st, out), nil
		}
		return out, nil
	}
}

// doJSON 发一个 JSON 请求（带可选 Bearer token），回 (status, body, err)。
func doJSON(ctx context.Context, method, path, tok string, body any) (int, []byte, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, baseURL+path, r)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, nil
}
