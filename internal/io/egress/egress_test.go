package egress

import (
	"errors"
	"sync"
	"testing"

	"taixu.icu/runtime/internal/bus"
	"taixu.icu/runtime/internal/runtime/reflex"
	"taixu.icu/runtime/internal/runtime/skill"
)

// fakeEgress 记录收到的 Send/React/Approval，供断言"收到且不串渠道"。
type fakeEgress struct {
	name string

	mu        sync.Mutex
	sends     []sendRec
	approvals []approvalRec
	reacts    []string
	sendErr   error

	approval bool // 是否实现 ApprovalSender
}

type sendRec struct{ to, content string }
type approvalRec struct{ to, skillID, skillName, deps string }

func (f *fakeEgress) Name() string { return f.name }

func (f *fakeEgress) Send(to, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sends = append(f.sends, sendRec{to, content})
	return f.sendErr
}

func (f *fakeEgress) React(msgID, emoji string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reacts = append(f.reacts, msgID)
	return nil
}

// approvalEgress 是带 ApprovalSender 的 fakeEgress。
type approvalEgress struct{ *fakeEgress }

func (f approvalEgress) SendApproval(to, skillID, skillName, deps string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.approvals = append(f.approvals, approvalRec{to, skillID, skillName, deps})
	return nil
}

func (f *fakeEgress) sendCount() int    { f.mu.Lock(); defer f.mu.Unlock(); return len(f.sends) }
func (f *fakeEgress) approvalCount() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.approvals) }

// --- 注册表 / For 路由 ---

func TestRegisterAndFor(t *testing.T) {
	resetForTest()
	fe := &fakeEgress{name: "feishu"}
	we := &fakeEgress{name: "web"}
	Register(fe)
	Register(we)

	if e, ok := For("feishu"); !ok || e != fe {
		t.Errorf("For(feishu) = %v,%v want fe,true", e, ok)
	}
	if e, ok := For("web"); !ok || e != we {
		t.Errorf("For(web) = %v,%v want we,true", e, ok)
	}
	if _, ok := For("wechat"); ok {
		t.Error("For(unknown) should be (nil,false)")
	}
	if e, ok := For(""); ok {
		t.Errorf("For(empty) should be (nil,false), got %v", e)
	}
}

func TestRegisterIgnoresEmptyName(t *testing.T) {
	resetForTest()
	Register(&fakeEgress{name: ""})
	Register(nil)
	if len(Channels()) != 0 {
		t.Errorf("empty-name / nil egress should not register, channels=%v", Channels())
	}
}

// --- ReplyEvent 按 channel 分发到对应 egress（不串渠道）---

func TestDispatchSendRoutesByChannel(t *testing.T) {
	resetForTest()
	fe := &fakeEgress{name: "feishu"}
	we := &fakeEgress{name: "web"}
	Register(fe)
	Register(we)

	dispatchSendForTest("life1", "feishu", "open_id_A", "你好", "reply")

	if got := fe.sendCount(); got != 1 {
		t.Fatalf("feishu egress send count = %d want 1", got)
	}
	if we.sendCount() != 0 {
		t.Error("web egress should NOT receive a feishu-channel reply (cross-channel leak)")
	}
	if fe.sends[0].to != "open_id_A" || fe.sends[0].content != "你好" {
		t.Errorf("feishu got %+v", fe.sends[0])
	}
}

func TestDispatchSendUnknownChannelDrops(t *testing.T) {
	resetForTest()
	fe := &fakeEgress{name: "feishu"}
	Register(fe)
	// 未注册渠道 → 落日志、不误发
	dispatchSendForTest("life1", "discord", "u1", "hi", "reply")
	if fe.sendCount() != 0 {
		t.Error("unknown channel must not fall through to another egress")
	}
}

// 端到端：ReplyEvent 经 bus → StartDispatcher → 对应 egress。
func TestReplyEventEndToEnd(t *testing.T) {
	bus.Reset()
	resetForTest()
	fe := &fakeEgress{name: "feishu"}
	we := &fakeEgress{name: "web"}
	Register(fe)
	Register(we)
	StartDispatcher()

	bus.Publish(reflex.ReplyEvent{LifeID: "L", Channel: "web", To: "sess1", Content: "嗨"})
	if we.sendCount() != 1 {
		t.Fatalf("web egress should get the web-channel reply, count=%d", we.sendCount())
	}
	if fe.sendCount() != 0 {
		t.Error("feishu egress must not get a web-channel reply")
	}
	if we.sends[0].to != "sess1" || we.sends[0].content != "嗨" {
		t.Errorf("web got %+v", we.sends[0])
	}
}

// --- 空 To / 空 channel 按渠道兜底（隐患②：不依赖全局 LastSender 串台）---

