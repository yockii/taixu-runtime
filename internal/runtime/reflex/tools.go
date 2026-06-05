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
	return nil
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
