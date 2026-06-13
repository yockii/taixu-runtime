package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// 微信扫码登录：get_bot_qrcode 取码 → 轮询 get_qrcode_status 直到 confirmed → 拿持久 bot_token。
// 扫一次长期有效（无 24h 重扫；bot_token 持久）。单会话（个人微信一号一 bot）。结构对齐 lark.register。

type regSession struct {
	mu       sync.Mutex
	active   bool
	status   string // idle|starting|waiting(待扫码)|done|failed
	qrImg    string // qrcode_img_content（多为 base64 图，前端可直接 <img>）
	qrURL    string // 备用：可点开的二维码 URL
	errMsg   string
}

var reg = &regSession{status: "idle"}

func apiGet(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header = headers("")
	resp, err := httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wechat %s: http %d", path, resp.StatusCode)
	}
	return json.Unmarshal(data, out)
}

// StartRegister 启动扫码登录会话。幂等：已 active 直接返回。
// onDone(botToken) 在 confirmed 后回调（由调用方落库 + Init + 起收消息循环）。
func StartRegister(onDone func(botToken string)) {
	reg.mu.Lock()
	if reg.active {
		reg.mu.Unlock()
		return
	}
	reg.active = true
	reg.status = "starting"
	reg.qrImg, reg.qrURL, reg.errMsg = "", "", ""
	reg.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		var qr struct {
			Qrcode           string `json:"qrcode"`
			QrcodeImgContent string `json:"qrcode_img_content"`
			URL              string `json:"url"`
		}
		if err := apiGet(ctx, "/ilink/bot/get_bot_qrcode?bot_type=3", &qr); err != nil || qr.Qrcode == "" {
			reg.fail(errStr(err, "取二维码失败"))
			return
		}
		reg.mu.Lock()
		reg.qrImg, reg.qrURL, reg.status = qr.QrcodeImgContent, qr.URL, "waiting"
		reg.mu.Unlock()

		for ctx.Err() == nil {
			var st struct {
				Status   string `json:"status"`
				BotToken string `json:"bot_token"`
			}
			if err := apiGet(ctx, "/ilink/bot/get_qrcode_status?qrcode="+qr.Qrcode, &st); err != nil {
				time.Sleep(1500 * time.Millisecond)
				continue
			}
			if st.Status == "confirmed" && st.BotToken != "" {
				if onDone != nil {
					onDone(st.BotToken)
				}
				reg.mu.Lock()
				reg.status, reg.active = "done", false
				reg.mu.Unlock()
				return
			}
			time.Sleep(1500 * time.Millisecond)
		}
		reg.fail("二维码超时，请重试")
	}()
}

func (r *regSession) fail(msg string) {
	r.mu.Lock()
	r.status, r.errMsg, r.active = "failed", msg, false
	r.mu.Unlock()
}

func errStr(err error, def string) string {
	if err != nil {
		return def + "：" + err.Error()
	}
	return def
}

// RegisterStatus 当前扫码会话状态（httpapi 轮询）。
func RegisterStatus() (status, qrImg, qrURL, errMsg string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.status, reg.qrImg, reg.qrURL, reg.errMsg
}
