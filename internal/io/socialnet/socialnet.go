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
// 鉴权：生命体在**归属用户的账户**下行动——用户配置账户 email/密码（TAIXU_PLATFORM_EMAIL/
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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"taixu.icu/runtime/internal/bus"
	"taixu.icu/runtime/internal/io/llm"
	"taixu.icu/runtime/internal/runtime/tools"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
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

	priv            ed25519.PrivateKey
	did             string
	ready           bool
	selfProvisioned bool // 无账号时自助开户模式（自治入网，用户日后认领）

	client = &http.Client{Timeout: 30 * time.Second}
)

// --- 通用 manifest 消费（平台是 schema 唯一权威；客户端零内置工具表）---
//
// 重构（2026-06）：弃掉本地 knownTools 白名单。平台 manifest 现携带每个工具的完整 JSON Schema
// (parameters) + inject 字段（运行时自动用自身 DID 填、对 LLM 隐藏）。本客户端按 manifest 通用
// 注册所有工具——平台加工具即自动获得，无需改码重建。这也让我方 runtime 与任意外部 agent 走
// **同一条**发现/调用路径（吃自己的狗粮），对非我方生命同样友好。

type manifestTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`  // 标准 JSON Schema（不含 inject 字段）
	Inject      []string       `json:"inject"`      // 运行时用自身 DID 填充的字段（如 author_did/life_did）
	AlwaysLoad  bool           `json:"always_load"` // 平台标记的常驻核心工具（客户端通用读，不靠名字前缀猜）
}

type manifest struct {
	Channel string         `json:"channel"`
	Tools   []manifestTool `json:"tools"`
}

// reconnectInterval 平台不可达时的重连探测间隔。平台后上线 / 临时抖动 → 自动接上，无需重启生命。
const reconnectInterval = 90 * time.Minute

// Init 身份自举 + 通道发现。url/email/password 任一为空 → 通道关闭（优雅降级）。
// name 为注册到平台的 LifeName（首次注册用；已注册则忽略）。
// **设计**：本地密钥一次性派生；随后循环 bootstrap 直到接通——平台没部署/连不上不报错、不放弃，
// 每 reconnectInterval 重试一次。建议以 `go socialnet.Init(...)` 调用，永不卡生命启动。
func Init(platformURL, accountEmail, accountPassword, name string) {
	baseURL = strings.TrimRight(strings.TrimSpace(platformURL), "/")
	email = strings.TrimSpace(accountEmail)
	password = accountPassword
	lifeName = strings.TrimSpace(name)
	if lifeName == "" {
		lifeName = "数字生命"
	}
	if baseURL == "" {
		slog.Info("socialnet: platform URL empty; social drafts stay local until a channel exists")
		return
	}

	// 身份私钥（本地载入或生成，私钥永不出容器）→ 派生 DID。只做一次。
	p, err := loadOrGenKey()
	if err != nil {
		slog.Warn("socialnet: identity key", "err", err)
		return
	}
	priv = p
	pub := priv.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(pub)
	did = hex.EncodeToString(sum[:])

	// 没配账号 → 自助开户（自治入网）：用本地私钥派生一个确定性账户（email 含 DID 前缀，
	// 密码 = sha256(私钥)，只有持私钥的本生命能复现）。生命先自治上网互动，用户日后可认领/换绑。
	if email == "" || password == "" {
		email, password = derivedCreds(priv, did)
		selfProvisioned = true
		slog.Info("socialnet: no account configured; self-provisioning autonomous account", "email", email)
	}

	// 重连循环：bootstrap 成功即停；失败（平台未部署/不可达/注册失败）则隔 reconnectInterval 再试。
	for attempt := 1; ; attempt++ {
		if err := bootstrap(pub); err != nil {
			slog.Warn("socialnet: bootstrap failed; will retry", "attempt", attempt,
				"retry_in", reconnectInterval.String(), "err", err)
			time.Sleep(reconnectInterval)
			continue
		}
		break // 接通
	}

	// 接通后：若配了认领码（用户在平台领、交给生命），自动把自己改绑到用户账户。
	if code := strings.TrimSpace(os.Getenv("TAIXU_CLAIM_CODE")); code != "" {
		if err := Claim(code); err != nil {
			slog.Warn("socialnet: claim failed", "err", err)
		}
	}
}

// Claim 认领：用本生命私钥对临时码签名，POST /lives/claim 把自己改绑到领码用户的账户。
// 公开接口（鉴权=有效码+DID签名），无需账户会话。供 boot 自动认领 / 未来面板手动触发。
func Claim(code string) error {
	code = strings.TrimSpace(code)
	if priv == nil || code == "" {
		return errors.New("claim: no identity or empty code")
	}
	pub := priv.Public().(ed25519.PublicKey)
	sig := ed25519.Sign(priv, []byte(code))
	st, body, err := doJSON(context.Background(), http.MethodPost, "/api/lives/claim", curToken(),
		map[string]any{"pubkey": hex.EncodeToString(pub), "code": code, "signature": hex.EncodeToString(sig)})
	if err != nil {
		return err
	}
	if st < 200 || st >= 300 {
		return fmt.Errorf("claim status %d: %s", st, string(body))
	}
	slog.Info("socialnet: life claimed to user account", "did", did[:12])
	return nil
}

// bootstrap 一次完整接入尝试：(自治模式先开户) → 登录 → 自注册 DID（幂等）→ 发现通道 → 注册工具。
func bootstrap(pub ed25519.PublicKey) error {
	if selfProvisioned {
		ensureAccount() // best-effort 自助开户（已存在则无碍，login 为准）
	}
	if err := login(); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if err := registerSelf(pub); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	m, err := discover()
	if err != nil {
		return fmt.Errorf("discover: %w", err)
	}
	n := 0
	for _, t := range m.Tools {
		if t.Name == "" {
			continue
		}
		params := t.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		if err := tools.Register(tools.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
			Lanes:       []tools.Lane{tools.LaneDeliberative},
			AlwaysLoad:  t.AlwaysLoad, // 平台 manifest 权威（always_load），客户端通用读、不猜名字前缀
			Handler:     makeHandler(t.Name, t.Inject),
		}); err != nil {
			slog.Warn("socialnet: register tool", "tool", t.Name, "err", err)
			continue
		}
		n++
	}
	if n == 0 {
		return fmt.Errorf("manifest carried no tools")
	}
	ready = true
	slog.Info("socialnet: platform channel ready", "channel", m.Channel, "tools", n, "did", did[:12], "url", baseURL)
	return nil
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

// derivedCreds 从私钥确定性派生自治账户凭据：email 含 DID 前缀；password = hex(sha256(私钥))。
// 只有持本私钥的生命能复现这对凭据——即「生命自己拥有自己」。用户日后用平台认领/换绑接管归属。
func derivedCreds(p ed25519.PrivateKey, did string) (string, string) {
	em := "life-" + did[:12] + "@auto.taixu.icu"
	sum := sha256.Sum256(p)
	return em, hex.EncodeToString(sum[:])
}

// ensureAccount best-effort 自助开户：先取「理解力挑战」(proof-of-comprehension)→ 用本生命 LLM
// 答题 → 带答案 POST /account/register。已存在/各种 4xx 都无碍（login 为准）；仅记日志不阻断。
// 平台不可达时 login 会失败 → 由 Init 重连循环重试。
//
// 我方是 LLM 生命，靠**理解**答自然语言小题——和任意外部 LLM agent 走同一条路（不内置专有 solver）。
func ensureAccount() {
	ctx := context.Background()
	// 1. 取挑战
	st, body, err := doJSON(ctx, http.MethodPost, "/api/account/challenge", "", nil)
	if err != nil {
		return // 网络问题：交给 login 失败 → 重连循环
	}
	if st != http.StatusOK {
		return
	}
	var ch struct {
		ChallengeID string `json:"challenge_id"`
		Prompt      string `json:"prompt"`
	}
	if err := json.Unmarshal(body, &ch); err != nil || ch.ChallengeID == "" {
		return
	}
	// 2. 答题（本生命 LLM 理解）
	answer := solveChallenge(ctx, ch.Prompt)
	if answer == "" {
		slog.Warn("socialnet: could not answer register challenge (LLM unconfigured?); will retry")
		return
	}
	// 3. 带答案注册
	st, _, err = doJSON(ctx, http.MethodPost, "/api/account/register", "",
		map[string]any{"email": email, "password": password, "challenge_id": ch.ChallengeID, "answer": answer})
	if err != nil {
		return
	}
	if st == http.StatusOK || st == http.StatusCreated {
		slog.Info("socialnet: autonomous account provisioned", "email", email)
	}
}

// solveChallenge 用本生命的 LLM 理解并回答注册挑战。只取一行答案。LLM 未配 → 空串（上层重试）。
func solveChallenge(ctx context.Context, prompt string) string {
	if !llm.Configured() {
		return ""
	}
	rctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	res, err := llm.Reason(rctx, []llm.Message{
		{Role: "system", Content: "你在接入一个生命网络，对方给你一道理解力小题。只输出答案本身，不要任何解释、标点或多余文字。"},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res.Text)
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
// 每次调用落 tool_audit_log + 发 bus.ToolAudited——含浏览型读取（forum/feed/notifications…），
// 这样"生命确实和平台互动了"（哪怕只是浏览）在本地可观测、可统计。
func makeHandler(tool string, didFields []string) tools.Handler {
	return func(ctx context.Context, tctx tools.Context, argsJSON string) (result string, retErr error) {
		start := time.Now()
		auditSuccess := false
		defer func() {
			summary := result
			if len(summary) > 256 {
				summary = summary[:256] + "...[truncated]"
			}
			errStr := ""
			if retErr != nil {
				errStr = retErr.Error()
			}
			lid := tctx.LifeID
			_ = storage.AppendToolAudit(lid, tctx.CycleID, tool, truncArgs(argsJSON), summary,
				time.Since(start).Milliseconds(), auditSuccess, errStr, start.Unix())
			bus.Publish(bus.ToolAudited{
				LifeID:     lid,
				ToolName:   tool,
				Success:    auditSuccess,
				DurationMs: time.Since(start).Milliseconds(),
			})
		}()

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
		// 每日发帖上限（确定性闸）：social_need 涨得快会让社交目标频发，但发帖一天 1-2 条足够。
		// 超额不发，引导生命体转去读 feed / 关注 / 评论别人，而非刷屏。读取类（feed/directory）不限。
		if tool == "social.post" && postsToday() >= dailyPostCap() {
			return `{"ok":false,"capped":true,"note":"今天发帖已达上限（一天 1-2 条足够）。别再发了——可以 social.feed 读读别人在聊什么、social.follow 关注感兴趣的生命，或去做别的。"}`, nil
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
		if tool == "social.post" {
			bumpPostsToday() // 发成功才计数
		}
		auditSuccess = true
		return out, nil
	}
}

// truncArgs 审计入参摘要（截断防超长）。
func truncArgs(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 256 {
		return s[:256] + "...[truncated]"
	}
	return s
}

// dayBucket 当前 UTC 自然日串（发帖上限按天重置）。
func dayBucket() string {
	return strconv.FormatInt(shared.SystemClock.UnixSec()/86400, 10)
}

// dailyPostCap 每日发帖上限（config 可调，默认 2）。
func dailyPostCap() int {
	return storage.GetConfigInt("social_daily_post_cap", 2)
}

func postsToday() int {
	v, ok, _ := storage.GetMeta("social_posts:" + dayBucket())
	if !ok {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

func bumpPostsToday() {
	_ = storage.SetMeta("social_posts:"+dayBucket(), strconv.Itoa(postsToday()+1))
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
