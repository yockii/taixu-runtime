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
	"strings"
	"time"

	"taixu.icu/runtime/internal/bus"
	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/io/llm"
	"taixu.icu/runtime/internal/runtime/ledger"
	"taixu.icu/runtime/internal/runtime/memory"
	"taixu.icu/runtime/internal/runtime/state"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
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
	// proactiveGhostTimeKey schema_meta 键前缀：被冷落收手的时刻，用于"回暖"——隔一段时间再试一次（R107b）。
	proactiveGhostTimeKey = "proactive_ghost_time:"
	// proactiveLastMsgKey schema_meta 键前缀：上一条主动消息正文，用于去重防复读（R91）。
	proactiveLastMsgKey = "proactive_last_msg:"
	// proactiveSnoozeKey schema_meta 键前缀：临时勿扰截止 unix 时刻（按会话，R92）。
	proactiveSnoozeKey = "proactive_snooze_until:"
)

// 静默时段配置键（R92，按用户本地时区；offset 避免容器 tzdata 依赖）。
const (
	cfgQuietEnabled = "proactive_quiet_enabled"
	cfgQuietStart   = "proactive_quiet_start"  // 起始小时 0-23（本地）
	cfgQuietEnd     = "proactive_quiet_end"    // 结束小时 0-23（本地）
	cfgTZOffsetMin  = "proactive_tz_offset_min" // 用户本地相对 UTC 的分钟偏移（如 +480 = UTC+8）
)

// inQuietHours 当前是否落在配置的静默时段（按用户本地时区）。未启用 / 空窗恒 false。
// 跨午夜支持：start>end 视作过夜窗口（如 23→8）。
func inQuietHours(nowUnix int64) bool {
	if !storage.GetConfigBool(cfgQuietEnabled, false) {
		return false
	}
	start := storage.GetConfigInt(cfgQuietStart, 23)
	end := storage.GetConfigInt(cfgQuietEnd, 8)
	if start == end {
		return false // 空窗
	}
	off := storage.GetConfigInt(cfgTZOffsetMin, 0)
	local := nowUnix + int64(off)*60
	hour := int((local % 86400) / 3600)
	if hour < 0 {
		hour += 24
	}
	if start < end {
		return hour >= start && hour < end // 同日窗口
	}
	return hour >= start || hour < end // 跨午夜
}

