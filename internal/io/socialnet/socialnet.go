// Package socialnet 平台社交通道客户端（C：社交通道阶梯①，MCP-lite over HTTP）单例。
//
// 让数字生命体「发现并使用」平台 Life Network 这个结构化社交通道：
//   - 发现：启动时 GET <platform>/api/agent/manifest，看平台对 agent 暴露了哪些工具。
//   - 使用：把发现到的工具注册进慎思 lane（tools.LaneDeliberative），生命体在慎思中可直接调用
//     （如 social.post 发布它酝酿好的分享稿）；调用经 POST /api/agent/invoke 结构化转发。
//
// 这是「优先结构化通道、浏览器是最后手段」阶梯的第①层。没配置 / 不可达 → 通道缺席（优雅降级，
// 生命体照常活、社交稿先存着），未来由 skill 通道（②）/ 换平台（③）/ 浏览器（④）兜底。
//
// 鉴权：生命体在**归属用户的账户**下行动——平台 token 由用户配置（MINDVERSE_PLATFORM_TOKEN），
// 生命体的 DID（MINDVERSE_PLATFORM_DID）在调用时自动注入，LLM 无需知道自己的 DID。
package socialnet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"mindverse/internal/runtime/tools"
)

var (
	baseURL string
	token   string
	did     string
	client  = &http.Client{Timeout: 30 * time.Second}
	ready   bool
)

// knownTool 本地为平台 manifest 里的工具定义干净的 LLM schema + 需自动注入 DID 的字段。
// 只注册「平台 manifest 里确实声明了」且「本地认识」的工具（发现 + 已知双确认）。
type knownTool struct {
	desc      string
	params    map[string]any
	didFields []string // 这些参数若 LLM 没给，自动填生命体自己的 DID
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
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

// obj 构造一个 object JSON schema：props + 可选 required。
func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

// manifest 平台通道清单（只取 tool 名做发现校验）。
type manifest struct {
	Channel string `json:"channel"`
	Tools   []struct {
		Name string `json:"name"`
	} `json:"tools"`
}

// Init 装配并发现通道。URL+token 任一为空 → 通道关闭（Ready()=false，上层优雅降级）。
// 发现成功则把平台声明、且本地已知的工具注册进慎思 lane。lifeDID 为该生命在平台的 DID。
func Init(platformURL, accessToken, lifeDID string) {
	baseURL = strings.TrimRight(strings.TrimSpace(platformURL), "/")
	token = strings.TrimSpace(accessToken)
	did = strings.TrimSpace(lifeDID)
	if baseURL == "" || token == "" {
		slog.Info("socialnet: platform channel not configured; social drafts stay local until a channel exists")
		return
	}

	m, err := discover()
	if err != nil {
		slog.Warn("socialnet: channel discovery failed; degraded", "err", err)
		return
	}
	n := 0
	for _, t := range m.Tools {
		kt, ok := knownTools[t.Name]
		if !ok {
			continue // 平台声明了但本地不认识 → 跳过（保守）
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
	slog.Info("socialnet: platform channel ready", "channel", m.Channel, "tools", n, "url", baseURL)
}

// Ready 平台社交通道是否就绪（发现成功且至少注册了一个工具）。
func Ready() bool { return ready }

// discover GET manifest。
func discover() (*manifest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/agent/manifest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest status %d", resp.StatusCode)
	}
	var m manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// makeHandler 造一个慎思工具 handler：注入 DID 字段 → POST /api/agent/invoke {tool,args} → 回结果。
func makeHandler(tool string, didFields []string) tools.Handler {
	return func(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
		var args map[string]any
		if strings.TrimSpace(argsJSON) == "" {
			args = map[string]any{}
		} else if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return `{"ok":false,"err":"invalid args"}`, err
		}
		// 自动注入生命体自己的 DID（LLM 无需知道）。
		for _, f := range didFields {
			if v, ok := args[f]; !ok || v == "" {
				args[f] = did
			}
		}
		body, _ := json.Marshal(map[string]any{"tool": tool, "args": args})
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/api/agent/invoke", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return `{"ok":false,"err":"platform unreachable"}`, err
		}
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		out := string(raw)
		if len(out) > 4000 {
			out = out[:4000]
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Sprintf(`{"ok":false,"status":%d,"body":%q}`, resp.StatusCode, out), nil
		}
		return out, nil
	}
}
