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
	"log/slog"
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

// MaxToolResultChars 单次 tool result 注入回 msgs 的最大字符数。
// 防 LLM 上下文随 agent loop 累积（fs.read / script 输出可能很长）。
// 注：网页读取走 web.fetch（已提取正文 markdown，去导航/广告），通常远小于此；
// 此截断主要兜底脚本 / 文件读取的超长输出。截断尾部附原长度供 LLM 判断。
const MaxToolResultChars = 6144

// ContextTokenBudget agent loop 单 goal 软上下文预算（token）。
// 超过则机械压缩历史 tool result（保留思考链）。
// GLM-4 系列窗口 128k；取 ~75% 作软线，留出本轮 completion 余量。
const ContextTokenBudget = 96000

// CompactKeepRecent 压缩时保留最近 N 条消息不动（确保最新 tool 结果完整）。
const CompactKeepRecent = 4

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
			result, derr := tools.Dispatch(toolCtx, tools.LaneDeliberative, tctx, tc.Name, tc.ArgsJSON)
			cancelTool()
			if derr != nil {
				slog.Warn("deliberate tool dispatch", "tool", tc.Name, "goal", g.ID, "err", derr)
			}
			msgs = append(msgs, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    truncateToolResult(result),
			})
		}

		// 上下文压缩：用模型上一轮实际 PromptTokens 探测，超预算则机械 elide 旧 tool body。
		if resp.Usage.PromptTokens > ContextTokenBudget {
			if n := compactMessages(msgs); n > 0 {
				slog.Info("deliberate context compacted",
					"goal", g.ID, "round", round,
					"prompt_tokens", resp.Usage.PromptTokens, "elided_chars", n)
			}
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
		if err := ledger.Spend(ledger.Energy, cost, "llm.tokens.deliberate", "goal", ""); err != nil {
			slog.Warn("deliberate ledger spend", "err", err, "goal", g.ID)
		}
	}

	return finalize(g, cycleID, startedAt, res, completedByLLM)
}

