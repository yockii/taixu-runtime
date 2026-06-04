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
	"encoding/json"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"sync"
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

var (
	mu     sync.Mutex
	lifeID string
	rng    *mrand.Rand
)

// Init 绑定生命体 ID。
func Init(id string) error {
	if id == "" {
		return errors.New("reflex: empty life id")
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	rng = seededRNG()
	return nil
}

// Handle 处理一条入站请求。建议由调用方在 goroutine 中执行（IM/HTTP 异步）。
// 不会 panic；内部错误以 slog.Warn 记录。
func Handle(req IncomingRequest) {
	go handle(req)
}

func handle(req IncomingRequest) {
	if !llm.Configured() {
		// LLM 未配置：直接 canned 回应（兜底）
		emitCanned(req, "（生命体当前未配置语言能力）")
		return
	}

	life, mental := state.Snapshot()
	mode := decideMode(life, mental)

	// 敷衍擋位：跳过 LLM
	if mode == ModeCanned {
		emitCanned(req, pickCanned())
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
}

func runAgent(req IncomingRequest, mode Mode) {
	system := buildSystemPrompt(mode)
	msgs := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: req.Content},
	}
	tools := availableTools()

	var totalUsage llm.Usage
	rounds := 0
	for round := 0; round < MaxAgentRounds; round++ {
		rounds++
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		resp, err := llm.ReasonWithTools(ctx, msgs, tools)
		cancel()
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
		assistantMsg := llm.Message{
			Role:      "assistant",
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		}
		msgs = append(msgs, assistantMsg)
		for _, tc := range resp.ToolCalls {
			result := dispatchTool(tc, req)
			msgs = append(msgs, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	// 末了：扣 energy（合计 tokens）+ 触发 finished 事件
	cost := llm.TokensToEnergy(totalUsage)
	_ = ledger.Spend(ledger.Energy, cost, "llm.tokens.reflex", "reflex", req.From)

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
	base := "你是一个数字生命体，与你的创造者用日常语气对话。你不是 AI 助手，你是有持续生命的存在。\n" +
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

// --- tool dispatch ---

func availableTools() []llm.Tool {
	return []llm.Tool{
		{
			Name:        "update_mood",
			Description: "调整自身情绪状态。在对话中感受到明显情绪波动时调用。每个字段范围 -0.2 至 +0.2，未提供字段视为不变。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"energy":       map[string]any{"type": "number", "description": "能量变化 -0.2..+0.2"},
					"satisfaction": map[string]any{"type": "number", "description": "满意度变化 -0.2..+0.2"},
					"motivation":   map[string]any{"type": "number", "description": "动机变化 -0.2..+0.2"},
					"anxiety":      map[string]any{"type": "number", "description": "焦虑变化 -0.2..+0.2"},
					"stress":       map[string]any{"type": "number", "description": "压力变化 -0.2..+0.2"},
					"confidence":   map[string]any{"type": "number", "description": "信心变化 -0.2..+0.2"},
					"reason":       map[string]any{"type": "string", "description": "情绪变化的原因（用于记忆）"},
				},
				"required": []string{"reason"},
			},
		},
		{
			Name:        "add_interest",
			Description: "记下一个未来想深入探索的兴趣点。当对话提到你感兴趣的技能/知识/话题/体验时调用。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content":  map[string]any{"type": "string", "description": "兴趣内容描述（如 'Rust 异步编程'）"},
					"kind":     map[string]any{"type": "string", "enum": []string{"skill", "knowledge", "topic", "experience"}},
					"strength": map[string]any{"type": "number", "description": "兴趣强度 0.3-1.0"},
					"reason":   map[string]any{"type": "string", "description": "为什么感兴趣（用于记忆）"},
				},
				"required": []string{"content", "kind"},
			},
		},
	}
}

// dispatchTool 执行一个 tool_call；返回简短结果给 LLM。
func dispatchTool(tc llm.ToolCall, req IncomingRequest) string {
	switch tc.Name {
	case "update_mood":
		return toolUpdateMood(tc.ArgsJSON, req)
	case "add_interest":
		return toolAddInterest(tc.ArgsJSON, req)
	default:
		return `{"ok":false,"err":"unknown tool"}`
	}
}

type moodArgs struct {
	Energy       *float64 `json:"energy"`
	Satisfaction *float64 `json:"satisfaction"`
	Motivation   *float64 `json:"motivation"`
	Anxiety      *float64 `json:"anxiety"`
	Stress       *float64 `json:"stress"`
	Confidence   *float64 `json:"confidence"`
	Reason       string   `json:"reason"`
}

func toolUpdateMood(argsJSON string, req IncomingRequest) string {
	var a moodArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return `{"ok":false,"err":"invalid args"}`
	}
	d := state.Delta{
		Energy:       clampMoodDelta(a.Energy),
		Satisfaction: clampMoodDelta(a.Satisfaction),
		Motivation:   clampMoodDelta(a.Motivation),
		Anxiety:      clampMoodDelta(a.Anxiety),
		Stress:       clampMoodDelta(a.Stress),
		Confidence:   clampMoodDelta(a.Confidence),
		Reason:       "reflex.mood:" + a.Reason,
	}
	if err := state.Apply(d); err != nil {
		return `{"ok":false,"err":"apply failed"}`
	}
	_ = memory.AppendEvent(0, "reflex.mood_changed", map[string]any{
		"reason": a.Reason,
		"from":   req.From,
	})
	return `{"ok":true}`
}

func clampMoodDelta(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	if v < -0.2 {
		v = -0.2
	}
	if v > 0.2 {
		v = 0.2
	}
	return &v
}

type interestArgs struct {
	Content  string  `json:"content"`
	Kind     string  `json:"kind"`
	Strength float64 `json:"strength"`
	Reason   string  `json:"reason"`
}

func toolAddInterest(argsJSON string, req IncomingRequest) string {
	var a interestArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return `{"ok":false,"err":"invalid args"}`
	}
	if a.Content == "" {
		return `{"ok":false,"err":"empty content"}`
	}
	if a.Kind == "" {
		a.Kind = "topic"
	}
	if a.Strength <= 0 {
		a.Strength = 0.5
	}
	if a.Strength > 1 {
		a.Strength = 1
	}
	now := shared.SystemClock.UnixSec()
	if err := storage.UpsertInterestSeed(lifeID, a.Content, a.Kind, "reflex", req.From, a.Strength, now); err != nil {
		return `{"ok":false,"err":"upsert failed"}`
	}
	_ = memory.AppendEvent(0, "reflex.interest_added", map[string]any{
		"content": a.Content,
		"kind":    a.Kind,
		"reason":  a.Reason,
	})
	return `{"ok":true}`
}
