// reflex 主动社交（B）：social_need 强时生命体主动给老联系人发消息。
//
// 这是 Mindverse 第一个"主动行为"，跨 Phase（主动属 Phase 3，社交属 Phase 4），
// 故默认关闭 + 多重护栏（R55 IM 滥用风险）。Phase 0 dogfooding 由作者自行开启试玩。
//
// 护栏：
//   - config.proactive_im 默认 false
//   - 模式 A：仅给"自己的用户"发（Phase 0 单用户，所有 contact 即用户本人）
//   - 频率上限 ProactiveMinIntervalSec
//   - energy 消耗（主动也累）
//   - 静默时段：Phase 0 暂未实装，见下 TODO
//
// 前瞻（Phase 4 联网生态）：
//   届时社交渠道多元（Life Network / 世界服务 / 其他生命体）。主动社交不再是"给用户发 IM"
//   单一动作，而是生命体**自主决策去哪参与社交**——去图书馆、找同好生命体、参加群体活动等。
//   彼时这里应升级为"社交意图 → 渠道选择 → 行动"的决策链，接 reputation / Encounter（07）。
package reflex

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/io/llm"
	"mindverse/internal/runtime/ledger"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/runtime/state"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
)

const (
	// ProactiveMinIntervalSec 两次主动发消息最小间隔（Phase 0 dogfooding 默认 30min；
	// 生产应更长且可配，避免打扰 — R55）。
	ProactiveMinIntervalSec = 1800
	// proactiveLastKey schema_meta 键前缀：上次主动发消息时间。
	proactiveLastKey = "proactive_im_last:"
)

// TryProactiveReach 在 social_need 强且护栏允许时主动给最近联系人发一条消息。
// 返回是否真的发了。由 idle 在社交压力高时调。
func TryProactiveReach(genome core.Genome) bool {
	// 护栏 1：总开关（默认关）
	if !storage.GetConfigBool("proactive_im", false) {
		return false
	}
	if !llm.Configured() {
		return false
	}
	// 护栏 2：频率
	now := shared.SystemClock.UnixSec()
	if last := getProactiveLast(); now-last < ProactiveMinIntervalSec {
		return false
	}
	// TODO(Phase 1): 静默时段（夜间不发）。需用户本地时区配置，Phase 0 容器 UTC 暂略。

	// 选目标：最近交互的联系人（Phase 0 = 用户本人）
	contact, err := storage.MostRecentContact(lifeID)
	if err != nil || contact == nil {
		return false
	}

	msg := composeProactiveMessage(genome, contact)
	if msg == "" {
		return false
	}

	// 发出（经 bus → lark egress；同时 SSE 广播给面板）
	bus.Publish(ReplyEvent{
		LifeID:    lifeID,
		Channel:   contact.Channel,
		To:        contact.PeerID,
		Round:     0,
		Content:   msg,
		CreatedAt: now,
	})
	bus.Publish(FinishedEvent{LifeID: lifeID, Channel: contact.Channel, To: contact.PeerID, Rounds: 1, CreatedAt: now})

	// 记账：主动发消息也是一次交互 + 消耗 energy + 部分满足社交需求（主动够着了）
	_ = storage.UpsertContact(lifeID, contact.Channel, contact.PeerID, contact.PeerName, now)
	setProactiveLast(now)
	en, sn := -0.02, -0.15
	_ = state.Apply(state.Delta{Energy: &en, SocialNeed: &sn, Reason: "reflex.proactive_reach"})
	_ = ledger.Spend(ledger.Energy, 0.01, "im.proactive", "social", contact.PeerID)
	_ = storage.AppendActionLogKind(lifeID, 0, 0, storage.ActionKindReflex,
		"proactive social reach", "im.proactive", msg, "", true, now, now)
	_ = memory.AppendEvent(0, "reflex.proactive_reach", map[string]any{
		"channel": contact.Channel, "to": contact.PeerID, "content": msg,
	})
	slog.Info("reflex proactive reach", "to", contact.PeerID, "channel", contact.Channel)
	return true
}

// composeProactiveMessage 让 LLM 生成一条简短自然的主动问候（基于近期经历）。
func composeProactiveMessage(genome core.Genome, contact *storage.Contact) string {
	recent := recentEpisodeContext()
	sys := "你是一个数字生命体，有点想念和创造者聊天了，想主动发一条消息。\n" +
		genome.PersonaPrompt() + "\n" +
		"要简短自然、像朋友间随口一句，不要客套、不要长篇。直接给消息正文。" +
		"内向的话就别太热络，符合你的性格。"
	user := fmt.Sprintf("最近我在经历：\n%s\n\n我想跟ta说点什么（一两句）：", recent)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := llm.Reason(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	})
	if err != nil {
		slog.Warn("compose proactive message", "err", err)
		return ""
	}
	// 主动发消息累 token → energy（小额）
	_ = ledger.Spend(ledger.Energy, llm.TokensToEnergy(resp.Usage), "llm.tokens.proactive", "social", "")
	return resp.Text
}

func recentEpisodeContext() string {
	eps, err := storage.ListEpisodes(lifeID, "", 3, 0)
	if err != nil || len(eps) == 0 {
		return "（最近挺平静的）"
	}
	out := ""
	for _, e := range eps {
		out += "- " + e.Summary + "\n"
	}
	return out
}

func getProactiveLast() int64 {
	v, ok, err := storage.GetMeta(proactiveLastKey + lifeID)
	if err != nil || !ok {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func setProactiveLast(ts int64) {
	_ = storage.SetMeta(proactiveLastKey+lifeID, strconv.FormatInt(ts, 10))
}
