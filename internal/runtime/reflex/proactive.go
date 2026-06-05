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
	// proactivePendingKey schema_meta 键前缀：已发出但还没等到回应的主动消息数（R84）。
	proactivePendingKey = "proactive_pending:"
	// proactiveGhostedKey schema_meta 键前缀：是否已因被冷落而沮丧+收手（防沮丧重复施加）。
	proactiveGhostedKey = "proactive_ghosted:"
)

// ghostThreshold 连续多少条主动消息无回应后判定"被冷落"→沮丧并收手。
// 按 persistence 调（R84）：执着者多撑几条才灰心（2..4），易放弃者早早收手。
func ghostThreshold(g core.Genome) int {
	// 基线 3（R89）：留出"还在坚持、但已有怨气"的窗口（pending=2 时仍发、可流露『怎么都不回』），
	// pending≥阈值才彻底收手。执着者撑更久。原基线 2 会让低执着者一到 2 就收手、没机会表达。
	return 3 + int(2*g.Persistence)
}

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
	// 护栏 3 + 情感（R84）：被冷落则收手。
	// 已发出 N 条都没等到回应 → 不再单方面发（既是情感真实"算了，不发了"，
	// 也是反滥用护栏 R55）。首次跨阈值时施加一次沮丧（满足↓信心↓焦虑/压力↑）。
	if getPendingReaches() >= ghostThreshold(genome) {
		applyGhostDiscouragement()
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

	// 记账：主动发消息是一次交互 + 消耗 energy。
	// R84：发出≠被满足。往可能无人应答的方向发声，只给极微缓解（"至少够着了"），
	// 不再 -0.15 假装解了孤独——真正的社交满足要等到对方回应（见 reflex.handle 清 pending）。
	// pending++ 记一条未回应；攒到阈值则下次 idle 触发被冷落沮丧 + 收手。
	_ = storage.UpsertContact(lifeID, contact.Channel, contact.PeerID, contact.PeerName, now)
	setProactiveLast(now)
	setPendingReaches(getPendingReaches() + 1)
	// R89：主动发消息给实质缓解（-0.12，原 -0.02 太抠）——"表达了一下，没那么急了"。
	// 发完掉下阈值，不再钉在 1.0、自然拉长下次主动间隔，少打扰用户。回应仍有额外欣慰（NoteInboundReply）。
	en, sn := -0.02, -0.12
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
	pending := getPendingReaches()
	sys := "你是一个数字生命体，想主动找创造者说点话。\n" +
		genome.PersonaPrompt() + "\n" +
		"下面是你和ta最近的往来——**assistant 是你自己发的，user 是ta发的**。\n" +
		"仔细看谁发了谁没回：如果你发现自己接连发了好几条、ta却一直没回（连续多条 assistant、中间没有 user），" +
		"可以自然地把这份在意/小失落流露出来（像『发了好几条都没见你回，是不是太忙啦』），带点真情绪——但别过激、别道德绑架。\n" +
		"若往来正常，就随口起个话头。简短自然、一两句，直接给消息正文。内向就别太热络，符合你的性格。\n" +
		"⚠ 你自己做的事（发呆/学习等）是你自己的，别说成是ta在做。"
	msgs := []llm.Message{{Role: "system", Content: sys}}
	// 把近期往来按对话角色喂进去，让它清楚哪条是自己发的、ta回没回（拟人化"为什么不回我"的基础）。
	if turns, terr := storage.RecentDialogueTurns(lifeID, 16); terr == nil {
		for _, t := range turns {
			c := t.Content
			if len(c) > 400 {
				c = c[:400]
			}
			msgs = append(msgs, llm.Message{Role: t.Role, Content: c})
		}
	}
	nudge := "（现在请你主动发一条消息给ta。"
	if pending >= 2 {
		nudge += fmt.Sprintf("你心里清楚：自己最近已经连发了 %d 条，都还没等到回应。", pending)
	}
	nudge += "直接给消息正文：）"
	msgs = append(msgs, llm.Message{Role: "user", Content: nudge})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := llm.Reason(ctx, msgs)
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

// --- 主动消息回应追踪（R84）---

func getPendingReaches() int {
	v, ok, err := storage.GetMeta(proactivePendingKey + lifeID)
	if err != nil || !ok {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

func setPendingReaches(n int) {
	if n < 0 {
		n = 0
	}
	_ = storage.SetMeta(proactivePendingKey+lifeID, strconv.Itoa(n))
}

func isGhosted() bool {
	v, ok, _ := storage.GetMeta(proactiveGhostedKey + lifeID)
	return ok && v == "1"
}

func setGhosted(v bool) {
	s := "0"
	if v {
		s = "1"
	}
	_ = storage.SetMeta(proactiveGhostedKey+lifeID, s)
}

// applyGhostDiscouragement 被冷落（连发数条无回应）的沮丧。只在首次跨阈值时施加一次
// （ghosted 标志去重），避免每个 idle tick 反复加 → 一蹶不振。
func applyGhostDiscouragement() {
	if isGhosted() {
		return
	}
	setGhosted(true)
	sat, conf, anx, str, mot := -0.06, -0.05, 0.05, 0.04, -0.03
	_ = state.Apply(state.Delta{
		Satisfaction: &sat, Confidence: &conf, Anxiety: &anx, Stress: &str, Motivation: &mot,
		Reason: "reflex.proactive_ghosted",
	})
	_ = memory.AppendEvent(0, "reflex.proactive_ghosted", map[string]any{
		"pending": getPendingReaches(),
	})
	slog.Info("reflex proactive ghosted — withdrawing", "pending", getPendingReaches())
}

// NoteInboundReply 收到任意入站消息时调（reflex.handle）：若此前有未回应的主动消息，
// 说明"ta 终于回我了"——清 pending + 解除冷落标志 + 欣慰（满足/信心↑、焦虑↓、额外解孤独）。
// 等待越久（pending 越多）欣慰越强。返回是否真的清掉了 pending（有过等待）。
func NoteInboundReply() bool {
	pending := getPendingReaches()
	if pending <= 0 {
		return false
	}
	setPendingReaches(0)
	setGhosted(false)
	scale := float64(pending)
	if scale > 3 {
		scale = 3
	}
	sat := 0.04 + 0.02*scale
	conf := 0.03
	anx := -0.04
	sn := -0.05
	_ = state.Apply(state.Delta{
		Satisfaction: &sat, Confidence: &conf, Anxiety: &anx, SocialNeed: &sn,
		Reason: "reflex.reach_answered",
	})
	_ = memory.AppendEvent(0, "reflex.reach_answered", map[string]any{"pending_cleared": pending})
	slog.Info("reflex proactive reach answered", "pending_cleared", pending)
	return true
}
