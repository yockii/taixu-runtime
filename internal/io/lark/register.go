package lark

import (
	"context"
	"sync"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/scene/registration"
)

// 飞书一键创建应用（OAuth 2.0 设备授权 RFC8628）：触发二维码 → 用户扫码授权 → SDK 轮询 →
// 返回 app_id/app_secret。单生命单会话（包级状态足够）。凭据落库后重启生效（ws 长连/事件分发在 boot 建立）。

type regSession struct {
	mu       sync.Mutex
	active   bool
	status   string // idle | starting | waiting(扫码中) | done | failed
	qrURL    string // 验证链接（前端渲染成二维码 / 可点开）
	expireAt int64  // 二维码过期 unix 秒
	errMsg   string
}

var reg = &regSession{status: "idle"}

// StartRegister 启动一键创建会话。幂等：已 active 直接返回当前会话。
// onDone(appID, secret) 在扫码授权成功后回调（由调用方落库）。RegisterApp 阻塞，故走 goroutine。
func StartRegister(preset *registration.AppPreset, onDone func(appID, secret string)) {
	reg.mu.Lock()
	if reg.active {
		reg.mu.Unlock()
		return
	}
	reg.active = true
	reg.status = "starting"
	reg.qrURL = ""
	reg.errMsg = ""
	reg.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		res, err := registration.RegisterApp(ctx, &registration.Options{
			AppPreset: preset,
			OnQRCode: func(q *registration.QRCodeInfo) {
				reg.mu.Lock()
				reg.qrURL = q.URL
				reg.expireAt = time.Now().Unix() + int64(q.ExpireIn)
				reg.status = "waiting"
				reg.mu.Unlock()
			},
			OnStatusChange: func(*registration.StatusChangeInfo) {},
		})
		if err != nil {
			reg.mu.Lock()
			reg.status = "failed"
			reg.errMsg = err.Error()
			reg.active = false
			reg.mu.Unlock()
			return
		}
		if onDone != nil && res != nil {
			onDone(res.ClientID, res.ClientSecret) // 先落库，再标 done（前端见 done 时凭据已存）
		}
		reg.mu.Lock()
		reg.status = "done"
		reg.active = false
		reg.mu.Unlock()
	}()
}

// RegisterStatus 当前一键创建会话状态（httpapi 轮询）。
func RegisterStatus() (status, qrURL, errMsg string, expireAt int64) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.status, reg.qrURL, reg.errMsg, reg.expireAt
}
