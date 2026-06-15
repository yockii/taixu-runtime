package httpapi

import (
	"encoding/json"
	"net/http"

	"taixu.icu/runtime/internal/io/codingagent"
	"taixu.icu/runtime/internal/runtime/lifecfg"
)

// 编码桥（C7 codingbridge）面板配置：仿 LLM/飞书 的 sqlite-config + env-兜底 + 面板配置模式。
// config 端点 POST（受 withAuth 写守卫：设了 control_token 则需 X-Taixu-Token）；status 为读、开放轮询。
// token 留空 = 沿用现有（面板掩码回显时不必重输）；url 写空 = 清除（coding_agent 工具失效）。

type bridgeReq struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	Agent string `json:"agent"`
}

// apiBridgeConfig POST /api/config/bridge —— 落库 + 热重配（codingagent.Reconfigure 即时生效，免重启）。
// 失败回 200+ok:false（非 4xx）便于前端统一读 error（apiPost 对非 2xx 抛错会丢 body）。
func apiBridgeConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req bridgeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := lifecfg.SetBridge(req.URL, req.Token, req.Agent); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// 落库后读回生效配置（含沿用的 token）→ 热重配工具。
	url, token, agent := lifecfg.BridgeConfig()
	codingagent.Reconfigure(url, token, agent)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "url": url, "agent": agent, "configured": url != "",
	})
}

// apiBridgeStatus GET /api/bridge/status —— 回当前配置 + 实时探测连通（GET bridge /health）。
// 未配 → configured:false；探测失败 connected:false（不报错，前端显 ✗）。
func apiBridgeStatus(w http.ResponseWriter, r *http.Request) {
	url, _, agent := lifecfg.BridgeConfig()
	resp := map[string]any{
		"configured": url != "",
		"url":        url,
		"agent":      agent,
		"connected":  false,
		"agents":     []string{},
	}
	if url != "" {
		if connected, agents, err := codingagent.Health(r.Context()); err == nil {
			resp["connected"] = connected
			if agents != nil {
				resp["agents"] = agents
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
