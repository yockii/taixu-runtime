package httpapi

import (
	"taixu.icu/runtime/internal/io/egress"
)

// WebChannelName "web" 渠道名（入站/出站对齐）。网页发起的请求其响应回 web 渠道。
const WebChannelName = "web"

// webEgress "web" 渠道出站实现：把响应推进 SSE 流，让网页请求者真能收到回复/汇报，
// 而非只靠观察广播。React 在网页上 no-op（网页用别的"已读"指示，TODO 富交互）。
//
// 注意：不破坏现有 SSE 观察广播——观察面板仍订阅 reflex_reply 等事件（见 sse.go startSSEFanout）。
// webEgress 另发一个 web_reply 定向事件（带 to），供网页请求者侧识别"这是发给我的回复"。
type webEgress struct{}

func (webEgress) Name() string { return WebChannelName }

func (webEgress) Send(to, content string) error {
	startSSEFanout() // 确保广播通道已就绪（幂等）
	broadcast("web_reply", map[string]any{"to": to, "content": content})
	return nil
}

// React 网页渠道暂无即时表态手势（TODO：用前端"已读"指示替代）。no-op 返 nil。
func (webEgress) React(msgID, emoji string) error { return nil }

// RegisterEgress 把网页注册为 "web" 渠道的出站实现（httpapi 启动后调用）。
func RegisterEgress() { egress.Register(webEgress{}) }
