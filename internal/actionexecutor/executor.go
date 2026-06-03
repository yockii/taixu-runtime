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
	"fmt"

	"mindverse/internal/core"
	"mindverse/internal/memoryengine"
	"mindverse/internal/shared"
	"mindverse/internal/skillregistry/toolrunner"
	"mindverse/internal/statemanager"
)

// Executor 行动执行器。
type Executor struct {
	store  *memoryengine.Store
	state  *statemanager.Manager
	tools  *toolrunner.Runner
	lifeID string
}

// New 构造。
func New(store *memoryengine.Store, state *statemanager.Manager, tools *toolrunner.Runner, lifeID string) *Executor {
	return &Executor{store: store, state: state, tools: tools, lifeID: lifeID}
}

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
		res.Action = "speak_dummy"
		res.Output = fmt.Sprintf("ack: %s", truncate(g.Payload, 120))
		res.Success = true

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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
