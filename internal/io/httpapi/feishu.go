package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/larksuite/oapi-sdk-go/v3/scene/registration"

	"taixu.icu/runtime/internal/io/lark"
	"taixu.icu/runtime/internal/runtime/lifecfg"
)

// 飞书接入：① 一键创建（扫码 OAuth 设备授权，推荐）② 手填 app_id/secret。
// 凭据落 sqlite config，重启生效（飞书 ws 长连 + 事件分发在 boot 建立，不热重连）。
// 写端点受 withAuth 守卫；status 为读、开放轮询。

// apiFeishuRegisterStart POST /api/feishu/register/start —— 启动一键创建，触发二维码。
// 扫码授权成功后凭据自动落库（onDone）。前端轮询 /status 拿二维码 URL + 进度。
func apiFeishuRegisterStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	preset := &registration.AppPreset{
		Name: "{user} 的数字生命 · 太虚",
		Desc: "由 {user} 在太虚孕育的数字生命，经此飞书应用与你对话。",
	}
	lark.StartRegister(preset, func(appID, secret string) {
		_ = lifecfg.SetFeishuConfig(appID, secret)
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// apiFeishuRegisterStatus GET /api/feishu/register/status —— 轮询一键创建进度。
// status: idle|starting|waiting(扫码中)|done|failed。done=凭据已落库，提示用户重启生效。
func apiFeishuRegisterStatus(w http.ResponseWriter, r *http.Request) {
	status, qrURL, errMsg, expireAt := lark.RegisterStatus()
	writeJSON(w, http.StatusOK, map[string]any{
		"status": status, "qr_url": qrURL, "error": errMsg, "expire_at": expireAt,
	})
}

// apiFeishuConfig POST /api/feishu/config —— 手填 app_id/secret 落库（一键之外的备选）。重启生效。
func apiFeishuConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AppID  string `json:"app_id"`
		Secret string `json:"app_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.AppID == "" || req.Secret == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "app_id / app_secret 必填"})
		return
	}
	if err := lifecfg.SetFeishuConfig(req.AppID, req.Secret); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
