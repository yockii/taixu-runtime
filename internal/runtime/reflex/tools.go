// Package reflex 反射通道 tool handler 集（注册到 tools.LaneReflex）。
//
// 这些 tool 与"对话即时回应"语义直接耦合，故住在 reflex 包内；
// 它们不需要外部副作用 / 不消耗 wealth，仅写本地 state + memory + interest_seed。
//
// Skill 装载来的反射 tool 不走此文件，由 skill loader 在 LaneReflex 桶里注册自己的 handler。
package reflex

import (
	"context"
	"encoding/json"
	"strconv"

	"mindverse/internal/bus"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/runtime/skill"
	"mindverse/internal/runtime/state"
	"mindverse/internal/runtime/tools"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
)

// registerCoreTools 在 reflex.Init 中调，把内置反射 tool 注册到 tools.LaneReflex。
func registerCoreTools() error {
	if err := tools.Register(tools.Tool{
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
		Lanes:   []tools.Lane{tools.LaneReflex},
		Handler: handlerUpdateMood,
	}); err != nil {
		return err
	}
	if err := tools.Register(tools.Tool{
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
		Lanes:   []tools.Lane{tools.LaneReflex},
		Handler: handlerAddInterest,
	}); err != nil {
		return err
	}
	if err := tools.Register(tools.Tool{
		Name: "set_quiet",
		Description: "当对方明确表示暂时不想被打扰（如『接下来1小时别发消息』『我在忙，晚点再说』『今晚先这样』），调用此工具。" +
			"在指定时长内你不会再主动给ta发消息（ta主动找你时你仍照常回应）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"minutes": map[string]any{"type": "number", "description": "安静多少分钟（如 60=一小时；『今晚』可估到次日早晨的分钟数）"},
				"reason":  map[string]any{"type": "string", "description": "对方说了什么（用于记忆）"},
			},
			"required": []string{"minutes"},
		},
		Lanes:   []tools.Lane{tools.LaneReflex},
		Handler: handlerSetQuiet,
	}); err != nil {
		return err
	}
	if err := tools.Register(tools.Tool{
		Name: "defer_research",
		Description: "当对方抛来一个值得你之后认真深入、而非现在三言两语能答完的请求/想法/研究课题时调用" +
			"（如『帮我研究下 X』『你觉得 Y 怎么实现』这类需要查资料/动手琢磨的慢工）。" +
			"调用后：① 引擎会把这件事记成你的一个待办研究目标，你慎思时去做；② 引擎会替你回一句『收到，回头研究下』的确认。" +
			"所以你**不必**自己再复述确认，调完这个工具即可（也可以再补一两句别的）。" +
			"⚠ 仅用于真需要事后深入的请求；能当场答的闲聊别用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topic": map[string]any{"type": "string", "description": "要研究/深入的事情（用你自己的话凝练成一句，作为待办目标内容）"},
				"why":   map[string]any{"type": "string", "description": "可选：为什么值得深入（用于记忆）"},
			},
			"required": []string{"topic"},
		},
		Lanes:   []tools.Lane{tools.LaneReflex},
		Handler: handlerDeferResearch,
	}); err != nil {
		return err
	}
	return nil
}

type deferResearchArgs struct {
	Topic string `json:"topic"`
	Why   string `json:"why"`
}

// deferResearchAcks 延迟研究的确认文案变体（任务 2）：拟人、随机挑一个，别每次一样。
var deferResearchAcks = []string{
	"收到，我有空时研究一下，有结果再告诉你 🌱",
	"嗯，这个我记下了，回头认真琢磨琢磨，想清楚了来找你。",
	"好，让我消化下，得花点时间——有眉目了第一时间同步你。",
	"收到～这事儿值得好好想想，我慢慢研究，回头跟你说。",
	"记下啦，这个不是一句两句能讲清的，我抽空深入下再回你 🌿",
}

func pickDeferAck() string {
	mu.Lock()
	defer mu.Unlock()
	if rng == nil {
		return deferResearchAcks[0]
	}
	return deferResearchAcks[rng.IntN(len(deferResearchAcks))]
}

