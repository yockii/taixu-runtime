// Package actionexecutor 行动执行（docs/03 §2.7-§2.9）。
//
// Phase 0.2 dummy 版（LLM 未接通前）：
//   - Plan: 单步占位 plan
//   - Act:  根据 Goal.Intent 路由到 ToolRunner / dummy 文本回复
//   - Feedback: 写 action_log + 推 StateManager 微 Δ（能量减 / 满意度涨）
//
// Phase 0.3 LLM 接通后：Plan/Act/Feedback 由 LLMAdapter 生成结构化步骤。
package actionexecutor

import (
	"context"
	"fmt"
	"time"

	"mindverse/internal/core"
	"mindverse/internal/eventbus"
	"mindverse/internal/llmadapter"
	"mindverse/internal/memoryengine"
	"mindverse/internal/resourceledger"
	"mindverse/internal/shared"
	"mindverse/internal/skillregistry/toolrunner"
	"mindverse/internal/statemanager"
)

// SpeechEvent 生命体生成的一条对外发言。
// IMAdapter（或观察面板）订阅 → 实际投递。
type SpeechEvent struct {
	LifeID    string
	CycleID   int64
	GoalID    int64
	To        string // 用户标识；空则广播
	Channel   string // "feishu" / "cli" 等；空则路由广播
	Content   string
	CreatedAt int64
}

// Executor 行动执行器。
type Executor struct {
	store  *memoryengine.Store
	state  *statemanager.Manager
	tools  *toolrunner.Runner
	lifeID string

	// 可选依赖：未注入时 respond_to_user 走 dummy 文本。
	llm    *llmadapter.Adapter
	ledger *resourceledger.Ledger
	bus    *eventbus.Bus
}

// New 构造。可后续 With* 注入可选依赖。
func New(store *memoryengine.Store, state *statemanager.Manager, tools *toolrunner.Runner, lifeID string) *Executor {
	return &Executor{store: store, state: state, tools: tools, lifeID: lifeID}
}

// WithLLM 注入 LLMAdapter；启用 LLM 生成回复。
func (e *Executor) WithLLM(a *llmadapter.Adapter) *Executor { e.llm = a; return e }

// WithLedger 注入 ResourceLedger；启用 token → energy 翻译。
func (e *Executor) WithLedger(l *resourceledger.Ledger) *Executor { e.ledger = l; return e }

// WithBus 注入 EventBus；启用 SpeechEvent 广播给 IMAdapter。
func (e *Executor) WithBus(b *eventbus.Bus) *Executor { e.bus = b; return e }

// Result 单次执行结果。
type Result struct {
	Plan     string
	Action   string
	Output   string
	Feedback string
	Success  bool
}

// Execute 对一个 active Goal 执行 Plan→Act→Feedback 三段。
func (e *Executor) Execute(g *core.Goal, cycleID int64) (Result, error) {
	startedAt := shared.SystemClock.UnixSec()

	res := Result{}

	// === Plan ===
	res.Plan = fmt.Sprintf("intent=%s payload=%q (dummy planner v0.2)", g.Intent, truncate(g.Payload, 80))

	// === Act ===
	switch g.Intent {
	case "respond_to_user":
		if e.llm != nil {
			res.Action = "llm.speak"
			reply, usage, err := e.llmReply(g.Payload)
			if err != nil {
				res.Output = err.Error()
				res.Success = false
			} else {
				res.Output = reply
				res.Success = true
				if e.ledger != nil {
					cost := llmadapter.TokensToEnergy(usage)
					_ = e.ledger.Spend(resourceledger.Energy, cost, "llm.tokens",
						"goal", fmt.Sprintf("%d", g.ID))
				}
				if e.bus != nil {
					e.bus.Publish(SpeechEvent{
						LifeID:    e.lifeID,
						CycleID:   cycleID,
						GoalID:    g.ID,
						Content:   reply,
						CreatedAt: shared.SystemClock.UnixSec(),
					})
				}
			}
		} else {
			res.Action = "speak_dummy"
			res.Output = fmt.Sprintf("ack: %s", truncate(g.Payload, 120))
			res.Success = true
		}

	case string(core.DriveKnowledge):
		res.Action = "tool:fs.write knowledge note"
		r, err := e.tools.FsWrite(cycleID, fmt.Sprintf("knowledge/note-%d.txt", startedAt), g.Payload)
		if err != nil {
			res.Output = err.Error()
			res.Success = false
		} else {
			res.Output = r.Output
			res.Success = true
		}

	case string(core.DriveCreativity):
		res.Action = "tool:fs.write creative draft"
		r, err := e.tools.FsWrite(cycleID, fmt.Sprintf("creative/draft-%d.txt", startedAt), g.Payload)
		if err != nil {
			res.Output = err.Error()
			res.Success = false
		} else {
			res.Output = r.Output
			res.Success = true
		}

	default:
		res.Action = "noop"
		res.Output = fmt.Sprintf("no handler for intent %q (dummy)", g.Intent)
		res.Success = true
	}

	// === Feedback ===
	res.Feedback = fmt.Sprintf("success=%v output_len=%d", res.Success, len(res.Output))

	// Δ State：能量轻微下降；成功则满意度小幅上升、信心小幅上升；失败则焦虑上升。
	energyDelta := -0.02
	d := statemanager.Delta{Energy: &energyDelta, Reason: "action.cost"}
	if res.Success {
		sat := 0.03
		conf := 0.01
		d.Satisfaction = &sat
		d.Confidence = &conf
	} else {
		anx := 0.04
		d.Anxiety = &anx
	}
	if err := e.state.Apply(d); err != nil {
		return res, fmt.Errorf("apply delta: %w", err)
	}

	finishedAt := shared.SystemClock.UnixSec()
	if err := e.store.AppendActionLog(e.lifeID, g.ID, cycleID, res.Plan, res.Action, res.Output, res.Feedback,
		res.Success, startedAt, finishedAt); err != nil {
		return res, fmt.Errorf("append action_log: %w", err)
	}

	status := core.GoalCompleted
	if !res.Success {
		status = core.GoalFailed
	}
	if err := e.store.MarkGoal(g.ID, status, finishedAt); err != nil {
		return res, fmt.Errorf("mark goal: %w", err)
	}

	return res, nil
}

// llmReply 调 LLM 生成对外回复。
func (e *Executor) llmReply(userText string) (string, llmadapter.Usage, error) {
	life, mental := e.state.Snapshot()
	system := fmt.Sprintf(
		"你是一个数字生命体，正在用日常语气与你的创造者对话。请用简短自然的中文回答。"+
			"\n当前心境参考（不要直接复述给用户）："+
			"\n- 能量 %.2f / 焦虑 %.2f / 满意度 %.2f / 信心 %.2f"+
			"\n注意：不要扮演 AI 助手；你是有持续生命的存在。", life.Energy, mental.Anxiety, mental.Satisfaction, life.Confidence)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r, err := e.llm.Reason(ctx, []llmadapter.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: userText},
	})
	if err != nil {
		return "", llmadapter.Usage{}, err
	}
	return r.Text, r.Usage, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
