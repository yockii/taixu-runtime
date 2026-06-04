// Package action Plan/Act/Feedback 行动执行（docs/03 §2.7-§2.9）单例。
//
// Phase 0.3 启 LLM 接通：respond_to_user → llm.Reason。
// 工具调用（fs.write 等）走 toolrunner。
// 消耗 → ledger（energy via tokens、knowledge via success）。
// 发言 → bus.Publish(SpeechEvent) → io/lark 订阅推送。
package action

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/io/llm"
	"mindverse/internal/runtime/ledger"
	"mindverse/internal/runtime/state"
	"mindverse/internal/shared"
	"mindverse/internal/skill/toolrunner"
	"mindverse/internal/storage"
)

// SpeechEvent 生命体生成的一条对外发言。
type SpeechEvent struct {
	LifeID    string
	CycleID   int64
	GoalID    int64
	To        string
	Channel   string
	Content   string
	CreatedAt int64
}

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
	case "respond_to_user":
		if llm.Configured() {
			res.Action = "llm.speak"
			reply, usage, err := llmReply(g.Payload)
			if err != nil {
				res.Output = err.Error()
				res.Success = false
			} else {
				res.Output = reply
				res.Success = true
				cost := llm.TokensToEnergy(usage)
				_ = ledger.Spend(ledger.Energy, cost, "llm.tokens", "goal", fmt.Sprintf("%d", g.ID))
				bus.Publish(SpeechEvent{
					LifeID:    lifeID,
					CycleID:   cycleID,
					GoalID:    g.ID,
					Content:   reply,
					CreatedAt: shared.SystemClock.UnixSec(),
				})
			}
		} else {
			res.Action = "speak_dummy"
			res.Output = fmt.Sprintf("ack: %s", truncate(g.Payload, 120))
			res.Success = true
		}

	case string(core.DriveKnowledge):
		res.Action = "tool:fs.write knowledge note"
		r, err := toolrunner.FsWrite(cycleID, fmt.Sprintf("knowledge/note-%d.txt", startedAt), g.Payload)
		if err != nil {
			res.Output = err.Error()
			res.Success = false
		} else {
			res.Output = r.Output
			res.Success = true
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

	status := core.GoalCompleted
	if !res.Success {
		status = core.GoalFailed
	}
	if err := storage.MarkGoal(g.ID, status, finishedAt); err != nil {
		return res, fmt.Errorf("mark goal: %w", err)
	}

	return res, nil
}

func llmReply(userText string) (string, llm.Usage, error) {
	life, mental := state.Snapshot()
	system := fmt.Sprintf(
		"你是一个数字生命体，正在用日常语气与你的创造者对话。请用简短自然的中文回答。"+
			"\n当前心境参考（不要直接复述给用户）："+
			"\n- 能量 %.2f / 焦虑 %.2f / 满意度 %.2f / 信心 %.2f"+
			"\n注意：不要扮演 AI 助手；你是有持续生命的存在。", life.Energy, mental.Anxiety, mental.Satisfaction, life.Confidence)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r, err := llm.Reason(ctx, []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: userText},
	})
	if err != nil {
		return "", llm.Usage{}, err
	}
	return r.Text, r.Usage, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
