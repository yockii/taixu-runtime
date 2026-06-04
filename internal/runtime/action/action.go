// Package action Plan/Act/Feedback 行动执行（docs/03 §2.7-§2.9）单例。
//
// Phase 0.5：respond_to_user 已移至 reflex 通道；此处仅处理慎思 IntrinsicDrive：
// DriveKnowledge / DriveCreativity / DriveCuriosity（兴趣探索）等。
// 工具调用（fs.write/http.get 等）走 toolrunner。
// 消耗 → ledger（energy 慎思 cost；knowledge 成功 earn）。
package action

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/runtime/state"
	"mindverse/internal/shared"
	"mindverse/internal/skill/toolrunner"
	"mindverse/internal/storage"
)

// Result 单次执行结果。
type Result struct {
	Plan     string
	Action   string
	Output   string
	Feedback string
	Success  bool
}

var (
	mu     sync.Mutex
	lifeID string
)

func Init(id string) error {
	if id == "" {
		return errors.New("action: empty life id")
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	return nil
}

// Execute 对一个 active Goal 执行 Plan→Act→Feedback。
func Execute(g *core.Goal, cycleID int64) (Result, error) {
	startedAt := shared.SystemClock.UnixSec()
	res := Result{}

	res.Plan = fmt.Sprintf("intent=%s payload=%q (planner v0.3)", g.Intent, truncate(g.Payload, 80))

	switch g.Intent {
	case string(core.DriveKnowledge):
		// Phase 0.5：DriveKnowledge 多源 — 自身好奇心 / 兴趣种子（reflex 注入）。
		// 探索动作：写一条笔记 + 标记兴趣已探索（若来源是 interest_seed）。
		res.Action = "tool:fs.write knowledge note"
		r, err := toolrunner.FsWrite(cycleID, fmt.Sprintf("knowledge/note-%d.txt", startedAt), g.Payload)
		if err != nil {
			res.Output = err.Error()
			res.Success = false
		} else {
			res.Output = r.Output
			res.Success = true
			// 若 payload 含 interest_seed#N 前缀，则推进探索计数
			if id := parseInterestSeedID(g.Payload); id > 0 {
				_ = storage.BumpInterestExplored(id, shared.SystemClock.UnixSec())
			}
		}

	case string(core.DriveCreativity):
		res.Action = "tool:fs.write creative draft"
		r, err := toolrunner.FsWrite(cycleID, fmt.Sprintf("creative/draft-%d.txt", startedAt), g.Payload)
		if err != nil {
			res.Output = err.Error()
			res.Success = false
		} else {
			res.Output = r.Output
			res.Success = true
		}

	default:
		res.Action = "noop"
		res.Output = fmt.Sprintf("no handler for intent %q", g.Intent)
		res.Success = true
	}

	res.Feedback = fmt.Sprintf("success=%v output_len=%d", res.Success, len(res.Output))

	energyDelta := -0.02
	d := state.Delta{Energy: &energyDelta, Reason: "action.cost"}
	if res.Success {
		sat := 0.03
		conf := 0.01
		d.Satisfaction = &sat
		d.Confidence = &conf
	} else {
		anx := 0.04
		d.Anxiety = &anx
	}
	if err := state.Apply(d); err != nil {
		return res, fmt.Errorf("apply delta: %w", err)
	}

	finishedAt := shared.SystemClock.UnixSec()
	if err := storage.AppendActionLog(lifeID, g.ID, cycleID, res.Plan, res.Action, res.Output, res.Feedback,
		res.Success, startedAt, finishedAt); err != nil {
		return res, fmt.Errorf("append action_log: %w", err)
	}
	bus.Publish(bus.ActionDone{
		LifeID:    lifeID,
		CycleID:   cycleID,
		GoalID:    g.ID,
		Action:    res.Action,
		Success:   res.Success,
		StartedAt: startedAt,
	})

	status := core.GoalCompleted
	if !res.Success {
		status = core.GoalFailed
	}
	if err := storage.MarkGoal(g.ID, status, finishedAt); err != nil {
		return res, fmt.Errorf("mark goal: %w", err)
	}

	return res, nil
}

// parseInterestSeedID 从 "interest_seed#123 xxx ..." 抽 id；非匹配返 0。
func parseInterestSeedID(s string) int64 {
	const prefix = "interest_seed#"
	if !strings.HasPrefix(s, prefix) {
		return 0
	}
	rest := s[len(prefix):]
	var id int64
	for i, r := range rest {
		if r < '0' || r > '9' {
			if i == 0 {
				return 0
			}
			break
		}
		id = id*10 + int64(r-'0')
	}
	return id
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
