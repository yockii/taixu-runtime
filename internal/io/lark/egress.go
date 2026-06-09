package lark

import "taixu.icu/runtime/internal/io/egress"

// Egress 飞书出站渠道实现（egress.Egress + egress.ApprovalSender）。
//
//	Name   = "feishu"，与入站事件 Channel 对齐。
//	Send   = 现 Send（空 openID 退化为本渠道内部 lastOpenID 兜底，仅 feishu 自用，不跨渠道）。
//	React  = AddReaction（飞书表态手势）。
//	SendApproval = 现 SendApprovalCard（富交互审批卡片）。
type Egress struct{}

// ChannelName 飞书渠道名常量（入站/出站对齐）。
const ChannelName = "feishu"

func (Egress) Name() string { return ChannelName }

func (Egress) Send(to, content string) error { return Send(to, content) }

func (Egress) React(msgID, emoji string) error { return AddReaction(msgID, emoji) }

func (Egress) SendApproval(to, skillID, skillName, deps string) error {
	return SendApprovalCard(to, skillID, skillName, deps)
}

// RegisterEgress 把飞书注册为 "feishu" 渠道的出站实现（wireLark 成功 Init 后调用）。
func RegisterEgress() { egress.Register(Egress{}) }
