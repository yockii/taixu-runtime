package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"taixu.icu/runtime/internal/io/socialnet"
)

// apiPlatformStatus GET：平台社交通道状态（是否接通 + 本生命 DID），供面板展示与认领前确认。
func apiPlatformStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ready": socialnet.Ready(),
		"did":   socialnet.DID(),
	})
}

// apiPlatformClaim POST {code}：用用户在平台领取的临时认领码，把本生命改绑到该用户账户。
// 面板上输入认领码即触发（替代仅启动时 env TAIXU_CLAIM_CODE）。私钥签名在 socialnet 内完成、不出容器。
func apiPlatformClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(body.Code)
	if code == "" {
		http.Error(w, "认领码为空", http.StatusBadRequest)
		return
	}
	if err := socialnet.Claim(code); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "did": socialnet.DID()})
}
