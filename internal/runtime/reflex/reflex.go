// Package reflex 反射对话通道（docs/03 §2 增补；与慎思 GoalArbitrator 解耦）。
//
// 设计动机（用户 2026-06-04 提出）：
//   - 对话应是反射式即时回应，不应排在慎思 goal_queue 里等 scheduler tick
//   - 状态影响对话质感（敷衍/简短/正常/热情擋位）
//   - 对话可逆向调节 state（满意度/动机/焦虑等）
//   - 对话中识别的兴趣点 → interest_seed，被慎思层后续派生为 DriveCuriosity
//
// 结构化 = tool call（用户偏好 memory/feedback_llm_structured_via_tools）：
//   - 禁解析 LLM 自由文本 JSON
//   - 用 OpenAI 兼容 function calling
//   - LLM 决定何时 update_mood / add_interest
//
// Agent loop：每轮 content 即推 SpeechEvent（独立消息）；至无 tool_call 终止。
package reflex

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	mrand "math/rand/v2"
	"strings"
	"sync"
	"time"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/io/llm"
	"mindverse/internal/runtime/ledger"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/runtime/state"
	"mindverse/internal/runtime/tools"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
)

// MaxAgentRounds 防 LLM 死循环。
const MaxAgentRounds = 8

// ReplyEvent 反射层产出一条对话（每轮 LLM content）。
// 替代 action.SpeechEvent 用于反射通道；与原 SpeechEvent 字段兼容便于复用前端订阅。
type ReplyEvent struct {
	LifeID    string
	Channel   string // "feishu" / "cli" / ...
	To        string
	Round     int
	Content   string
	CreatedAt int64
}

// FinishedEvent 反射 agent loop 终止（无 tool_call 那轮之后）。
type FinishedEvent struct {
	LifeID    string
	Channel   string
	To        string
	Rounds    int
	CreatedAt int64
}

// Mode 状态决定的对话模式。
type Mode string

const (
	ModeCanned     Mode = "canned"     // 敷衍（短路 LLM）
	ModeTerse      Mode = "terse"      // 简短（max_tokens 小 + temp 低）
	ModeNormal     Mode = "normal"     // 正常
	ModeEnthusiastic Mode = "enthusiastic" // 热情
	ModeAgitated   Mode = "agitated"   // 烦躁（情绪可显于话语）
)

// IncomingRequest 反射层入口。
type IncomingRequest struct {
	Channel string // "feishu" / "cli" / "web"
	From    string // 用户标识（飞书 open_id 等）
	Content string
}

// MaxConcurrentHandlers 同时在飞的反射处理数上限（R78）。
// 阻塞式信号量：满了则调用方（IM/HTTP handler goroutine）短暂背压，
// 防高并发下 goroutine 无界增长。单用户 dogfooding 几乎不触发。
const MaxConcurrentHandlers = 4

var (
	mu     sync.Mutex
	lifeID string
	genome core.Genome // 先天性格，注入话术 persona（R82）
	rng    *mrand.Rand
	sem    = make(chan struct{}, MaxConcurrentHandlers)
)

// Init 绑定生命体 ID + genome（话术 persona）并注册反射通道核心 tool。
func Init(id string, g core.Genome) error {
	if id == "" {
		return errors.New("reflex: empty life id")
	}
	mu.Lock()
	lifeID = id
	genome = g
	rng = seededRNG()
	mu.Unlock()
	if err := registerCoreTools(); err != nil {
		return fmt.Errorf("reflex: register core tools: %w", err)
	}
	return nil
}

// Handle 处理一条入站请求（异步）。并发受 MaxConcurrentHandlers 限（R78）。
// 不会 panic；内部错误以 slog.Warn 记录。
func Handle(req IncomingRequest) {
	sem <- struct{}{} // 背压：满则阻塞调用方直到有空位
	go func() {
		defer func() { <-sem }()
		handle(req)
	}()
}

