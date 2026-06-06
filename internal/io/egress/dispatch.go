package egress

import (
	"log/slog"
	"sync"

	"mindverse/internal/bus"
	"mindverse/internal/runtime/reflex"
	"mindverse/internal/runtime/skill"
	"mindverse/internal/storage"
)

// peerResolver 兜底：当出站事件 channel/to 缺失时，按渠道找最近对端。
// 默认实现走 storage.MostRecentContact（返回 channel + peer），分发器据返回的 channel 再选 egress——
// 因此兜底也是「按渠道」路由，而非写死飞书。测试可替换。
type peerResolver func(lifeID string) (channel, to string, ok bool)

var defaultPeerResolver peerResolver = func(lifeID string) (string, string, bool) {
	c, err := storage.MostRecentContact(lifeID)
	if err != nil || c == nil {
		return "", "", false
	}
	return c.Channel, c.PeerID, true
}

var (
	subOnce  sync.Once
	resolver = defaultPeerResolver
)

// StartDispatcher 一次性订阅出站事件并按 channel 路由到对应 egress。
// 在所有渠道 Register 之前或之后调用均可（订阅在运行期触发，那时渠道已就绪）。
func StartDispatcher() {
	subOnce.Do(func() {
		bus.Subscribe(reflex.ReplyEvent{}, func(e bus.Event) {
			ev := e.(reflex.ReplyEvent)
			dispatchSend(ev.LifeID, ev.Channel, ev.To, ev.Content, "reply")
		})
		bus.Subscribe(skill.ApprovalNeededEvent{}, func(e bus.Event) {
			ev := e.(skill.ApprovalNeededEvent)
			dispatchApproval(ev)
		})
		slog.Info("egress dispatcher started")
	})
}

// dispatchSend 把一条文本响应路由到来源渠道。channel/to 缺失时按渠道兜底最近对端。
func dispatchSend(lifeID, channel, to, content, kind string) {
	channel, to = resolveTarget(lifeID, channel, to)
	if channel == "" {
		slog.Warn("egress: no channel for outbound", "kind", kind)
		return
	}
	eg, ok := For(channel)
	if !ok {
		slog.Warn("egress: no egress registered for channel, dropping", "channel", channel, "kind", kind)
		return
	}
	if err := eg.Send(to, content); err != nil {
		slog.Error("egress send", "channel", channel, "kind", kind, "err", err)
	}
}

// dispatchApproval 把技能审批请求路由到来源渠道（隐患①修复：不再写死飞书）。
// 来源渠道支持富交互（ApprovalSender）→ 发审批卡片；否则退化为文本审批提示。
func dispatchApproval(ev skill.ApprovalNeededEvent) {
	channel, to := resolveTarget(ev.LifeID, ev.Channel, ev.To)
	if channel == "" {
		slog.Warn("skill approval: no source channel, fallback to panel", "skill", ev.SkillID)
		return
	}
	eg, ok := For(channel)
	if !ok {
		slog.Warn("skill approval: no egress for channel, fallback to panel", "channel", channel, "skill", ev.SkillID)
		return
	}
	if as, ok := eg.(ApprovalSender); ok {
		if err := as.SendApproval(to, ev.SkillID, ev.SkillName, ev.Deps); err != nil {
			slog.Error("egress send approval", "channel", channel, "skill", ev.SkillID, "err", err)
		}
		return
	}
	// TODO 富交互审批：非飞书渠道暂以文本提示代替按钮卡片（用户经面板批准）。
	msg := "需要审批：技能「" + ev.SkillName + "」请求安装依赖\n" + ev.Deps + "\n请到观察面板批准或拒绝。"
	if err := eg.Send(to, msg); err != nil {
		slog.Error("egress send approval text", "channel", channel, "skill", ev.SkillID, "err", err)
	}
}

// resolveTarget 解析出站目标：优先用事件自带 channel/to；
// channel 缺失时按渠道兜底最近对端（隐患②修复：不再依赖全局 lark.LastSenderOpenID 跨渠道串台）。
func resolveTarget(lifeID, channel, to string) (string, string) {
	if channel != "" {
		return channel, to
	}
	if c, t, ok := resolver(lifeID); ok {
		return c, t
	}
	return "", to
}