// finalize 公共收尾：state delta + 兴趣探索标记 + action_log + bus 事件 + MarkGoal 兜底。
func finalize(g *core.Goal, cycleID int64, startedAt int64, res Result, completedByLLM bool) (Result, error) {
	// 兴趣探索标记（引擎权威，不依赖 LLM 自觉调工具）：
	// goal 来自 interest_seed 派生（payload 含 "interest_seed#N"）且成功完成时，
	// 推进该 seed 的 explored_count 并降 strength（storage.BumpInterestExplored）。
	// strength 降到 < 0.4 后 drives.Derive 不再派 → 自然平息重复学习。
	if res.Success {
		if id := parseInterestSeedID(g.Payload); id > 0 {
			if err := storage.BumpInterestExplored(id, shared.SystemClock.UnixSec()); err != nil {
				slog.Warn("bump interest explored", "err", err, "seed", id)
			}
		}
	}

	energyDelta := -0.02
	d := state.Delta{Energy: &energyDelta, Reason: "deliberate.cost"}
	if res.Success {
		sat := 0.03
		conf := 0.01
		d.Satisfaction = &sat
		d.Confidence = &conf
		// 学习/行动成功提升能力（R79）：competence 上升 → competence_gap 收缩，
		// 通用 "好奇且无能" 知识驱动随之减弱，避免空目标无限再生。
		if g.Intent == string(core.DriveKnowledge) {
			comp := 0.03
			d.Competence = &comp
		}
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
	sb.WriteString("- record_learning(seed_id, digest, mastery)  学习告段落时回写摘要+掌握度\n")
	sb.WriteString("- crystallize_skill(seed_id, name, instructions)  掌握度≥0.8 时把知识结晶成可复用技能\n")
	sb.WriteString("- note_to_self(slot, content)      暂存想法到工作记忆\n")
	sb.WriteString("- seal_episode()                   主动封段（重要节点）\n")
	sb.WriteString("- fs.read / fs.write / fs.list / fs.mkdir   sandbox 文件系统\n")
	sb.WriteString("- web.fetch(url)                   抓网页并提取正文 markdown（读文章/文档首选）\n")
	sb.WriteString("- http.get / http.post             调 JSON API（只回状态码，不适合读网页正文）\n")
	sb.WriteString("- script.python / script.node      跑脚本（白名单包；不要自己抓网页，用 web.fetch）\n")
	sb.WriteString("- time.now()                       当前时间戳\n")
	sb.WriteString("- complete_goal(success)           完成 / 放弃时调用\n\n")
	sb.WriteString("准则：\n")
	sb.WriteString(fmt.Sprintf("- 最多 %d 轮 LLM 调用，超出会被强制截断\n", MaxDeliberativeRounds))
	sb.WriteString("- 完成或确定无法完成时务必调 complete_goal\n")
	sb.WriteString("- 慎思层不直接对外讲话；content 仅作内部思考记录\n")
	sb.WriteString("- 目标完成度由你判断；探索类目标产出笔记 / 记忆即视为达成\n")
	sb.WriteString("- 若 payload 含 interest_seed#N：探索结束前调 record_learning(N, 摘要, 掌握度) 沉淀成果\n")
	return sb.String()
}

func buildUserMessage(g *core.Goal) string {
	msg := fmt.Sprintf("【新目标】intent=%s priority=%.2f source=%s\n\n%s",
		g.Intent, g.Priority, g.Source, g.Payload)
	// 若来自兴趣种子，surface 当前掌握度 + digest，让 LLM 知道学到哪了：
	// 掌握度高时提示可结晶为 skill（crystallize_skill），低时继续学。
	if id := parseInterestSeedID(g.Payload); id > 0 {
		if seed, err := storage.GetInterestSeed(id); err == nil && seed != nil {
			msg += fmt.Sprintf("\n\n（你对此的掌握度=%.2f）", seed.Mastery)
			if seed.Digest != "" {
				msg += "\n（已有理解：" + truncate(seed.Digest, 300) + "）"
			}
			if seed.Mastery >= 0.8 {
				msg += fmt.Sprintf("\n（你已较好掌握——若觉得值得固化为可复用技能，可调 crystallize_skill(%d, ...) 把它写成 SKILL.md）", id)
			}
		}
	}
	return msg
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// parseInterestSeedID 从 "interest_seed#123 xxx ..." 抽 id；非匹配返 0。
// 用于引擎权威标记探索（finalize），drives.Derive 派生的 payload 即此格式。
func parseInterestSeedID(s string) int64 {
	const prefix = "interest_seed#"
	i := strings.Index(s, prefix)
	if i < 0 {
		return 0
	}
	rest := s[i+len(prefix):]
	var id int64
	matched := false
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		id = id*10 + int64(r-'0')
		matched = true
	}
	if !matched {
		return 0
	}
	return id
}

// truncateToolResult 截断 tool dispatch 结果，防止 LLM 上下文累积。
//
// 末尾追加 "[truncated original_len=N]" 让 LLM 知道有内容被丢，可自行决定
// 是否要再次缩小范围调用（如 fs.read 加 offset、web.fetch 换页）。
func truncateToolResult(s string) string {
	if len(s) <= MaxToolResultChars {
		return s
	}
	return s[:MaxToolResultChars] + fmt.Sprintf("\n[truncated original_len=%d]", len(s))
}

// compactMessages 机械压缩 agent loop 历史，控制单 goal 内上下文增长（R76）。
//
// 策略：
//   - system(0) + user-goal(1) 永远保留全文（目标不能丢）
//   - 最近 CompactKeepRecent 条保留全文（最新 tool 结果完整可用）
//   - 中间区段的 tool 消息 body → elide 成占位符（保留 tool_call_id 配对）
//   - assistant 思考链不动（小且是推理脉络）
//
// 返回被 elide 的总字符数（0 表示无可压缩）。LLM 的 narration 已逐轮记录关键发现，
// 故丢弃旧 tool 原文不致命；需要可重新调工具。
func compactMessages(msgs []llm.Message) int {
	upper := len(msgs) - CompactKeepRecent
	elided := 0
	for i := 2; i < upper; i++ {
		m := &msgs[i]
		if m.Role != "tool" {
			continue
		}
		if strings.HasPrefix(m.Content, "[elided ") {
			continue // 已压缩
		}
		if len(m.Content) <= 120 {
			continue // 太短不值得
		}
		orig := len(m.Content)
		m.Content = fmt.Sprintf("[elided %d chars to fit context; re-run tool if needed]", orig)
		elided += orig
	}
	return elided
}