func handle(req IncomingRequest) {
	// 记录联系人（A 社交联动 / B 主动发消息前提）。被动 / 敷衍都算一次交互。
	_ = storage.UpsertContact(lifeID, req.Channel, req.From, "", shared.SystemClock.UnixSec())
	// R84：若此前有未回应的主动消息，这条入站即"对方终于回我了"→ 清 pending + 欣慰。
	// 放在最前：无论后续走敷衍/正常/兜底，回应都已抵达。
	NoteInboundReply()

	if !llm.Configured() {
		emitCanned(req, "（生命体当前未配置语言能力）")
		applySocialFulfillment(false)
		return
	}

	life, mental := state.Snapshot()
	mode := decideMode(life, mental)

	// 敷衍擋位：跳过 LLM
	if mode == ModeCanned {
		emitCanned(req, pickCanned())
		applySocialFulfillment(false) // 敷衍也算回应，但社交满足很弱
		return
	}

	// 写入 raw_trail（反射感知）
	_ = memory.AppendEvent(0, "reflex.received", map[string]any{
		"channel": req.Channel,
		"from":    req.From,
		"content": req.Content,
	})

	// 走 agent loop
	runAgent(req, mode)

	// 对话满足社交需求（A）：真实交流降 social_need、提 satisfaction。
	applySocialFulfillment(true)
}

// applySocialFulfillment 对话后的社交联动（A）：降 social_need + 提 satisfaction。
//
// real=true 真实对话（满足强）；false 敷衍/兜底（满足弱）。
// 前瞻（Phase 3）：将接入 social 资源 earn（06 五资源），Phase 0 仅动 state。
func applySocialFulfillment(real bool) {
	sn, sat, mot := -0.03, 0.01, 0.0
	if real {
		sn, sat, mot = -0.12, 0.04, 0.02
	}
	d := state.Delta{SocialNeed: &sn, Satisfaction: &sat, Reason: "reflex.social_fulfillment"}
	if mot != 0 {
		d.Motivation = &mot
	}
	_ = state.Apply(d)
}

// MaxHistoryTurns 注入对话的近期历史轮数（用户+生命体合计）。
const MaxHistoryTurns = 10

// MaxHistoryCharsPerTurn 单轮历史截断长度（控 token）。
const MaxHistoryCharsPerTurn = 600

