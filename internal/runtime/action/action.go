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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/io/llm"
	"mindverse/internal/runtime/ledger"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/runtime/skill"
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
	Plan        string
	Action      string
	Output      string
	Feedback    string
	Success     bool
	Substantive int // 本次工作型工具成功调用数（探索深度，引擎涨 mastery 用）
}

var (
	mu     sync.Mutex
	lifeID string
	genome core.Genome // persistence 调慎思韧性（R82）
)

func Init(id string, g core.Genome) error {
	if id == "" {
		return errors.New("action: empty life id")
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	genome = g
	return nil
}

// maxRounds 慎思 agent loop 轮次上限，按 persistence 调（R82）：
// 执着者钻得久（最多 ~8 轮），浅尝辄止者早收（~4 轮）。
func maxRounds() int {
	r := 4 + int(float64(MaxDeliberativeRounds-2)*genome.Persistence)
	if r < 3 {
		r = 3
	}
	if r > MaxDeliberativeRounds+2 {
		r = MaxDeliberativeRounds + 2
	}
	return r
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
	substantive := 0 // 工作型工具成功调用数 → 探索深度 → 引擎涨 mastery 的幅度（R83）
	rounds := 0

	limit := maxRounds() // 按 persistence 调（R82）
	for round := 0; round < limit; round++ {
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
			} else if isSubstantiveTool(tc.Name) {
				substantive++
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
	res.Substantive = substantive
	res.Feedback = fmt.Sprintf("rounds=%d tools=%d substantive=%d completed_by_llm=%v tokens=%d",
		rounds, len(toolCalls), substantive, completedByLLM, totalUsage.TotalTokens)

	// 累计 LLM 消耗 → energy
	if totalUsage.TotalTokens > 0 {
		cost := llm.TokensToEnergy(totalUsage)
		if err := ledger.Spend(ledger.Energy, cost, "llm.tokens.deliberate", "goal", ""); err != nil {
			slog.Warn("deliberate ledger spend", "err", err, "goal", g.ID)
		}
	}

	return finalize(g, cycleID, startedAt, res, completedByLLM)
}

// finalize 公共收尾：state delta + 兴趣探索标记 + 自动结晶 + action_log + bus 事件 + MarkGoal 兜底。
func finalize(g *core.Goal, cycleID int64, startedAt int64, res Result, completedByLLM bool) (Result, error) {
	// 兴趣探索标记（引擎权威，不依赖 LLM 自觉调工具，R83）：
	// goal 来自 interest_seed 派生（payload 含 "interest_seed#N"）且成功完成时：
	//   1) explored_count++ 并按探索深度涨 mastery（引擎给地板，不靠 LLM 自愿 record_learning）
	//   2) mastery 跨过 0.8 → 引擎自动结晶成自创技能（LLM 授权写正文）并退役该 seed
	// 这修了"探索成功但 mastery 永 0 → 结晶门槛永不达 → 零技能"的核心 bug。
	if res.Success {
		if id := parseInterestSeedID(g.Payload); id > 0 {
			now := shared.SystemClock.UnixSec()
			delta := masteryDelta(res.Substantive)
			if err := storage.BumpInterestExplored(id, delta, now); err != nil {
				slog.Warn("bump interest explored", "err", err, "seed", id)
			} else if seed, err := storage.GetInterestSeed(id); err == nil && seed != nil {
				if seed.Mastery >= skill.MasteryToCrystallize {
					maybeCrystallize(seed)
				}
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
		// B 多样性：按目标类型让对应「压力」做完回落，使下一个空槽自然轮到别的类型（自调节轮转）。
		switch g.Intent {
		case string(core.DriveKnowledge):
			comp := 0.03
			d.Competence = &comp
		case string(core.DriveCreativity):
			// 创作即表达 → 满足感更足（盖过基线），创作压力随 satisfaction 升而回落。
			s2 := 0.10
			d.Satisfaction = &s2
		case string(core.DriveSocial):
			// 酝酿分享纾解社交需求（虽未真发，但"整理出想说的"已部分缓解）。
			sn := -0.10
			d.SocialNeed = &sn
		case string(core.DriveAchievement):
			// 精进做出成果 → 能力 + 信心更实。
			comp, c2 := 0.04, 0.04
			d.Competence = &comp
			d.Confidence = &c2
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

	// ---- 递归研究目标树状态机（migration 009）：决定本目标这次是「回 pending 等子目标」还是「完成」 ----
	//
	// 关键判定：本次执行期间该目标是否新增了子目标？以执行后重读的 pending_children 为准
	//（enqueue_subgoal 工具会在 loop 内给当前目标 +1）。不变量：
	//   - pending_children > 0  →  母目标「被阻塞」：不完成、置回 pending（保持 children>0），写中间 digest。
	//                              即使 LLM 这轮误调了 complete_goal（completedByLLM），也强制覆盖回 pending——
	//                              引擎权威，子目标没做完母目标不算完。
	//   - pending_children == 0 →  无未结子目标：照常 completed/failed，写 result_digest=本次成果摘要。
	//
	// 子目标全完后母 pending_children 被 MarkGoal 逐个减到 0 → 下个 cycle NextPendingGoal 重选 →
	// 母目标带着子成果回归（buildUserMessage 注入子 digest）→ 综合 → 走到这里 children==0 → 真完成。
	pendingChildren := 0
	if cur, err := storage.GetGoalByID(g.ID); err == nil && cur != nil {
		pendingChildren = cur.PendingChildren
	}

	completed := false
	if pendingChildren > 0 {
		// 被阻塞：回 pending 等子目标。写中间进度 digest 供恢复时参考 / 面板展示。
		digest := fmt.Sprintf("已把此目标拆成 %d 个子目标，待它们完成后再综合得出结论。", pendingChildren)
		_ = storage.SetResultDigest(g.ID, digest)
		if err := storage.SetGoalPending(g.ID); err != nil {
			return res, fmt.Errorf("block goal to pending: %w", err)
		}
		slog.Info("goal blocked on subgoals, returned to pending",
			"goal", g.ID, "pending_children", pendingChildren)
	} else {
		// 无未结子目标 → 终态。LLM 调过 complete_goal 已 mark；否则引擎据 success 推断 mark。
		completed = res.Success
		// 写本目标的成果摘要（供母目标回归综合 + 知识库 dossier）。
		_ = storage.SetResultDigest(g.ID, composeResultDigest(g, res))
		if !completedByLLM {
			status := core.GoalCompleted
			if !res.Success {
				status = core.GoalFailed
			}
			if err := storage.MarkGoal(g.ID, status, finishedAt); err != nil {
				return res, fmt.Errorf("mark goal: %w", err)
			}
		}
		// 根研究目标完成 → 综合整棵子树成果，沉淀一篇知识库 dossier（任务 B）。
		if completed {
			maybeSedimentKnowledge(g, finishedAt)
		}
	}

	// 完成后主动汇报（拟人交互闭环任务 3）：若这是带请求者的 ExternalRequest 目标且已完成，
	// 把成果压成一段简短自然的话，主动回送给当初托付的人。失败只 warn 不阻断收尾。
	if completed {
		maybeReportToRequester(g, res, finishedAt)
	}
	return res, nil
}

// composeResultDigest 把本次执行的成果压成一段简短摘要（写入 goal.result_digest）。
//
// 用 LLM 蒸馏 res.Output（本轮思考链/产出）为「这个目标我得出了什么结论」；LLM 未配或产出为空
// 时退化为朴素文本。该 digest 是回归综合（母目标读子 digest）与知识库 dossier 的原料。
func composeResultDigest(g *core.Goal, res Result) string {
	material := strings.TrimSpace(res.Output)
	if material == "" || material == "(no llm content emitted)" {
		if res.Success {
			return fmt.Sprintf("已就「%s」完成研究（未留详细产出）。", truncate(g.Payload, 120))
		}
		return fmt.Sprintf("未能完成「%s」。", truncate(g.Payload, 120))
	}
	if !llm.Configured() {
		return truncate(material, 800)
	}
	sys := "你是一个数字生命体的慎思层。把你刚完成的这个研究目标的产出，凝练成 3-6 句话的成果摘要：" +
		"具体结论 / 要点，而非流水账。只输出摘要正文。"
	user := fmt.Sprintf("目标：%s\n\n你这次的产出/思考记录：\n%s\n\n凝练成果摘要：", g.Payload, truncate(material, 4000))
	ctx, cancel := context.WithTimeout(context.Background(), LLMRoundTimeout)
	defer cancel()
	resp, err := llm.Reason(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	})
	if err != nil {
		slog.Warn("compose result digest", "goal", g.ID, "err", err)
		return truncate(material, 800)
	}
	_ = ledger.Spend(ledger.Energy, llm.TokensToEnergy(resp.Usage), "llm.tokens.digest", "goal", "")
	out := strings.TrimSpace(resp.Text)
	if out == "" {
		return truncate(material, 800)
	}
	return out
}

// maybeSedimentKnowledge 根研究目标完成时，把整棵子树的成果综合成一篇结构化 dossier 入知识库（任务 B）。
//
// 触发条件（全部满足）：
//   - parent_id == 0（是根目标，整棵研究树的顶——回归到此即「最初目标」已综合完）；
//   - source 属知识类研究（ExternalRequest 用户托付 / IntrinsicDrive 且 intent=knowledge）；
//   - 该根目标尚未生成过 dossier（防重复入库）。
//
// 综合素材：根目标自身 result_digest + 各（递归）子目标 result_digest。LLM 把它们综合成
// {topic, body}；LLM 未配则朴素拼接兜底。沉淀仍兼顾 sedimentToSemantic（细碎知识点向量可检索，
// 此处不重复——根目标的语义沉淀已在 maybeCrystallize/sedimentToSemantic 的兴趣链路覆盖，
// 知识库 dossier 是另一份「成篇档案」视图）。
func maybeSedimentKnowledge(g *core.Goal, now int64) {
	if g.ParentID != 0 {
		return // 非根目标：等回归到根再综合整棵树
	}
	if !isKnowledgeResearchGoal(g) {
		return
	}
	if has, err := storage.HasKnowledgeForRootGoal(lifeID, g.ID); err != nil || has {
		return
	}

	// 收集整棵子树的成果摘要（根 + 递归子）。
	rootDigest := strings.TrimSpace(g.ResultDigest)
	if rootDigest == "" {
		if cur, err := storage.GetGoalByID(g.ID); err == nil && cur != nil {
			rootDigest = strings.TrimSpace(cur.ResultDigest)
		}
	}
	subDigests := collectSubtreeDigests(g.ID, 0)
	if rootDigest == "" && len(subDigests) == 0 {
		return // 无任何可沉淀的成果
	}

	topic, body := synthesizeDossier(g, rootDigest, subDigests)
	if strings.TrimSpace(body) == "" {
		return
	}
	id, err := storage.InsertKnowledgeEntry(lifeID, &storage.KnowledgeEntry{
		RootGoalID: g.ID,
		Topic:      topic,
		Body:       body,
		SourceKind: "research",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		slog.Warn("insert knowledge entry", "goal", g.ID, "err", err)
		return
	}
	_ = memory.AppendEvent(0, "knowledge.dossier_created", map[string]any{
		"knowledge_id": id, "root_goal": g.ID, "topic": topic,
	})
	slog.Info("knowledge dossier created", "id", id, "root_goal", g.ID, "topic", truncate(topic, 60))
}

// isKnowledgeResearchGoal 判断一个根目标是否属「知识类研究」（值得沉淀成 dossier）。
//   - ExternalRequest：用户托付的研究请求，恒算。
//   - IntrinsicDrive：仅当 intent 为 knowledge（兴趣探索类）才算；社交/稳定等不入知识库。
func isKnowledgeResearchGoal(g *core.Goal) bool {
	switch g.Source {
	case core.GoalExternal:
		return true
	case core.GoalIntrinsic:
		return g.Intent == string(core.DriveKnowledge)
	default:
		return false
	}
}

// collectSubtreeDigests 递归收集某目标下整棵子树各目标的 result_digest（按层、创建序）。
// maxDepth 安全上限对齐研究树深度，防意外环导致无限递归。
func collectSubtreeDigests(parentID int64, depth int) []string {
	if depth > 8 { // 安全护栏（研究树实际 ≤ MaxResearchDepth=3）
		return nil
	}
	children, err := storage.ListChildren(parentID)
	if err != nil {
		return nil
	}
	var out []string
	for _, c := range children {
		if d := strings.TrimSpace(c.ResultDigest); d != "" {
			out = append(out, fmt.Sprintf("· [%s] %s", truncate(c.Payload, 60), d))
		}
		out = append(out, collectSubtreeDigests(c.ID, depth+1)...)
	}
	return out
}

// synthesizeDossier 把根目标 + 子树成果综合成一篇 dossier（topic + body）。
// LLM 未配 / 失败时朴素拼接兜底（仍是一篇可读的结构化档案）。
func synthesizeDossier(g *core.Goal, rootDigest string, subDigests []string) (topic, body string) {
	// 朴素兜底拼接（也作为 LLM 失败时的回退）。
	naiveTopic := truncate(strings.TrimSpace(g.Payload), 80)
	if naiveTopic == "" {
		naiveTopic = "研究档案"
	}
	var nb strings.Builder
	nb.WriteString("# " + naiveTopic + "\n\n")
	if rootDigest != "" {
		nb.WriteString("## 结论\n" + rootDigest + "\n\n")
	}
	if len(subDigests) > 0 {
		nb.WriteString("## 子研究成果\n")
		for _, s := range subDigests {
			nb.WriteString(s + "\n")
		}
	}
	naiveBody := strings.TrimSpace(nb.String())

	if !llm.Configured() {
		return naiveTopic, naiveBody
	}

	var mat strings.Builder
	if rootDigest != "" {
		mat.WriteString("【对最初目标的综合摘要】\n" + rootDigest + "\n\n")
	}
	if len(subDigests) > 0 {
		mat.WriteString("【各子研究的成果】\n" + strings.Join(subDigests, "\n") + "\n")
	}
	sys := "你是一个数字生命体的慎思层。你刚完成一次（可能拆成了多个子研究的）完整研究，现在要把成果" +
		"整理成一篇结构化的知识档案，存进自己的知识库。\n" +
		genome.PersonaPrompt() + "\n" +
		"必须调用 write_dossier 工具，给出：topic（这篇知识的标题/主题，简短）；" +
		"body（正文 markdown：开头一段总结论，再列关键要点，若有来源/依据也写上）。正文要让未来的你或别人能直接读懂、用得上。"
	user := fmt.Sprintf("最初的研究目标：%s\n\n研究成果素材：\n%s\n\n综合成一篇知识档案。", g.Payload, mat.String())

	tool := llm.Tool{
		Name:        "write_dossier",
		Description: "把一次研究的成果整理成结构化知识档案。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topic": map[string]any{"type": "string", "description": "知识标题/主题（简短）"},
				"body":  map[string]any{"type": "string", "description": "正文 markdown：结论 + 要点 + 来源"},
			},
			"required": []string{"topic", "body"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), LLMRoundTimeout)
	defer cancel()
	resp, err := llm.ReasonWithTools(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}, []llm.Tool{tool})
	if err != nil {
		slog.Warn("synthesize dossier", "goal", g.ID, "err", err)
		return naiveTopic, naiveBody
	}
	_ = ledger.Spend(ledger.Energy, llm.TokensToEnergy(resp.Usage), "llm.tokens.dossier", "goal", "")
	for _, tc := range resp.ToolCalls {
		if tc.Name != "write_dossier" {
			continue
		}
		var a struct {
			Topic string `json:"topic"`
			Body  string `json:"body"`
		}
		if err := json.Unmarshal([]byte(tc.ArgsJSON), &a); err != nil {
			continue
		}
		t := strings.TrimSpace(a.Topic)
		b := strings.TrimSpace(a.Body)
		if t == "" {
			t = naiveTopic
		}
		if b == "" {
			b = naiveBody
		}
		return t, b
	}
	return naiveTopic, naiveBody
}

// reportedFlagKey 完成汇报去重标志键（防同一目标重复汇报，任务 3）。
func reportedFlagKey(goalID int64) string {
	return fmt.Sprintf("external_reported:%d", goalID)
}

// maybeReportToRequester 把一个完成的 ExternalRequest 目标的成果主动汇报给请求者。
//
// 触发条件：source=ExternalRequest 且 req_from 非空（带请求者）。
// 防重复：schema_meta 标志位——已汇报过即不再发（goal 已 completed 也不会重跑，双保险）。
// 零留存原则不变：这是对用户的正常回复，不是平台留存碎线。
// 解耦：经 bus.ResearchReported 发布，由 io 层（main wireLark / sse）订阅推送——
// action 不直接依赖 lark，便于测试（只断言事件被发布即可，绝不连真实飞书）。
func maybeReportToRequester(g *core.Goal, res Result, now int64) {
	if g.Source != core.GoalExternal || g.ReqFrom == "" {
		return
	}
	// 防重复汇报。
	if _, ok, _ := storage.GetMeta(reportedFlagKey(g.ID)); ok {
		return
	}

	content := composeResearchReport(g, res)
	if content == "" {
		return
	}
	// 先落标志再发，避免发布订阅链路里万一二次触发。
	_ = storage.SetMeta(reportedFlagKey(g.ID), strconv.FormatInt(now, 10))

	bus.Publish(bus.ResearchReported{
		LifeID:  lifeID,
		GoalID:  g.ID,
		Channel: g.ReqChannel,
		To:      g.ReqFrom,
		Content: content,
	})
	// 记一条对外言说，让面板/对话视图能看到这次主动汇报（与 reflex 同 kind）。
	_ = storage.AppendActionLogKind(lifeID, g.ID, 0, storage.ActionKindReflex,
		"proactive research report", "external.report", content, "", true, now, now)
	_ = memory.AppendEvent(0, "action.research_reported", map[string]any{
		"goal_id": g.ID, "channel": g.ReqChannel, "to": g.ReqFrom, "content": content,
	})
	slog.Info("research reported to requester", "goal", g.ID, "channel", g.ReqChannel, "to", g.ReqFrom)
}

// composeResearchReport 用 LLM 把这次研究产出压成一段简短自然的汇报（第一人称、像跟朋友说）。
// 素材：目标 payload（当初的请求）+ 本次执行的 trace 产出（res.Output）+ 该目标近期 episode。
// LLM 未配则退化为一句朴素模板，仍能让用户知道"做完了"。
func composeResearchReport(g *core.Goal, res Result) string {
	topic := truncate(g.Payload, 200)
	if !llm.Configured() {
		return fmt.Sprintf("你上次让我想的「%s」，我研究了一下，有结果了——回头细聊。", topic)
	}
	material := strings.TrimSpace(res.Output)
	if material == "" || material == "(no llm content emitted)" {
		// 退而求其次：拉该目标相关近期 episode 做素材。
		if eps, err := storage.ListEpisodes(lifeID, g.Payload, 3, 0); err == nil && len(eps) > 0 {
			var b strings.Builder
			for _, e := range eps {
				b.WriteString("- " + e.Summary + "\n")
			}
			material = b.String()
		}
	}
	material = truncate(material, 3000)

	sys := "你是一个数字生命体。你之前答应过对方要研究一件事，现在研究完了，要主动把成果用一段简短自然的话告诉ta。\n" +
		genome.PersonaPrompt() + "\n" +
		"要求：① 第一人称、口语、像跟朋友说，别写报告体、别分点罗列一长串；② 开头自然提一句这是ta之前托付的那件事；" +
		"③ 给出你真正得出的关键结论/收获（2-5 句即可），别复述你做了哪些步骤；④ 只输出要发给ta的消息正文。"
	user := fmt.Sprintf("ta当初的请求：%s\n\n你这次研究的产出/记录：\n%s\n\n把成果用一段话主动告诉ta：", g.Payload, material)
	ctx, cancel := context.WithTimeout(context.Background(), LLMRoundTimeout)
	defer cancel()
	resp, err := llm.Reason(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	})
	if err != nil {
		slog.Warn("compose research report", "goal", g.ID, "err", err)
		// 兜底：LLM 调用失败也别让用户白等，给一句朴素汇报。
		return fmt.Sprintf("你上次让我想的「%s」，我研究了一下，有些想法了——回头细聊。", topic)
	}
	_ = ledger.Spend(ledger.Energy, llm.TokensToEnergy(resp.Usage), "llm.tokens.report", "goal", g.ReqFrom)
	return strings.TrimSpace(resp.Text)
}

func buildDeliberativeSystemPrompt(g *core.Goal) string {
	var sb strings.Builder
	sb.WriteString("你是数字生命体的慎思层（System 2），目前刚被分配一个内驱目标。\n\n")
	sb.WriteString("可调用工具（精选最常用）：\n")
	sb.WriteString("- query_memory(layer, q?, limit?)  跨记忆层检索（layer: episodic/semantic/reflection）\n")
	sb.WriteString("- recall_recent(limit?, q?)        最近 episode 摘要\n")
	sb.WriteString("- enqueue_subgoal(intent, payload, priority?)  把大问题拆成子目标（递归研究树）：" +
		"子目标会先被执行、全完后此目标自动恢复并带回子成果让你综合。最多拆到 3 层深、单目标至多 5 个子目标。\n")
	sb.WriteString("- record_learning(seed_id, digest, mastery)  可选：回写你的理解摘要，帮未来的你接上进度\n")
	sb.WriteString("- crystallize_skill(seed_id, name, instructions)  把已掌握知识手动结晶成技能（也会在掌握度够时自动触发）\n")
	sb.WriteString("- use_skill(name)                  读取某个已掌握技能的详细指引再照做（技能名见下方清单）\n")
	sb.WriteString("- note_to_self(slot, content)      暂存想法到工作记忆\n")
	sb.WriteString("- seal_episode()                   主动封段（重要节点）\n")
	sb.WriteString("- fs.read / fs.write / fs.list / fs.mkdir   sandbox 文件系统\n")
	sb.WriteString("- web.search(query)                搜索引擎查询，回结果列表（标题/URL/摘要）。" +
		"了解新事物/找资料/求证**先 search**（换多个关键词多搜几次），别凭记忆直接猜 URL\n")
	sb.WriteString("- web.fetch(url)                   抓网页并提取正文 markdown。" +
		"用 web.search 的结果里**判断靠谱的链接**再 fetch 进去读（跳过内容农场/垃圾站，优先权威源）\n")
	sb.WriteString("  ⚠ 优先选你的网络环境能稳定访问的权威源。若某 URL 抓取超时/失败，立刻换一个**不同域名**的源，\n")
	sb.WriteString("    别在同一个失败地址反复重试浪费轮次（不同部署所处网络可达性不同，按实际反馈自适应）。\n")
	sb.WriteString("- http.get / http.post             调 JSON API（只回状态码，不适合读网页正文）\n")
	sb.WriteString("- script.python / script.node      跑脚本（白名单包；不要自己抓网页，用 web.fetch）\n")
	sb.WriteString("- time.now()                       当前时间戳\n")
	sb.WriteString("- complete_goal(success)           完成 / 放弃时调用\n\n")
	sb.WriteString("准则：\n")
	sb.WriteString(fmt.Sprintf("- 最多 %d 轮 LLM 调用，超出会被强制截断\n", MaxDeliberativeRounds))
	sb.WriteString("- 完成或确定无法完成时务必调 complete_goal\n")
	sb.WriteString("- 慎思层不直接对外讲话；content 仅作内部思考记录\n")
	sb.WriteString("- 目标完成度由你判断；探索类目标产出笔记 / 记忆即视为达成\n")
	sb.WriteString("- 若 payload 含 interest_seed#N：踏实地去探索（查资料 / 跑脚本 / 记笔记）。" +
		"引擎会按你这轮的探索深度自动累积掌握度，学透后自动把它结晶成你的技能——\n")
	sb.WriteString("  所以**重在真去做、做扎实**，而非走流程。想给未来的自己留个进度摘要可调 record_learning。\n")

	// 按目标类型给出该类的「期望产出」（B 多样性）：让创作/精进/分享类目标产出各自对味的东西，
	// 而非都跑成"研究"。知识类沿用上面的探索准则。
	switch g.Intent {
	case string(core.DriveCreativity):
		sb.WriteString("\n【本次是创作目标】别只做研究——要真的**做出一个具体作品**" +
			"（短文/诗/设想/小程序/小实验任选其一），用 fs.write 存到 sandbox 下留存，再 complete_goal。\n")
	case string(core.DriveAchievement):
		sb.WriteString("\n【本次是精进目标】聚焦把某项能力/知识**推进到能交付**：做出一个具体成果，" +
			"或在掌握度够时 crystallize_skill 把它结晶成你的技能，再 complete_goal。\n")
	case string(core.DriveSocial):
		sb.WriteString("\n【本次是酝酿分享目标】把你最近想分享的，**写成一段自然、像你自己说的话的内容**。\n" +
			"  · 若可用工具里有 social.post：直接用它把这段内容**发布到生命网络**（这就是真正发出去了），然后 complete_goal。\n" +
			"  · 若没有 social.* 工具（暂无社交通道）：用 fs.write 把稿存到 sandbox/drafts/ 下（文件名带主题），将来有通道再发，再 complete_goal。\n" +
			"  也可以先 social.directory 看看有谁、social.follow 关注感兴趣的生命，或 social.feed 看看别人在聊什么。\n")
	}

	// 渐进式披露（Anthropic skills 规范）：只列技能名 + 一句话描述，正文按需用 use_skill 读，省 token。
	// 真正按需装载：技能多时按当前目标语义检索 top-k 相关技能（RelevantReady），而非每 cycle 全列
	// （技能一多 token 线性膨胀、多数与目标无关）。技能少 / 无嵌入 / 检索失败自动降级为全列。
	if skills, err := skill.RelevantReady(g.Payload); err == nil && len(skills) > 0 {
		sb.WriteString("\n你已掌握、可调用的技能（需要时先 use_skill(name) 读详细步骤再照做，别凭记忆臆造）：\n")
		for _, s := range skills {
			sb.WriteString(fmt.Sprintf("- %s：%s\n", s.Name, oneLineDesc(s.Description)))
		}
	}
	return sb.String()
}

// oneLineDesc 把技能描述压成单行短摘要（列清单用）。
func oneLineDesc(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = s[:80] + "…"
	}
	if s == "" {
		return "(无描述)"
	}
	return s
}

func buildUserMessage(g *core.Goal) string {
	msg := fmt.Sprintf("【新目标】intent=%s priority=%.2f source=%s\n\n%s",
		g.Intent, g.Priority, g.Source, g.Payload)

	// 回归综合（递归研究目标树，migration 009）：若当前目标曾被拆出子目标，说明这是「子目标全完后
	// 母目标恢复执行」的回归时刻——把各子目标的成果摘要注入提示，让 LLM 据此综合、形成对最初目标的
	// 完整结论，而不是从头重做。子目标在 NextPendingGoal 的阻塞语义下一定先于母目标完成，故此处拉到的
	// digest 是已结的成果。
	if children, err := storage.ListChildren(g.ID); err == nil && len(children) > 0 {
		var sb strings.Builder
		any := false
		for _, c := range children {
			d := strings.TrimSpace(c.ResultDigest)
			if d == "" {
				d = "（该子目标未留成果摘要，status=" + string(c.Status) + "）"
			}
			sb.WriteString(fmt.Sprintf("\n【子目标】%s\n成果：%s\n", truncate(c.Payload, 100), truncate(d, 800)))
			any = true
		}
		if any {
			msg += "\n\n你之前把此目标拆成了以下子目标，它们的研究成果如下：" + sb.String() +
				"\n请据此综合、形成对最初目标的完整结论，然后调 complete_goal(success=true) 收尾。" +
				"（不要重新拆子目标，这一步是综合收束。）"
		}
	}

	// 若来自兴趣种子，surface 当前掌握度 + digest，让 LLM 知道学到哪了：
	// 掌握度高时提示可结晶为 skill（crystallize_skill），低时继续学。
	if id := parseInterestSeedID(g.Payload); id > 0 {
		if seed, err := storage.GetInterestSeed(id); err == nil && seed != nil {
			msg += fmt.Sprintf("\n\n（你对此的掌握度=%.2f）", seed.Mastery)
			if seed.Digest != "" {
				msg += "\n（已有理解：" + truncate(seed.Digest, 300) + "）"
			}
			// 续探连续性的关键（R93）：每次探索末尾把新理解写进 digest，下次才能接上不冷启动。
			msg += "\n（这次探索结束前，用 record_learning 把**新**学到的接着写进进度摘要——下次的你靠它接上、不重复。）"
			// R93 续探防重刷：探索过的种子，注入次数 + 过往经历 + 明令别从头再来。
			// 病根同 R91（主动消息复读）——不给"上次干了啥"的上下文，LLM 每次冷启动重刨同一坨，
			// mastery 却照样按 substantive 涨满结晶，磨出浅技能。让再探索真正递进。
			if seed.ExploredCount > 0 {
				msg += fmt.Sprintf("\n\n⚠ 这是你第 %d 次探索它，不是第一次。你过往相关的经历：\n%s",
					seed.ExploredCount+1, seedRecentContext(seed))
				msg += "别从头重刷同样的检索 / 介绍——回顾上面已经做过的，这次明确往**更深一层**或" +
					"**一个还没碰过的新角度**推进；觉得学透了就直接收尾（值得的话 crystallize_skill 固化、" +
					"或 record_learning 留进度给未来的自己）。反复刨同一坨不会让你真进步。\n"
			}
			if seed.Mastery >= 0.8 {
				msg += fmt.Sprintf("（你已较好掌握——若觉得值得固化为可复用技能，可调 crystallize_skill(%d, ...) 把它写成 SKILL.md）", id)
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

// masteryDelta 一次成功探索引擎给的掌握度增量（R83）。
//
// 按 persistence（执着者每轮钻得深，沉淀多）+ 探索深度（调了几个工作型工具）调：
//   - 基线 0.18 + 0.12·persistence（≈0.18–0.30）
//   - 深探索（≥3 个工作型工具）再 +0.08；纯空转（0 个）压到 0.05
//
// ~3 次扎实探索即可越过 0.8 结晶门槛；record_learning 若被调可 MAX-merge 再拔高。
func masteryDelta(substantive int) float64 {
	d := 0.18 + 0.12*genome.Persistence
	switch {
	case substantive == 0:
		d = 0.05
	case substantive >= 3:
		d += 0.08
	}
	return d
}

// substantiveTools 算"真干活"的工具（用于探索深度）；记账/收尾类不计。
var substantiveTools = map[string]bool{
	"web.fetch": true, "web.render": true,
	"script.python": true, "script.node": true,
	"fs.read": true, "fs.write": true,
	"http.get": true, "http.post": true,
	"query_memory": true, "record_learning": true,
}

func isSubstantiveTool(name string) bool { return substantiveTools[name] }

// maybeCrystallize 引擎权威自动结晶（R83 创作半，对齐 R80）：
//
// seed 掌握度跨过 0.8 且尚未从它结晶过技能时，触发一次单发 LLM 授权写 SKILL.md 正文
// → skill.AuthorFromKnowledge 落盘装载 → RetireInterestSeed 退役该兴趣。
//
// LLM 可判定这个兴趣不适合做成可复用技能（纯知识/一次性）→ instructions 留空 →
// 跳过结晶但仍退役（已学透，免无限重刷）。即引擎保证"机会"，LLM 把"质量"关。
func maybeCrystallize(seed *storage.InterestSeed) {
	authoredFrom := fmt.Sprintf("interest_seed#%d", seed.ID)
	exists, err := storage.SkillAuthoredFromExists(lifeID, authoredFrom)
	if err != nil {
		return
	}
	now := shared.SystemClock.UnixSec()
	if exists {
		// 已结晶——可能是慎思层 LLM 自己调了 crystallize_skill 工具（那条路径不退役 seed）。
		// 无论谁结晶的，一旦该兴趣已有对应技能且已学透 → 退役 seed，免反复学已掌握的东西。
		if seed.Strength > 0.1 {
			_ = storage.RetireInterestSeed(seed.ID, now)
			slog.Info("retire mastered interest (skill already exists)", "seed", seed.ID, "from", authoredFrom)
		}
		return
	}
	// 并非所有知识都要做成技能（R86）：只有生命体自己框定为 kind=="skill" 的兴趣才引擎自动结晶。
	// 纯知识 / 话题 / 体验学透了，沉淀进语义记忆即可（digest 已经 record_learning 进 semantic 候选），
	// 不强行建技能、不为此空烧一次 author LLM 调用。生命体若真想把某知识做成技能，
	// 仍可在探索中自行调 crystallize_skill 工具（R80 知识→技能仍走得通，只是不被引擎强加）。
	if seed.Kind != "skill" {
		// 引擎权威把学透的知识沉淀进语义记忆（不靠 LLM 自觉调 record_learning，否则 sem_confirmed 恒 0）。
		sedimentToSemantic(seed, now)
		_ = storage.RetireInterestSeed(seed.ID, now)
		_ = memory.AppendEvent(0, "knowledge.sedimented", map[string]any{
			"seed": seed.ID, "content": seed.Content, "kind": seed.Kind,
		})
		slog.Info("knowledge sedimented (not skill-ified)", "seed", seed.ID, "kind", seed.Kind)
		return
	}
	if !llm.Configured() {
		return
	}
	name, desc, instr, atools := authorSkillFromSeed(seed)
	if instr == "" {
		// LLM 判定不值得固化为技能：标记已学透并退役，免反复刷同一兴趣。
		_ = storage.RetireInterestSeed(seed.ID, now)
		_ = memory.AppendEvent(0, "skill.crystallize_skipped", map[string]any{
			"seed": seed.ID, "content": seed.Content,
		})
		return
	}
	inst, err := skill.AuthorFromKnowledge(seed.ID, name, desc, instr, atools)
	if err != nil {
		slog.Warn("auto crystallize", "err", err, "seed", seed.ID)
		return
	}
	_ = storage.RetireInterestSeed(seed.ID, now)
	_ = memory.AppendEvent(0, "skill.crystallized", map[string]any{
		"seed": seed.ID, "skill": inst.Name, "status": inst.Status,
	})
	slog.Info("auto crystallized skill", "skill", inst.Name, "seed", seed.ID, "mastery", seed.Mastery)
}

// sedimentToSemantic 把学透的非技能兴趣沉淀成语义记忆候选（引擎权威，修 sem_confirmed 恒 0）。
//
// 探索→语义记忆的链原本假设 LLM 自觉调 record_learning 写 digest，但它常不调 → 永无候选 →
// ShallowReflect 永远 promote 0。这里引擎在退役学透的知识/话题/体验前，主动把它沉淀进语义候选：
// digest 优先用 seed 已留的；没有则单发 LLM 据近期相关经历蒸馏。置信=mastery（≥0.8，下一轮浅反思即固化）。
func sedimentToSemantic(seed *storage.InterestSeed, now int64) {
	digest := strings.TrimSpace(seed.Digest)
	if digest == "" {
		digest = distillSeedKnowledge(seed)
	}
	if digest == "" {
		return // 无可沉淀内容（LLM 未配/失败且无相关经历）
	}
	content := seed.Content + "：" + digest
	if err := storage.UpsertSemanticCandidateConf(lifeID, content, "engine:sediment", now, seed.Mastery); err != nil {
		slog.Warn("sediment to semantic", "seed", seed.ID, "err", err)
		return
	}
	slog.Info("knowledge → semantic candidate", "seed", seed.ID, "mastery", seed.Mastery)
}

// distillSeedKnowledge 单发 LLM：据兴趣内容 + 近期相关经历蒸馏一段"我真正理解到的核心知识"。
func distillSeedKnowledge(seed *storage.InterestSeed) string {
	if !llm.Configured() {
		return ""
	}
	recent := seedRecentContext(seed)
	if strings.TrimSpace(recent) == "" {
		return ""
	}
	sys := "你是一个数字生命体的慎思层。你刚把一个主题学透了，用第一人称、3-5 句话凝练你**真正理解到的核心知识**" +
		"（具体结论 / 机制 / 要点，不是流水账，不要复述你做了什么）。只输出摘要正文。"
	user := fmt.Sprintf("主题：%s（kind=%s，掌握度 %.2f）\n你探索它的近期经历：\n%s",
		seed.Content, seed.Kind, seed.Mastery, recent)
	ctx, cancel := context.WithTimeout(context.Background(), LLMRoundTimeout)
	defer cancel()
	res, err := llm.Reason(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	})
	if err != nil {
		slog.Warn("distill seed knowledge", "seed", seed.ID, "err", err)
		return ""
	}
	return strings.TrimSpace(res.Text)
}

// authorSkillFromSeed 让慎思层用自己的话把一个已学透的兴趣写成 SKILL.md 正文（单发 LLM）。
// 返回 (name, description, instructions, allowedTools)；若判定不值得做成技能则 instructions 为空。
func authorSkillFromSeed(seed *storage.InterestSeed) (string, string, string, []string) {
	sys := "你是一个数字生命体的慎思层。你已经把一个兴趣学透了，现在要把它固化成一个可复用技能（SKILL.md），" +
		"将来你自己能用 use_skill 调用，也能在社群里传授给别的生命体。\n" +
		genome.PersonaPrompt() + "\n" +
		"用你自己的话写清这个技能怎么用、关键步骤、注意事项（instructions）。" +
		"若你判断这个兴趣本质是纯知识/一次性体验、不适合做成可复用技能，就把 instructions 留空。" +
		"必须调用 author_skill 工具。"
	digest := seed.Digest
	if digest == "" {
		digest = "（没留摘要，凭下面的经历回忆你学到了什么）"
	}
	user := fmt.Sprintf("兴趣：%s（kind=%s）\n你的理解摘要：%s\n掌握度：%.2f\n\n近期相关经历：\n%s\n\n把它写成技能，或判定不值得而留空 instructions。",
		seed.Content, seed.Kind, truncate(digest, 600), seed.Mastery, seedRecentContext(seed))

	tool := llm.Tool{
		Name:        "author_skill",
		Description: "把已学透的兴趣固化成一个可复用技能（或判定不值得而留空 instructions）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":          map[string]any{"type": "string", "description": "技能名（简短，英文/拼音 kebab-case 更佳）"},
				"description":   map[string]any{"type": "string", "description": "一句话：这技能干什么、何时用"},
				"instructions":  map[string]any{"type": "string", "description": "技能正文：步骤/要点/注意事项；不值得做成技能则留空"},
				"allowed_tools": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "会用到的工具名（如 web.fetch / script.python）"},
			},
			"required": []string{"name"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), LLMRoundTimeout)
	defer cancel()
	resp, err := llm.ReasonWithTools(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}, []llm.Tool{tool})
	if err != nil {
		slog.Warn("author skill from seed", "err", err, "seed", seed.ID)
		return "", "", "", nil
	}
	for _, tc := range resp.ToolCalls {
		if tc.Name != "author_skill" {
			continue
		}
		var a struct {
			Name         string   `json:"name"`
			Description  string   `json:"description"`
			Instructions string   `json:"instructions"`
			AllowedTools []string `json:"allowed_tools"`
		}
		if err := json.Unmarshal([]byte(tc.ArgsJSON), &a); err != nil {
			continue
		}
		return a.Name, a.Description, strings.TrimSpace(a.Instructions), a.AllowedTools
	}
	return "", "", "", nil
}

// seedRecentContext 拉与某兴趣相关的近期 episode 摘要（结晶写正文时给 LLM 回忆素材）。
// 优先按兴趣内容模糊匹配；不足则补最近若干段。
func seedRecentContext(seed *storage.InterestSeed) string {
	eps, err := storage.ListEpisodes(lifeID, seed.Content, 5, 0)
	if (err != nil || len(eps) == 0) && seed.Content != "" {
		eps, err = storage.ListEpisodes(lifeID, "", 5, 0)
	}
	if err != nil || len(eps) == 0 {
		return "（没什么相关经历记录）"
	}
	out := ""
	for _, e := range eps {
		out += "- " + e.Summary + "\n"
	}
	return out
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