// handlerDeferResearch 把"需事后深入的请求"落成一个带请求者的 ExternalRequest 目标（任务 2），
// 并由引擎替生命体回一句拟人确认（经 bus → lark egress / SSE，与正常 reflex 回复同路）。
//
// 设计取舍：判定"是否值得研究"交给 reflex LLM（它最懂当下语境，闲聊 vs 慢工一眼可辨），
// 引擎只负责"入队 + 回执"两件确定性的事——既保证有请求者追踪、又保证用户必得到一句确认，
// 不依赖 LLM 自己记得复述。dedup 在 storage.EnqueueExternalRequest 里（同主题在飞不重复入队）。
func handlerDeferResearch(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a deferResearchArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return `{"ok":false,"err":"invalid args"}`, err
	}
	now := shared.SystemClock.UnixSec()
	// 外部托付的请求优先级略高于一般内驱探索（用户在等），但不抢占式插队。
	id, created, err := storage.EnqueueExternalRequest(tctx.LifeID, a.Topic, tctx.Channel, tctx.From, 0.7, now)
	if err != nil {
		return `{"ok":false,"err":"enqueue failed"}`, err
	}
	_ = memory.AppendEvent(0, "reflex.defer_research", map[string]any{
		"topic": a.Topic, "why": a.Why, "channel": tctx.Channel, "from": tctx.From,
		"goal_id": id, "created": created,
	})
	// 引擎替它回一句拟人确认（与 emitReply 同走 bus → 飞书/网页）。
	ack := pickDeferAck()
	bus.Publish(ReplyEvent{
		LifeID:    lifeID,
		Channel:   tctx.Channel,
		To:        tctx.From,
		Round:     0,
		Content:   ack,
		CreatedAt: now,
	})
	_ = storage.AppendActionLogKind(lifeID, 0, 0, storage.ActionKindReflex,
		"defer research ack", "defer_research.ack", ack, "", true, now, now)
	if !created {
		return `{"ok":true,"queued":false,"note":"已有相同主题的待办研究在进行中，未重复入队；已回确认"}`, nil
	}
	return `{"ok":true,"queued":true,"goal_id":` + strconv.FormatInt(id, 10) + `,"note":"已入队并已替你回一句确认"}`, nil
}

type quietArgs struct {
	Minutes float64 `json:"minutes"`
	Reason  string  `json:"reason"`
}

func handlerSetQuiet(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a quietArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return `{"ok":false,"err":"invalid args"}`, err
	}
	if a.Minutes <= 0 {
		return `{"ok":false,"err":"minutes must be > 0"}`, nil
	}
	if a.Minutes > 10080 { // 上限 7 天，防离谱时长
		a.Minutes = 10080
	}
	until := shared.SystemClock.UnixSec() + int64(a.Minutes*60)
	setSnoozeUntil(tctx.Channel, tctx.From, until)
	_ = memory.AppendEvent(0, "reflex.quiet_set", map[string]any{
		"minutes": a.Minutes, "until": until, "reason": a.Reason,
		"channel": tctx.Channel, "from": tctx.From,
	})
	return `{"ok":true,"until":` + strconv.FormatInt(until, 10) + `}`, nil
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

func handlerUpdateMood(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a moodArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return `{"ok":false,"err":"invalid args"}`, err
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
		return `{"ok":false,"err":"apply failed"}`, err
	}
	_ = memory.AppendEvent(0, "reflex.mood_changed", map[string]any{
		"reason": a.Reason,
		"from":   tctx.From,
	})
	return `{"ok":true}`, nil
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

func handlerAddInterest(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a interestArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return `{"ok":false,"err":"invalid args"}`, err
	}
	if a.Content == "" {
		return `{"ok":false,"err":"empty content"}`, nil
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
	if err := storage.UpsertInterestSeed(tctx.LifeID, a.Content, a.Kind, "reflex", tctx.From, a.Strength, now); err != nil {
		return `{"ok":false,"err":"upsert failed"}`, err
	}
	_ = memory.AppendEvent(0, "reflex.interest_added", map[string]any{
		"content": a.Content,
		"kind":    a.Kind,
		"reason":  a.Reason,
	})
	// 相关的归档技能重新拾起（R88）：兴趣再现 → 想起自己其实会这个。
	if revived := skill.ReactivateForInterest(a.Content); len(revived) > 0 {
		_ = memory.AppendEvent(0, "skill.reactivated", map[string]any{"skills": revived, "interest": a.Content})
	}
	return `{"ok":true}`, nil
}