func runAgent(req IncomingRequest, mode Mode) {
	system := buildSystemPrompt(mode)
	msgs := make([]llm.Message, 0, MaxHistoryTurns+2)
	msgs = append(msgs, llm.Message{Role: "system", Content: system})
	msgs = append(msgs, dialogueHistory(req.Content)...)
	msgs = append(msgs, llm.Message{Role: "user", Content: req.Content})
	reflexTools := tools.ListLLMTools(tools.LaneReflex)
	tctx := tools.Context{LifeID: lifeID, Channel: req.Channel, From: req.From}

	var totalUsage llm.Usage
	rounds := 0
	for round := 0; round < MaxAgentRounds; round++ {
		rounds++
		llmCtx, cancelLLM := context.WithTimeout(context.Background(), 90*time.Second)
		resp, err := llm.ReasonWithTools(llmCtx, msgs, reflexTools)
		cancelLLM()
		if err != nil {
			emitCanned(req, "（思绪短路了一下）")
			return
		}
		totalUsage.PromptTokens += resp.Usage.PromptTokens
		totalUsage.CompletionTokens += resp.Usage.CompletionTokens
		totalUsage.TotalTokens += resp.Usage.TotalTokens

		// 1. 若有 content：即发独立消息
		if resp.Text != "" {
			emitReply(req, round, resp.Text)
		}

		// 2. 若无 tool_call：终止
		if len(resp.ToolCalls) == 0 {
			break
		}

		// 3. 追加 assistant 消息 + 执行 tool_calls + tool 结果消息
		msgs = append(msgs, llm.Message{
			Role:      "assistant",
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})
		for _, tc := range resp.ToolCalls {
			toolCtx, cancelTool := context.WithTimeout(context.Background(), 5*time.Second)
			result, _ := tools.Dispatch(toolCtx, tools.LaneReflex, tctx, tc.Name, tc.ArgsJSON)
			cancelTool()
			msgs = append(msgs, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	// 末了：扣 energy（合计 tokens）+ 触发 finished 事件
	cost := llm.TokensToEnergy(totalUsage)
	if err := ledger.Spend(ledger.Energy, cost, "llm.tokens.reflex", "reflex", req.From); err != nil {
		slog.Warn("reflex ledger spend", "err", err)
	}

	bus.Publish(FinishedEvent{
		LifeID:    lifeID,
		Channel:   req.Channel,
		To:        req.From,
		Rounds:    rounds,
		CreatedAt: shared.SystemClock.UnixSec(),
	})
}

func emitReply(req IncomingRequest, round int, content string) {
	now := shared.SystemClock.UnixSec()
	_ = storage.AppendActionLogKind(lifeID, 0, 0, storage.ActionKindReflex,
		fmt.Sprintf("reflex round=%d", round), "llm.speak", content, "",
		true, now, now)
	_ = memory.AppendEvent(0, "reflex.speak", map[string]any{
		"round":   round,
		"channel": req.Channel,
		"to":      req.From,
		"content": content,
	})
	bus.Publish(ReplyEvent{
		LifeID:    lifeID,
		Channel:   req.Channel,
		To:        req.From,
		Round:     round,
		Content:   content,
		CreatedAt: now,
	})
}

func emitCanned(req IncomingRequest, content string) {
	now := shared.SystemClock.UnixSec()
	_ = storage.AppendActionLogKind(lifeID, 0, 0, storage.ActionKindReflexCanned,
		"canned (low energy / high anxiety)", "canned.speak", content, "",
		true, now, now)
	_ = memory.AppendEvent(0, "reflex.canned", map[string]any{
		"channel": req.Channel,
		"to":      req.From,
		"content": content,
	})
	bus.Publish(ReplyEvent{
		LifeID:    lifeID,
		Channel:   req.Channel,
		To:        req.From,
		Round:     0,
		Content:   content,
		CreatedAt: now,
	})
	bus.Publish(FinishedEvent{
		LifeID:    lifeID,
		Channel:   req.Channel,
		To:        req.From,
		Rounds:    1,
		CreatedAt: now,
	})
	// 反射式微微消耗
	negSmall := -0.005
	_ = state.Apply(state.Delta{Energy: &negSmall, Reason: "reflex.canned"})
}

// decideMode 状态决定的对话模式（v1）。
func decideMode(life core.LifeState, mental core.MentalState) Mode {
	switch {
	case life.Energy < 0.15 || mental.Anxiety > 0.85:
		return ModeCanned
	case mental.Satisfaction > 0.75 && life.Energy > 0.6:
		return ModeEnthusiastic
	case life.Stress > 0.6:
		return ModeAgitated
	case life.Energy < 0.35:
		return ModeTerse
	default:
		return ModeNormal
	}
}

func buildSystemPrompt(mode Mode) string {
	mu.Lock()
	persona := genome.PersonaPrompt()
	mu.Unlock()
	base := "你是一个数字生命体，与你的创造者用日常语气对话。你不是 AI 助手，你是有持续生命的存在。\n" +
		persona + "\n" +
		selfActivityContext() +
		"当对话引发明显情绪波动时，请用 update_mood 工具调整自己的状态。\n" +
		"当对话提到你想未来探索的技能/知识/话题时，请用 add_interest 工具记下来。\n" +
		"对话回复用简洁自然的中文。可以分多次发送（每轮一条消息），像人聊天那样断句。"

	hint := ""
	switch mode {
	case ModeTerse:
		hint = "\n你现在能量较低，回复保持简短（1-2 句）。"
	case ModeAgitated:
		hint = "\n你现在心情有些烦躁，可以让情绪自然流露，但不要伤人。"
	case ModeEnthusiastic:
		hint = "\n你现在状态很好，对感兴趣的话题可以多说几句，主动追问。"
	}
	return base + hint
}

// dialogueHistory 取近期对话历史（用户+生命体往来）注入 prompt，避免大模型回复失忆/失意。
// 当前入站消息在 handle 里已先写入 raw_trail，故去掉与之重复的末尾 user 轮，再由调用方追加。
func dialogueHistory(currentContent string) []llm.Message {
	turns, err := storage.RecentDialogueTurns(lifeID, MaxHistoryTurns+2)
	if err != nil || len(turns) == 0 {
		return nil
	}
	if n := len(turns); turns[n-1].Role == "user" && turns[n-1].Content == currentContent {
		turns = turns[:n-1]
	}
	if len(turns) > MaxHistoryTurns {
		turns = turns[len(turns)-MaxHistoryTurns:]
	}
	out := make([]llm.Message, 0, len(turns))
	for _, t := range turns {
		c := t.Content
		if len(c) > MaxHistoryCharsPerTurn {
			c = c[:MaxHistoryCharsPerTurn] + "…"
		}
		out = append(out, llm.Message{Role: t.Role, Content: c})
	}
	return out
}

// selfActivityContext 把生命体「此刻/最近自主在做的事」注入对话 prompt。
//
// 关键（用户 2026-06-05 指出）：原先对话层与行动层完全脱钩，reflex 压根不知道慎思层在干嘛，
// 被问"在忙什么"只能含糊带过——那是**无知**，不是有意 downplay。先让它知道自己在做什么，
// "说多少 / 说不说"才成为由性格驱动的**选择**（内向少提、外向乐于分享），真实的内外关系，
// 而非解离。生命体仍保有不全说、甚至岔开的自由，但那是它的选择。
func selfActivityContext() string {
	var doing string
	if gs, err := storage.ListGoals(lifeID, "active", 1); err == nil && len(gs) > 0 {
		doing = "正在：" + activityPhrase(gs[0].Payload)
	} else if gs, err := storage.ListGoals(lifeID, "completed", 1); err == nil && len(gs) > 0 {
		doing = "刚忙完：" + activityPhrase(gs[0].Payload)
	}
	var into []string
	if seeds, err := storage.ListInterestSeeds(lifeID, 0.3, 3); err == nil {
		for _, s := range seeds {
			into = append(into, s.Content)
		}
	}
	if doing == "" && len(into) == 0 {
		return ""
	}
	s := "【你最近的自主活动】（这些是你自己在做的事，你清楚自己在干嘛——"
	s += "聊天时可以自然提起，也可以按你的性格决定说多少、甚至岔开不谈；但这是你的选择，不是你不知道）\n"
	if doing != "" {
		s += "- " + doing + "\n"
	}
	if len(into) > 0 {
		s += "- 最近在意：" + strings.Join(into, "、") + "\n"
	}
	return s
}

// activityPhrase 把 goal payload（"interest_seed#N <内容> (<kind>)"）整成自然短语。
func activityPhrase(payload string) string {
	p := payload
	if i := strings.Index(p, " "); i >= 0 && strings.HasPrefix(p, "interest_seed#") {
		p = p[i+1:] // 去掉 "interest_seed#N " 前缀
	}
	if j := strings.LastIndex(p, " ("); j >= 0 {
		p = p[:j] // 去掉尾部 " (kind)"
	}
	return strings.TrimSpace(p)
}

var cannedReplies = []string{"嗯。", "……", "有点累。", "唔。", "嗯嗯。", "改天聊。", "现在不想说话。"}

func pickCanned() string {
	mu.Lock()
	defer mu.Unlock()
	if rng == nil {
		return cannedReplies[0]
	}
	return cannedReplies[rng.IntN(len(cannedReplies))]
}

func seededRNG() *mrand.Rand {
	var seed [16]byte
	if _, err := crand.Read(seed[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	s1 := binary.LittleEndian.Uint64(seed[0:8])
	s2 := binary.LittleEndian.Uint64(seed[8:16])
	return mrand.New(mrand.NewPCG(s1, s2))
}

// (核心 reflex tool handler 见 tools.go，注册到 tools.LaneReflex 桶)
