// Package action 慎思层 agent loop（docs/03 §2.7-§2.9 + SKILLS-AND-TOOLS §6）单例。
//
// Phase 0.5+：respond_to_user 已移至 reflex 通道；此处 deliberative agent loop 走
// llm.ReasonWithTools + tools.Dispatch(LaneDeliberative)。LLM 决定调用什么工具，
// 引擎仅约束最大轮次、超时、能量消耗。
//
// 与 reflex 的差异：
//
//	reflex      System 1，对话即时回应，content emit → 飞书消息
//	deliberative System 2，cycle 内单 goal，content emit → action_log（不发飞书）
//	            tool 多/重，含 fs/http/script；MaxRounds=6；单轮 LLM timeout 120s
package action

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/io/llm"
	"mindverse/internal/runtime/ledger"
	"mindverse/internal/runtime/state"
	"mindverse/internal/runtime/tools"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
)

// MaxDeliberativeRounds 慎思 agent loop 单 goal 最大轮次。
const MaxDeliberativeRounds = 6

// LLMRoundTimeout 单轮 LLM 调用超时。
const LLMRoundTimeout = 120 * time.Second

// ToolDispatchTimeout 单次 tool dispatch 超时（http / script 可能较慢）。
const ToolDispatchTimeout = 30 * time.Second

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

// Execute 对一个 active Goal 跑慎思 agent loop。
func Execute(g *core.Goal, cycleID int64) (Result, error) {
	startedAt := shared.SystemClock.UnixSec()
	res := Result{
		Plan: fmt.Sprintf("intent=%s payload=%q", g.Intent, truncate(g.Payload, 80)),
	}

	if !llm.Configured() {
		// LLM 未配：标记失败 + 退出。
		return finalize(g, cycleID, startedAt, Result{
			Plan:    res.Plan,
			Action:  "llm.unavailable",
			Output:  "llm not configured",
			Success: false,
		}, false)
	}

	system := buildDeliberativeSystemPrompt(g)
	userMsg := buildUserMessage(g)
	msgs := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: userMsg},
	}
	delibTools := tools.ListLLMTools(tools.LaneDeliberative)
	tctx := tools.Context{LifeID: lifeID, CycleID: cycleID, GoalID: g.ID}

	var totalUsage llm.Usage
	var trace []string // 每轮 content
	var toolCalls []string
	completedByLLM := false
	rounds := 0

	for round := 0; round < MaxDeliberativeRounds; round++ {
		rounds++
		llmCtx, cancelLLM := context.WithTimeout(context.Background(), LLMRoundTimeout)
		resp, err := llm.ReasonWithTools(llmCtx, msgs, delibTools)
		cancelLLM()
		if err != nil {
			trace = append(trace, fmt.Sprintf("[r%d] llm err: %v", round, err))
			break
		}
		totalUsage.PromptTokens += resp.Usage.PromptTokens
		totalUsage.CompletionTokens += resp.Usage.CompletionTokens
		totalUsage.TotalTokens += resp.Usage.TotalTokens

		if resp.Text != "" {
			trace = append(trace, fmt.Sprintf("[r%d] %s", round, resp.Text))
		}

		if len(resp.ToolCalls) == 0 {
			break
		}

		msgs = append(msgs, llm.Message{
			Role:      "assistant",
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})
		for _, tc := range resp.ToolCalls {
			toolCalls = append(toolCalls, tc.Name)
			if tc.Name == "complete_goal" {
				completedByLLM = true
			}
			toolCtx, cancelTool := context.WithTimeout(context.Background(), ToolDispatchTimeout)
			result, _ := tools.Dispatch(toolCtx, tools.LaneDeliberative, tctx, tc.Name, tc.ArgsJSON)
			cancelTool()
			msgs = append(msgs, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	res.Action = fmt.Sprintf("llm.agent rounds=%d tools=[%s]", rounds, strings.Join(toolCalls, ","))
	res.Output = strings.Join(trace, "\n\n")
	if res.Output == "" {
		res.Output = "(no llm content emitted)"
	}
	res.Success = len(trace) > 0 || len(toolCalls) > 0
	res.Feedback = fmt.Sprintf("rounds=%d tools=%d completed_by_llm=%v tokens=%d",
		rounds, len(toolCalls), completedByLLM, totalUsage.TotalTokens)

	// 累计 LLM 消耗 → energy
	if totalUsage.TotalTokens > 0 {
		cost := llm.TokensToEnergy(totalUsage)
		_ = ledger.Spend(ledger.Energy, cost, "llm.tokens.deliberate", "goal", "")
	}

	return finalize(g, cycleID, startedAt, res, completedByLLM)
}

// finalize 公共收尾：state delta + action_log + bus 事件 + MarkGoal 兜底。
func finalize(g *core.Goal, cycleID int64, startedAt int64, res Result, completedByLLM bool) (Result, error) {
	energyDelta := -0.02
	d := state.Delta{Energy: &energyDelta, Reason: "deliberate.cost"}
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
	if err := storage.AppendActionLogKind(lifeID, g.ID, cycleID, storage.ActionKindDeliberate,
		res.Plan, res.Action, res.Output, res.Feedback,
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

	// MarkGoal 兜底：LLM 调过 complete_goal 已 mark；否则引擎据 success 推断 mark。
	if !completedByLLM {
		status := core.GoalCompleted
		if !res.Success {
			status = core.GoalFailed
		}
		if err := storage.MarkGoal(g.ID, status, finishedAt); err != nil {
			return res, fmt.Errorf("mark goal: %w", err)
		}
	}
	return res, nil
}

func buildDeliberativeSystemPrompt(g *core.Goal) string {
	var sb strings.Builder
	sb.WriteString("你是数字生命体的慎思层（System 2），目前刚被分配一个内驱目标。\n\n")
	sb.WriteString("可调用工具（精选最常用）：\n")
	sb.WriteString("- query_memory(layer, q?, limit?)  跨记忆层检索（layer: episodic/semantic/reflection）\n")
	sb.WriteString("- recall_recent(limit?, q?)        最近 episode 摘要\n")
	sb.WriteString("- enqueue_subgoal(intent, payload, priority?)  拆子任务入队\n")
	sb.WriteString("- explore_interest_seed(seed_id)   推进兴趣探索计数\n")
	sb.WriteString("- note_to_self(slot, content)      暂存想法到工作记忆\n")
	sb.WriteString("- seal_episode()                   主动封段（重要节点）\n")
	sb.WriteString("- fs.read / fs.write / fs.list / fs.mkdir   sandbox 文件系统\n")
	sb.WriteString("- http.get / http.post             网络请求\n")
	sb.WriteString("- time.now()                       当前时间戳\n")
	sb.WriteString("- complete_goal(success)           完成 / 放弃时调用\n\n")
	sb.WriteString("准则：\n")
	sb.WriteString(fmt.Sprintf("- 最多 %d 轮 LLM 调用，超出会被强制截断\n", MaxDeliberativeRounds))
	sb.WriteString("- 完成或确定无法完成时务必调 complete_goal\n")
	sb.WriteString("- 慎思层不直接对外讲话；content 仅作内部思考记录\n")
	sb.WriteString("- 若 payload 含 'interest_seed#N' 前缀，应调 explore_interest_seed(N)\n")
	return sb.String()
}

func buildUserMessage(g *core.Goal) string {
	return fmt.Sprintf("【新目标】intent=%s priority=%.2f source=%s\n\n%s",
		g.Intent, g.Priority, g.Source, g.Payload)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
