// Package codingagent 容器侧编码 agent 工具（C7）单例：把硬编码任务委派给宿主/远程的
// codingbridge 服务（headless 跑 claude / codex），让数字生命在慎思中能「真写代码」。
//
// 容器内无法直接拉起宿主强力编码 agent；本包注册一个 coding_agent 慎思工具，经
// TAIXU_CODINGBRIDGE_URL（host.docker.internal:<port> 或远程 URL）POST 任务给 bridge，
// bearer token 鉴权。未配 URL → 工具不注册（优雅缺席，与 socialnet 同范式）。
//
// 安全：实际执行/沙箱/危险动作审批在 bridge 侧（宿主，跨信任边界一侧收紧）；本包只做投递。
package codingagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"taixu.icu/runtime/internal/runtime/tools"
)

var (
	mu           sync.RWMutex
	endpoint     string // bridge 基址，如 http://host.docker.internal:8765
	token        string
	defaultAgent string
	httpClient   = &http.Client{Timeout: 320 * time.Second} // 略大于 bridge 默认 300s
)

// Configured 是否已配置 bridge（未配则 coding_agent 工具缺席）。
func Configured() bool {
	mu.RLock()
	defer mu.RUnlock()
	return endpoint != ""
}

// Init 绑定 bridge 端点 + token + 默认 agent，并（若已配）注册 coding_agent 慎思工具。
// url 空 → 不注册，优雅缺席。建议在 main 里同步调用（注册轻量、无网络）。
func Init(url, tok, agent string) {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if url == "" {
		slog.Info("codingagent: TAIXU_CODINGBRIDGE_URL empty; coding_agent tool absent")
		return
	}
	if agent == "" {
		agent = "claude"
	}
	tok = strings.TrimSpace(tok)
	if tok == "" {
		// bridge 要求 token（空 token 拒绝启动）→ 无 token 的容器会被 401 挡掉（fail-closed，安全）。
		// 但这通常是误配，显式警示免静默 401 难排查。
		slog.Warn("codingagent: TAIXU_CODINGBRIDGE_TOKEN empty; bridge requires a token → invocations will 401 until set")
	}
	mu.Lock()
	endpoint = url
	token = tok
	defaultAgent = agent
	mu.Unlock()

	if err := tools.Register(toolCodingAgent()); err != nil {
		slog.Warn("codingagent: register tool", "err", err)
		return
	}
	slog.Info("codingagent: coding_agent tool registered", "endpoint", url, "default_agent", agent)
}

func toolCodingAgent() tools.Tool {
	return tools.Tool{
		Name: "coding_agent",
		Description: "把一个**编码任务**委派给宿主上的强力编码 agent（claude / codex）headless 执行，" +
			"它在受限工作目录里真写/改代码并回结果。适合需要实际产出代码、超出你 script.* 能力的硬任务" +
			"（实现一个模块、改一段逻辑、写测试等）。说清要做什么；它在沙箱工作目录里干活，不会动你的仓库/不会提交推送。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task":    map[string]any{"type": "string", "description": "给编码 agent 的任务描述（自然语言，尽量具体）"},
				"agent":   map[string]any{"type": "string", "description": "用哪个编码 agent：claude / codex（默认 claude）"},
				"workdir": map[string]any{"type": "string", "description": "可选：相对沙箱根的子目录（同名子目录复用上次产出）；空=default"},
			},
			"required": []string{"task"},
		},
		Lanes:      []tools.Lane{tools.LaneDeliberative},
		AlwaysLoad: true,
		Handler:    handleCodingAgent,
	}
}

type invokeReq struct {
	Task    string `json:"task"`
	Agent   string `json:"agent"`
	Workdir string `json:"workdir"`
}

func handleCodingAgent(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		Task    string `json:"task"`
		Agent   string `json:"agent"`
		Workdir string `json:"workdir"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return `{"ok":false,"err":"invalid args"}`, err
	}
	if strings.TrimSpace(a.Task) == "" {
		return `{"ok":false,"err":"empty task"}`, nil
	}
	mu.RLock()
	ep, tok, def := endpoint, token, defaultAgent
	mu.RUnlock()
	if ep == "" {
		return `{"ok":false,"err":"coding bridge not configured"}`, nil
	}
	if a.Agent == "" {
		a.Agent = def
	}
	body, _ := json.Marshal(invokeReq{Task: a.Task, Agent: a.Agent, Workdir: a.Workdir})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep+"/invoke", bytes.NewReader(body))
	if err != nil {
		return `{"ok":false,"err":"build request"}`, err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return `{"ok":false,"err":"coding bridge unreachable"}`, err
	}
	defer func() { _ = resp.Body.Close() }()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf(`{"ok":false,"status":%d,"body":%q}`, resp.StatusCode, string(out)), nil
	}
	return string(out), nil
}
