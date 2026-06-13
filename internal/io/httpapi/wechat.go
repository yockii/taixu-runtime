package httpapi

import (
	"net/http"

	"taixu.icu/runtime/internal/io/wechat"
	"taixu.icu/runtime/internal/runtime/lifecfg"
)

// 微信接入：扫码登录 iLink（个人微信官方 Bot API）。bot_token 落 sqlite，重启生效（收消息长轮询在 boot 起）。
// 一号一 bot（个人微信单会话）。扫一次长效，无 24h 重扫。写端点受 withAuth 守卫；status 开放轮询。

// apiWechatRegisterStart POST /api/wechat/register/start —— 启动扫码登录。成功后 bot_token 落库（onDone）。
func apiWechatRegisterStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	wechat.StartRegister(func(token string) {
		_ = lifecfg.SetWechatBotToken(token)
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// apiWechatRegisterStatus GET /api/wechat/register/status —— 轮询进度。
// status idle|starting|waiting|done|failed；qr_img=二维码图(base64,前端直接 <img>)；qr_url=备用链接。
func apiWechatRegisterStatus(w http.ResponseWriter, r *http.Request) {
	status, qrImg, qrURL, errMsg := wechat.RegisterStatus()
	writeJSON(w, http.StatusOK, map[string]any{
		"status": status, "qr_img": qrImg, "qr_url": qrURL, "error": errMsg,
	})
}
