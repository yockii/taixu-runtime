// Package egress 出站渠道抽象 + 注册表 + 中央分发器（单例风格，免注入）。
//
// 设计动机（出站渠道路由重构）：「消息从哪个渠道来，响应就回哪个渠道」。
// 出站事件（reflex.ReplyEvent / skill.ApprovalNeededEvent / 未来更多）都带 Channel 字段；
// 分发器据此 For(channel) 取对应 Egress 实现，调 Send/React，而非在各处散落
// `if channel != "feishu"` 守卫。新增渠道（微信/TG/Discord）只需各自实现 Egress + 启动期 Register。
//
// 线程模型：渠道在启动期（wire 阶段）一次性 Register，运行期只读 For。
// 仍用 RWMutex 兜底并发安全（启动与首条出站事件理论上可交错）。
package egress

import (
	"log/slog"
	"sync"
)

// Egress 一个出站渠道的最小能力。
//
//	Name()  渠道标识，与入站事件的 Channel 字段对齐（"feishu" / "web" / ...）。
//	Send    向 to 发一条文本。to 的语义由各渠道自定（飞书 open_id / web 会话键 等）；
//	        空 to 由各渠道自行兜底（按渠道记的 lastPeer），不依赖跨渠道全局单值。
//	React   对一条入站消息加表态/已读指示（飞书 reaction）。不支持的渠道 no-op 返 nil。
type Egress interface {
	Name() string
	Send(to, content string) error
	React(msgID, emoji string) error
}

// ApprovalSender 可选能力：渠道支持富交互审批（飞书审批卡片）。
// 分发器优先用它发审批；未实现该接口的渠道退化为 Send 一段文本审批提示。
type ApprovalSender interface {
	SendApproval(to, skillID, skillName, deps string) error
}

var (
	mu       sync.RWMutex
	registry = map[string]Egress{}
)

// Register 注册一个出站渠道（启动期调用）。重名后注册覆盖前者并告警。
func Register(e Egress) {
	if e == nil {
		return
	}
	name := e.Name()
	if name == "" {
		slog.Warn("egress register: empty channel name, skipped")
		return
	}
	mu.Lock()
	if _, exists := registry[name]; exists {
		slog.Warn("egress register: overwriting existing channel", "channel", name)
	}
	registry[name] = e
	mu.Unlock()
	slog.Info("egress registered", "channel", name)
}

// For 取某渠道的出站实现；未注册返回 (nil, false)。
func For(channel string) (Egress, bool) {
	mu.RLock()
	e, ok := registry[channel]
	mu.RUnlock()
	return e, ok
}

// Channels 返回已注册的渠道名（观察/诊断用，无序）。
func Channels() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

// reset 清空注册表（仅测试用）。
func reset() {
	mu.Lock()
	registry = map[string]Egress{}
	mu.Unlock()
}