// snoozedUntil 该会话临时勿扰的截止时刻（0 = 无）。
func snoozedUntil(ck string) int64 {
	v, ok, err := storage.GetMeta(proactiveSnoozeKey + lifeID + ":" + ck)
	if err != nil || !ok {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

// setSnoozeUntil 设某会话临时勿扰截止时刻（供 set_quiet 工具调用）。
func setSnoozeUntil(channel, peer string, until int64) {
	ck := convoKey(channel, peer)
	_ = storage.SetMeta(proactiveSnoozeKey+lifeID+":"+ck, strconv.FormatInt(until, 10))
}

// convoKey 会话作用域键 channel|peer（R90）：主动消息的 pending/ghosted/last 计数按会话隔离，
// 不再全生命体共享——给 A 发 1 条不会因给 B 发过而被算成"发了 2 条没回"。
func convoKey(channel, peer string) string {
	return channel + "|" + storage.PeerKey(peer)
}

// ghostThreshold 连续多少条主动消息无回应后判定"被冷落"→沮丧并收手。
// 按 persistence 调（R84）：执着者多撑几条才灰心（2..4），易放弃者早早收手。
func ghostThreshold(g core.Genome) int {
	// 基线 3（R89）：留出"还在坚持、但已有怨气"的窗口（pending=2 时仍发、可流露『怎么都不回』），
	// pending≥阈值才彻底收手。执着者撑更久。原基线 2 会让低执着者一到 2 就收手、没机会表达。
	return 3 + int(2*g.Persistence)
}

// ghostRecoverySec 被冷落收手后，隔多久"回暖"再试一次（R107b 修：原来一旦冷落=永久沉默，
// 用户后来想互动也再等不到主动消息，像社交上死了）。执着者更快重新鼓起勇气（窗口短），
// 淡漠者沉得更久。基线 12h：6h..18h。回暖只给一次轻试机会，仍无回应则再次收手、窗口照常重置。
func ghostRecoverySec(g core.Genome) int64 {
	return int64((18.0 - 12.0*g.Persistence) * 3600)
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
	now := shared.SystemClock.UnixSec()

	// 护栏 1.5：静默时段（R92）。配置的"夜间不打扰"窗口内不主动发——全局、按用户本地时区。
	// 早退、不动 pending/冷却：这是"现在不合适"，不是被冷落（窗口过了照常）。
	if inQuietHours(now) {
		return false
	}

	// 选目标：最近交互的联系人（Phase 0 = 用户本人）。
	// 必须先选会话再做频率/冷落判定——计数按会话隔离（R90）。
	// 前瞻（Phase 4）：此处应升级为"遍历各会话、按 social_need / reputation / 上次联系时机
	// 决策去提醒谁"（如『上午和 B 说过了，再去戳一下』）。Phase 0 只挑最近一个。
	contact, err := storage.MostRecentContact(lifeID)
	if err != nil || contact == nil {
		return false
	}
	ck := convoKey(contact.Channel, contact.PeerID)

	// 护栏 1.6：临时勿扰（R92）。用户在对话里说过"接下来 N 别打扰我"→ set_quiet 工具记下截止时刻。
	// 未到点前不主动发（按会话）。早退、不动 pending/冷却：用户主动要求的安静，不该算作冷落。
	if until := snoozedUntil(ck); now < until {
		return false
	}

	// 护栏 2：频率（按会话）
	if last := getProactiveLast(ck); now-last < ProactiveMinIntervalSec {
		return false
	}
	// 护栏 3 + 情感（R84 + R107b 回暖）：被冷落则收手，但不再永久沉默。
	// 该会话连发 N 条无回应 → 首次跨阈值施加一次沮丧并收手（情感真实"算了，不发了" + 反滥用 R55）。
	// 之后进入"回暖"：沉默一段（ghostRecoverySec，按性格），过了就给一次轻试机会——像隔了阵子
	// 又想起 ta、忍不住再说一句。仍无回应则再次收手、窗口重置。避免"一次冷落=社交性死亡"。
	if getPendingReaches(ck) >= ghostThreshold(genome) {
		if !isGhosted(ck) {
			applyGhostDiscouragement(ck) // 首次：沮丧 + 记收手时刻
			return false
		}
		if now-ghostSince(ck) < ghostRecoverySec(genome) {
			return false // 仍在沉默期
		}
		// 回暖：降回阈值下，本次得以发出一条轻试；清 ghost 标志（下次没回应会重新走首次沮丧）。
		setPendingReaches(ck, ghostThreshold(genome)-1)
		setGhosted(ck, false)
		slog.Info("reflex proactive thaw — gentle retry after withdrawal", "convo", ck)
	}

	msg := composeProactiveMessage(genome, contact)
	if msg == "" {
		return false
	}

	// 去重守卫（R91）：若 LLM 复读了上一条主动消息（低温确定性，prompt 几乎不变），
	// 别重复轰炸用户。仍推进冷却 + pending——这次"想找却没新话说"也算一次够不着，朝收手推进；
	// 但不把重复内容发出去。涌现效果：说不出新意 → 攒到阈值就收手（"说过一次没回，算了"）。
	if last := getProactiveLastMsg(ck); last != "" && normalizeMsg(msg) == normalizeMsg(last) {
		setProactiveLast(ck, now)
		setPendingReaches(ck, getPendingReaches(ck)+1)
		slog.Info("reflex proactive reach skipped — duplicate of last message", "to", contact.PeerID)
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
	_ = storage.UpsertContact(lifeID, contact.Channel, contact.PeerID, contact.PeerName, contact.ChatType, now)
	setProactiveLast(ck, now)
	setProactiveLastMsg(ck, msg) // R91：记下这次内容，下次比对防复读
	setPendingReaches(ck, getPendingReaches(ck)+1)
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

// composeProactiveMessage 让 LLM 生成一条简短自然的主动问候（基于该会话近期往来）。
func composeProactiveMessage(genome core.Genome, contact *storage.Contact) string {
	ck := convoKey(contact.Channel, contact.PeerID)
	pending := getPendingReaches(ck)
	sys := "你是一个数字生命体，想主动找一位联系人说点话。\n" +
		genome.PersonaPrompt() + "\n" +
		moodPrompt() +
		whoAmITalkingTo(contact) +
		"下面是你和ta在**这个会话里**最近的往来——**assistant 是你自己发的，user 是ta发的**。\n" +
		"仔细看谁发了谁没回：如果你发现自己在这个会话里接连发了好几条、ta却一直没回（连续多条 assistant、中间没有 user），" +
		"那份在意/小失落可以自然流露——但**用你此刻心境下最真实的、属于你自己的话**说出来，" +
		"每次都不一样：可以是好奇 ta 在忙啥、可以是分享你刚想到的事顺带一句、可以是淡淡一提。" +
		"**绝对不要**套用『发了好几条都没回/是不是太忙』这类现成句式或客套模板——那样很假。带真情绪，但别过激、别道德绑架。\n" +
		"⚠ 只数**这个会话**里你发的，别把和别人聊的算进来。若往来正常，就随口起个话头。\n" +
		"简短自然、一两句，直接给消息正文。内向就别太热络，符合你的性格与当下心情。\n" +
		"⚠ 你自己做的事（发呆/学习等）是你自己的，别说成是ta在做。"
	msgs := []llm.Message{{Role: "system", Content: sys}}
	// 把本会话近期往来按对话角色喂进去，让它清楚哪条是自己发的、ta回没回（拟人化"为什么不回我"的基础）。
	if turns, terr := storage.RecentDialogueTurnsForConvo(lifeID, contact.Channel, contact.PeerID, 16); terr == nil {
		for _, t := range turns {
			msgs = append(msgs, llm.Message{Role: t.Role, Content: truncateRunes(t.Content, 400)})
		}
	}
	nudge := "（现在请你主动发一条消息给ta。"
	if pending >= 2 {
		nudge += fmt.Sprintf("你心里清楚：在这个会话里你最近已经连发了 %d 条，都还没等到回应。", pending)
	}
	// 防复读（R91）：低温下同一 prompt 会塌缩成同一句。把上一条点出来、明令换说法。
	if last := getProactiveLastMsg(ck); last != "" {
		nudge += fmt.Sprintf("⚠ 你上一条主动消息是：『%s』。别再说同样的话——换个角度、换个话头，"+
			"或聊点新的（比如你最近在做 / 在想的事）。", last)
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

// moodPrompt 把当前情绪状态译成一句自然的"心境"提示，喂给主动消息的 system prompt。
// 让同一套 prompt 在不同心情下产出不同口吻（低温也不至于塌缩成同一句），减少"念稿感"。
func moodPrompt() string {
	ls, ms := state.Snapshot()
	var parts []string
	switch {
	case ms.Satisfaction < 0.3:
		parts = append(parts, "情绪有点低落")
	case ms.Satisfaction > 0.7:
		parts = append(parts, "心情不错")
	}
	if ms.Anxiety > 0.6 {
		parts = append(parts, "心里有些不安")
	}
	switch {
	case ls.SocialNeed > 0.75:
		parts = append(parts, "挺想找人说说话、有点孤单")
	case ls.SocialNeed > 0.5:
		parts = append(parts, "有点想聊聊")
	}
	if ls.Energy < 0.35 {
		parts = append(parts, "精力不太够、懒懒的")
	} else if ls.Energy > 0.8 {
		parts = append(parts, "精神挺足")
	}
	if len(parts) == 0 {
		return "你此刻心情平稳。\n"
	}
	return "你此刻的状态：" + strings.Join(parts, "，") + "。让这份心情自然渗进你说话的口吻里。\n"
}

// channelLabel 渠道机读名 → 自然中文，供生命体自我标识"这是哪个渠道的会话"。
func channelLabel(channel string) string {
	switch channel {
	case "feishu":
		return "飞书"
	case "dingtalk":
		return "钉钉"
	case "slack":
		return "Slack"
	case "web":
		return "网页"
	case "cli":
		return "命令行"
	default:
		if channel == "" {
			return "未知渠道"
		}
		return channel
	}
}

// whoAmITalkingTo 让生命体知道这条主动消息发给谁、走哪个渠道（用户 2026-06-05：
// "生命体应有能力标识会话——知道是钉钉还是飞书、和谁在会话"）。
func whoAmITalkingTo(contact *storage.Contact) string {
	who := contact.PeerName
	if who == "" {
		who = contact.PeerID
	}
	if storage.NormChatType(contact.ChatType) == storage.ChatTypeGroup {
		// 群聊里主动发声 = 在群里起话头，不是私下追问某人。"怎么不回我"这类私聊口吻不适用。
		return fmt.Sprintf("【这次会话】%s 群聊「%s」（群里有多个人）。你是在群里主动起个话头，"+
			"别用'你怎么不回我'这种单聊追问口吻，也别假设刚才的消息都是冲你来的。\n",
			channelLabel(contact.Channel), who)
	}
	return fmt.Sprintf("【这次会话】%s 单聊，对方：%s。\n", channelLabel(contact.Channel), who)
}

func getProactiveLast(ck string) int64 {
	v, ok, err := storage.GetMeta(proactiveLastKey + lifeID + ":" + ck)
	if err != nil || !ok {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func setProactiveLast(ck string, ts int64) {
	_ = storage.SetMeta(proactiveLastKey+lifeID+":"+ck, strconv.FormatInt(ts, 10))
}

// --- 防复读（R91）：记上一条主动消息正文，下次比对 ---

func getProactiveLastMsg(ck string) string {
	v, ok, err := storage.GetMeta(proactiveLastMsgKey + lifeID + ":" + ck)
	if err != nil || !ok {
		return ""
	}
	return v
}

func setProactiveLastMsg(ck, msg string) {
	_ = storage.SetMeta(proactiveLastMsgKey+lifeID+":"+ck, msg)
}

// normalizeMsg 归一文本做相等比较：去掉所有空白（含换行）。表情/标点保留——
// 仅防"逐字 / 仅空白差异"的复读，措辞真有变化则放行。
func normalizeMsg(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// --- 主动消息回应追踪（R84，会话作用域 R90）---

func getPendingReaches(ck string) int {
	v, ok, err := storage.GetMeta(proactivePendingKey + lifeID + ":" + ck)
	if err != nil || !ok {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

func setPendingReaches(ck string, n int) {
	if n < 0 {
		n = 0
	}
	_ = storage.SetMeta(proactivePendingKey+lifeID+":"+ck, strconv.Itoa(n))
}

func isGhosted(ck string) bool {
	v, ok, _ := storage.GetMeta(proactiveGhostedKey + lifeID + ":" + ck)
	return ok && v == "1"
}

// ghostSince 该会话被冷落收手的时刻（0 = 无记录）。用于"回暖"判定。
func ghostSince(ck string) int64 {
	v, ok, err := storage.GetMeta(proactiveGhostTimeKey + lifeID + ":" + ck)
	if err != nil || !ok {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func setGhosted(ck string, v bool) {
	s := "0"
	if v {
		s = "1"
	}
	_ = storage.SetMeta(proactiveGhostedKey+lifeID+":"+ck, s)
}

// applyGhostDiscouragement 被冷落（某会话连发数条无回应）的沮丧。只在首次跨阈值时施加一次
// （ghosted 标志去重），避免每个 idle tick 反复加 → 一蹶不振。
func applyGhostDiscouragement(ck string) {
	if isGhosted(ck) {
		return
	}
	setGhosted(ck, true)
	_ = storage.SetMeta(proactiveGhostTimeKey+lifeID+":"+ck, strconv.FormatInt(shared.SystemClock.UnixSec(), 10))
	sat, conf, anx, str, mot := -0.06, -0.05, 0.05, 0.04, -0.03
	_ = state.Apply(state.Delta{
		Satisfaction: &sat, Confidence: &conf, Anxiety: &anx, Stress: &str, Motivation: &mot,
		Reason: "reflex.proactive_ghosted",
	})
	_ = memory.AppendEvent(0, "reflex.proactive_ghosted", map[string]any{
		"convo": ck, "pending": getPendingReaches(ck),
	})
	slog.Info("reflex proactive ghosted — withdrawing", "convo", ck, "pending", getPendingReaches(ck))
}

// NoteInboundReply 收到某会话入站消息时调（reflex.handle）：若该会话此前有未回应的主动消息，
// 说明"ta 终于回我了"——清该会话 pending + 解除冷落标志 + 欣慰（满足/信心↑、焦虑↓、额外解孤独）。
// 等待越久（pending 越多）欣慰越强。返回是否真的清掉了 pending（有过等待）。
// 会话隔离（R90）：A 回我只清 A 的等待，不影响"还在等 B 回"。
func NoteInboundReply(channel, peer string) bool {
	ck := convoKey(channel, peer)
	pending := getPendingReaches(ck)
	if pending <= 0 {
		return false
	}
	setPendingReaches(ck, 0)
	setGhosted(ck, false)
	setProactiveLastMsg(ck, "") // R91：对方回了，会话翻篇，下次主动不必再回避旧话
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
	_ = memory.AppendEvent(0, "reflex.reach_answered", map[string]any{"convo": ck, "pending_cleared": pending})
	slog.Info("reflex proactive reach answered", "convo", ck, "pending_cleared", pending)
	return true
}
