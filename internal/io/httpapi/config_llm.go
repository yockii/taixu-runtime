package httpapi

import (
	"encoding/json"
	"net/http"

	"taixu.icu/runtime/internal/runtime/lifecfg"
)

// 界面换 LLM：测连通 + 热切换。两端点都是 POST（受 withAuth 写守卫：设了 control_token 则需 X-Taixu-Token）。
// base/key/model 从前端表单来；key 留空=沿用现有（面板掩码回显时不必重输密钥）。

type llmReq struct {
	BaseURL     string `json:"base_url"`
	APIKey      string `json:"api_key"`
	Model       string `json:"model"`
	Temperature string `json:"temperature"`
}

// apiLLMTest POST /api/config/llm/test —— 用候选配置发最小请求验连通，不改在用模型。
// 回 {ok:true} 或 {ok:false, error:"401 ..."}（前端据此显 ✓/✗）。
func apiLLMTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req llmReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := lifecfg.TestLLM(r.Context(), req.BaseURL, req.APIKey, req.Model); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// apiLLMConfig POST /api/config/llm —— 先测通、再写 sqlite、再热重装。失败回 400+原因，不留半套坏配置。
func apiLLMConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req llmReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := lifecfg.ApplyLLM(r.Context(), req.BaseURL, req.APIKey, req.Model, req.Temperature); err != nil {
		// 回 200+ok:false（非 4xx）便于前端统一读 error 文案（apiPost 对非 2xx 抛错会丢 body）。
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	base, model, temp := lifecfg.EffectiveLLM()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "base_url": base, "model": model, "temperature": temp,
	})
}
