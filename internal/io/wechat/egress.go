package wechat

import "taixu.icu/runtime/internal/io/egress"

// Egress 微信出站实现。React 无操作（个人微信无表态 API）。Send 自动带 context_token（见 wechat.Send）。
type Egress struct{}

func (Egress) Name() string { return ChannelName }

func (Egress) Send(to, content string) error { return Send(to, content) }

func (Egress) React(string, string) error { return nil }

// RegisterEgress 注册微信为 "wechat" 渠道出站实现（wireWechat 成功 Init 后调用）。
func RegisterEgress() { egress.Register(Egress{}) }