func TestResolveTargetFallbackByChannel(t *testing.T) {
	resetForTest()
	// 兜底解析器返回最近对端所在渠道 + 对端——据此选 egress，按渠道兜底而非写死飞书。
	setResolverForTest(func(lifeID string) (string, string, bool) {
		return "web", "recent_web_peer", true
	})
	ch, to := resolveTargetForTest("L", "", "")
	if ch != "web" || to != "recent_web_peer" {
		t.Errorf("fallback resolveTarget = %q,%q want web,recent_web_peer", ch, to)
	}
	// 事件自带 channel 时，优先用事件值，不走兜底。
	ch2, to2 := resolveTargetForTest("L", "feishu", "openid")
	if ch2 != "feishu" || to2 != "openid" {
		t.Errorf("explicit channel kept = %q,%q want feishu,openid", ch2, to2)
	}
}

func TestDispatchSendEmptyChannelFallsBackToResolvedChannel(t *testing.T) {
	resetForTest()
	fe := &fakeEgress{name: "feishu"}
	we := &fakeEgress{name: "web"}
	Register(fe)
	Register(we)
	// 最近对端在 web → 空 channel 的回复应路由到 web egress（按渠道兜底）。
	setResolverForTest(func(lifeID string) (string, string, bool) {
		return "web", "peer_web", true
	})
	dispatchSendForTest("L", "", "", "回声", "reply")
	if we.sendCount() != 1 || fe.sendCount() != 0 {
		t.Fatalf("empty-channel reply should go to resolved web egress; web=%d feishu=%d",
			we.sendCount(), fe.sendCount())
	}
	if we.sends[0].to != "peer_web" {
		t.Errorf("fallback to = %q want peer_web", we.sends[0].to)
	}
}

// --- 审批按来源渠道路由（隐患①：不写死飞书）---

func TestDispatchApprovalRoutesBySourceChannel(t *testing.T) {
	resetForTest()
	// 飞书实现 ApprovalSender → 用富交互卡片。
	feishu := approvalEgress{&fakeEgress{name: "feishu"}}
	// web 不实现 ApprovalSender → 退化为文本 Send。
	web := &fakeEgress{name: "web"}
	Register(feishu)
	Register(web)

	// 来源 = feishu → 走审批卡片，不串到 web。
	dispatchApprovalForTest(skill.ApprovalNeededEvent{
		LifeID: "L", SkillID: "s1", SkillName: "demo", Deps: "python: numpy",
		Channel: "feishu", To: "openid_A",
	})
	if feishu.approvalCount() != 1 {
		t.Fatalf("feishu approval card count = %d want 1", feishu.approvalCount())
	}
	if feishu.approvals[0].to != "openid_A" || feishu.approvals[0].skillID != "s1" {
		t.Errorf("feishu approval got %+v", feishu.approvals[0])
	}
	if web.sendCount() != 0 || feishu.sendCount() != 0 {
		t.Error("approval to feishu must not leak as text Send anywhere")
	}

	// 来源 = web → 无 ApprovalSender，退化为文本提示 Send（不写死飞书）。
	dispatchApprovalForTest(skill.ApprovalNeededEvent{
		LifeID: "L", SkillID: "s2", SkillName: "demo2", Deps: "node: axios",
		Channel: "web", To: "sess_w",
	})
	if web.sendCount() != 1 {
		t.Fatalf("web approval should fall back to text Send, count=%d", web.sendCount())
	}
	if feishu.approvalCount() != 1 {
		t.Error("web approval must NOT route to feishu approval card")
	}
}

func TestDispatchApprovalEmptyChannelFallback(t *testing.T) {
	resetForTest()
	feishu := approvalEgress{&fakeEgress{name: "feishu"}}
	Register(feishu)
	// 来源上下文缺失（如 boot ScanDir）→ 按渠道兜底最近对端。
	setResolverForTest(func(lifeID string) (string, string, bool) {
		return "feishu", "openid_recent", true
	})
	dispatchApprovalForTest(skill.ApprovalNeededEvent{
		LifeID: "L", SkillID: "s3", SkillName: "d3", Deps: "python: pandas",
	})
	if feishu.approvalCount() != 1 || feishu.approvals[0].to != "openid_recent" {
		t.Fatalf("empty-source approval should fall back by channel to feishu peer; approvals=%v",
			feishu.approvals)
	}
}

func TestDispatchApprovalNoChannelNoFallbackDrops(t *testing.T) {
	resetForTest()
	feishu := approvalEgress{&fakeEgress{name: "feishu"}}
	Register(feishu)
	setResolverForTest(func(lifeID string) (string, string, bool) {
		return "", "", false // 无最近对端
	})
	dispatchApprovalForTest(skill.ApprovalNeededEvent{LifeID: "L", SkillID: "s4", SkillName: "d4"})
	if feishu.approvalCount() != 0 {
		t.Error("no source + no fallback → drop (panel approval), must not misfire to feishu")
	}
}

func TestDispatchSendSurfacesEgressError(t *testing.T) {
	resetForTest()
	fe := &fakeEgress{name: "feishu", sendErr: errors.New("boom")}
	Register(fe)
	// 不应 panic；错误落日志即可（这里只验证调用路径不崩）。
	dispatchSendForTest("L", "feishu", "x", "y", "reply")
	if fe.sendCount() != 1 {
		t.Error("send should still be attempted even if it errors")
	}
}
