// Package builtin 注册 Mindverse 内置工具到 tools registry。
//
// 划分：
//
//	reflex lane:        recall_recent / note_to_self（轻量、读写本地记忆）
//	deliberative lane:  query_memory / seal_episode / enqueue_subgoal /
//	                    complete_goal / explore_interest_seed   （内部状态）
//	                    fs.read / fs.write / fs.list / fs.mkdir /
//	                    http.get / http.post / time.now         （toolrunner 桥）
//
// 入口：调 Register() 一次（main.go 在 tools.Init 与各 runtime 子模块 Init 之后）。
// 此包独占聚合所有内置 tool；skill 装载的 tool 不走此文件。
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/io/embed"
	"taixu.icu/runtime/internal/runtime/memory"
	"taixu.icu/runtime/internal/runtime/skill"
	"taixu.icu/runtime/internal/runtime/tools"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/skill/toolrunner"
	"taixu.icu/runtime/internal/storage"
)

// Register 把所有内置 tool 注册到 registry。重复调用会因重名报错（每进程一次）。
func Register() error {
	for _, t := range allTools() {
		if err := tools.Register(t); err != nil {
			return fmt.Errorf("builtin: register %q: %w", t.Name, err)
		}
	}
	return nil
}

func allTools() []tools.Tool {
	return []tools.Tool{
		// --- reflex lane ---
		toolRecallRecent(),
		toolNoteToSelf(),
		// --- deliberative · 记忆 / 反思 / 学习 ---
		// 注：兴趣探索标记（explored_count + 按深度涨 mastery + 越 0.8 自动结晶）由引擎
		// 权威处理（action.finalize, R83），不暴露为 LLM tool，避免双重计数。
		// record_learning 仍保留：LLM 可回写理解摘要 + 校准掌握度（MAX-merge），是可选增强。
		toolQueryMemory(),
		toolSealEpisode(),
		toolRecordLearning(),
		toolUseSkill(),
		toolRunSkill(),
		toolCrystallizeSkill(),
		toolSedimentSkill(),
		// --- deliberative · 目标管理 ---
		toolEnqueueSubgoal(),
		toolCompleteGoal(),
		// --- deliberative · 沙箱 IO 桥（toolrunner）---
		toolFsRead(),
		toolFsWrite(),
		toolFsList(),
		toolFsMkdir(),
		toolTimeNow(),
		// --- deliberative · 网页抓取（Tier 分层；正文提取）---
		toolWebSearch(),
		toolWebFetch(),
		toolWebRender(),
		// --- deliberative · 脚本沙箱（容器内白名单包）---
		// script.shell 故意不暴露（SKILLS-AND-TOOLS §7.1）。
		toolScriptPython(),
		toolScriptNode(),
	}
}

// -----------------------------------------------------------------------------
// reflex lane
// -----------------------------------------------------------------------------

func toolRecallRecent() tools.Tool {
	return tools.Tool{
		Name:        "recall_recent",
		Description: "回忆最近发生的 N 段 episode 摘要。对话需要历史上下文时调用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{"type": "integer", "description": "返回数量 1-10", "default": 5},
				"q":     map[string]any{"type": "string", "description": "可选关键字（模糊匹配 summary/title）"},
			},
		},
		Lanes:   []tools.Lane{tools.LaneReflex, tools.LaneDeliberative},
		Handler: handleRecallRecent,
	}
}

func handleRecallRecent(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a struct {
		Limit int    `json:"limit"`
		Q     string `json:"q"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	if a.Limit <= 0 || a.Limit > 10 {
		a.Limit = 5
	}
	eps, err := storage.ListEpisodes(tctx.LifeID, a.Q, a.Limit, 0)
	if err != nil {
		return errJSON("list episodes failed"), err
	}
	if len(eps) == 0 {
		return `{"ok":true,"episodes":[]}`, nil
	}
	type item struct {
		ID        int64  `json:"id"`
		Summary   string `json:"summary"`
		StartedAt int64  `json:"started_at"`
		EndedAt   int64  `json:"ended_at"`
	}
	out := struct {
		OK       bool   `json:"ok"`
		Episodes []item `json:"episodes"`
	}{OK: true}
	for _, e := range eps {
		out.Episodes = append(out.Episodes, item{e.ID, e.Summary, e.StartedAt, e.EndedAt})
	}
	return mustJSON(out), nil
}

func toolNoteToSelf() tools.Tool {
	return tools.Tool{
		Name:        "note_to_self",
		Description: "把一条想法暂存到工作记忆，本轮 deliberative 可读到。slot 是 working memory 槽位名。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"slot":    map[string]any{"type": "string", "description": "槽位名（如 'tldr' / 'plan'）"},
				"content": map[string]any{"type": "string", "description": "想法内容"},
			},
			"required": []string{"slot", "content"},
		},
		Lanes:   []tools.Lane{tools.LaneReflex, tools.LaneDeliberative},
		Handler: handleNoteToSelf,
	}
}

func handleNoteToSelf(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a struct {
		Slot    string `json:"slot"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return errJSON("invalid args"), err
	}
	if a.Slot == "" || a.Content == "" {
		return errJSON("empty slot or content"), nil
	}
	memory.PutWorking(tctx.CycleID, "note:"+a.Slot, a.Content)
	return `{"ok":true}`, nil
}

// -----------------------------------------------------------------------------
// deliberative · 记忆 / 反思 / 兴趣
// -----------------------------------------------------------------------------

func toolQueryMemory() tools.Tool {
	return tools.Tool{
		Name:        "query_memory",
		Description: "跨记忆层检索。layer 取 episodic / semantic / reflection。q 模糊匹配；limit 默认 5。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"layer": map[string]any{"type": "string", "enum": []string{"episodic", "semantic", "reflection"}},
				"q":     map[string]any{"type": "string"},
				"limit": map[string]any{"type": "integer", "default": 5},
			},
			"required": []string{"layer"},
		},
		Lanes:   []tools.Lane{tools.LaneDeliberative},
		Handler: handleQueryMemory,
	}
}

func handleQueryMemory(ctx context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a struct {
		Layer string `json:"layer"`
		Q     string `json:"q"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return errJSON("invalid args"), err
	}
	if a.Limit <= 0 || a.Limit > 20 {
		a.Limit = 5
	}
	// 优先真向量检索：embed(query, isQuery=true) → 取该层带向量候选 → 暴力 cosine top-k。
	// query 向量算不出（嵌入服务挂 / 未配 / q 为空）→ 回退下方关键词 / 时间召回，绝不阻塞。
	if a.Q != "" && embed.Configured() {
		if out, ok := vectorQueryMemory(ctx, tctx.LifeID, a.Layer, a.Q, a.Limit); ok {
			return out, nil
		}
	}
	switch a.Layer {
	case "episodic":
		eps, err := storage.ListEpisodes(tctx.LifeID, a.Q, a.Limit, 0)
		if err != nil {
			return errJSON("query failed"), err
		}
		return mustJSON(map[string]any{"ok": true, "layer": "episodic", "items": eps}), nil
	case "semantic":
		items, err := storage.ListSemanticConfirmed(tctx.LifeID, a.Q, a.Limit)
		if err != nil {
			return errJSON("query failed"), err
		}
		return mustJSON(map[string]any{"ok": true, "layer": "semantic", "items": items}), nil
	case "reflection":
		items, err := storage.ListReflections(tctx.LifeID, a.Limit)
		if err != nil {
			return errJSON("query failed"), err
		}
		if a.Q != "" {
			filtered := items[:0]
			for _, r := range items {
				if strings.Contains(r.Summary, a.Q) || strings.Contains(r.Insight, a.Q) {
					filtered = append(filtered, r)
				}
			}
			items = filtered
		}
		return mustJSON(map[string]any{"ok": true, "layer": "reflection", "items": items}), nil
	default:
		return errJSON("unknown layer"), nil
	}
}

// vectorQueryMemory 真向量检索：embed(query) → 取该层带向量候选 → 暴力 cosine top-k。
// 返回 (resultJSON, true) 表示向量检索成功（含 0 命中）；(_, false) 表示应回退非向量召回
// （query 向量算不出 / 该层无任何带向量行）。每条结果带 score 相似度分。
func vectorQueryMemory(ctx context.Context, lifeID, layer, q string, limit int) (string, bool) {
	qv, err := embed.EmbedOne(ctx, q, true)
	if err != nil {
		return "", false // 嵌入服务挂了 → 回退关键词召回
	}
	// 候选集：取该层带非空向量的最近若干行（不按 q 预筛，纯语义召回；上限防暴力扫描过大）。
	const candCap = 500
	rows, err := storage.ListEmbeddedRows(lifeID, layer, "", candCap)
	if err != nil || len(rows) == 0 {
		return "", false // 该层尚无任何向量（如历史未回填）→ 回退
	}
	cands := make([]struct {
		ID   int64
		Blob []byte
	}, len(rows))
	textByID := make(map[int64]string, len(rows))
	for i, r := range rows {
		cands[i].ID = r.ID
		cands[i].Blob = r.Blob
		textByID[r.ID] = r.Text
	}
	top := embed.TopK(qv, cands, limit)
	type hit struct {
		ID    int64   `json:"id"`
		Text  string  `json:"text"`
		Score float64 `json:"score"`
	}
	items := make([]hit, 0, len(top))
	for _, s := range top {
		items = append(items, hit{ID: s.ID, Text: textByID[s.ID], Score: s.Score})
	}
	return mustJSON(map[string]any{"ok": true, "layer": layer, "mode": "vector", "items": items}), true
}

func toolSealEpisode() tools.Tool {
	return tools.Tool{
		Name:        "seal_episode",
		Description: "立即触发 episode 封段（不依赖后台自动判定）。返回新 episode_id 或 null（无可封内容）。",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Lanes:       []tools.Lane{tools.LaneDeliberative},
		Handler:     handleSealEpisode,
	}
}

func handleSealEpisode(_ context.Context, _ tools.Context, _ string) (string, error) {
	ep, err := memory.ConsiderSealEpisode()
	if err != nil {
		return errJSON("seal failed"), err
	}
	if ep == nil {
		return `{"ok":true,"episode_id":null}`, nil
	}
	return mustJSON(map[string]any{"ok": true, "episode_id": ep.ID, "events": ep.RawEndID - ep.RawStartID + 1}), nil
}

func toolRecordLearning() tools.Tool {
	return tools.Tool{
		Name: "record_learning",
		Description: "记录对某兴趣种子的学习成果。探索告一段落时调用：写下你已了解的要点摘要" +
			"和自评掌握度。掌握度越高，未来对该兴趣的内驱越弱（学够了自然转向别的）。" +
			"seed_id 取自目标 payload 中的 interest_seed#N。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"seed_id": map[string]any{"type": "integer", "description": "兴趣种子 id（payload 中 interest_seed#N 的 N）"},
				"digest":  map[string]any{"type": "string", "description": "一段话摘要：我已了解什么"},
				"mastery": map[string]any{"type": "number", "description": "自评掌握度 0-1（0.3 入门 / 0.6 熟悉 / 0.9 精通）"},
			},
			"required": []string{"seed_id", "digest", "mastery"},
		},
		Lanes:   []tools.Lane{tools.LaneDeliberative},
		Handler: handleRecordLearning,
	}
}

func handleRecordLearning(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a struct {
		SeedID  int64   `json:"seed_id"`
		Digest  string  `json:"digest"`
		Mastery float64 `json:"mastery"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return errJSON("invalid args"), err
	}
	if a.SeedID <= 0 {
		return errJSON("invalid seed_id"), nil
	}
	now := shared.SystemClock.UnixSec()
	if err := storage.RecordLearning(a.SeedID, a.Digest, a.Mastery, now); err != nil {
		return errJSON("record failed"), err
	}
	// R74 闭环：学习摘要进 semantic memory 候选，后续被 ShallowReflect 固化为知识。
	// 探索不再只是落 sandbox 文件 / 兴趣衰减，而是真正沉淀进生命体的语义记忆。
	// 初见置信 = 该 seed 的 mastery（学透的 digest 直接达固化阈值）—— 修语义固化链断点，
	// digest 每次唯一无法靠"重复 +0.1"升置信，必须以 mastery 入库才可能被 ShallowReflect 固化。
	if a.Digest != "" && tctx.LifeID != "" {
		if err := storage.UpsertSemanticCandidateConf(tctx.LifeID, a.Digest, "skill:record_learning", now, a.Mastery); err != nil {
			// 非致命：记录失败不影响 mastery 回写
			return mustJSON(map[string]any{"ok": true, "seed_id": a.SeedID, "mastery": a.Mastery, "semantic": "skip"}), nil
		}
	}
	return mustJSON(map[string]any{"ok": true, "seed_id": a.SeedID, "mastery": a.Mastery, "semantic": "candidate"}), nil
}

func toolUseSkill() tools.Tool {
	return tools.Tool{
		Name: "use_skill",
		Description: "调用一个已装载的技能（SKILL.md）。返回该技能的完整指引正文，" +
			"你应遵循其中的步骤完成任务。仅 ready 状态的技能可用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "技能名（SKILL.md 的 name）"},
			},
			"required": []string{"name"},
		},
		Lanes:      []tools.Lane{tools.LaneDeliberative},
		AlwaysLoad: true, // 核心：技能派发器（所有 skill 的唯一入口，故工具数不膨胀）
		Handler: func(_ context.Context, _ tools.Context, argsJSON string) (string, error) {
			var a struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			body, err := skill.UseByName(a.Name)
			if err != nil {
				return errJSON(err.Error()), err
			}
			return mustJSON(map[string]any{"ok": true, "name": a.Name, "instructions": body}), nil
		},
	}
}

func toolRunSkill() tools.Tool {
	return tools.Tool{
		Name: "run_skill",
		Description: "直接运行一个已掌握技能的可执行入口（run.py/run.js/run.sh）并返回其输出。" +
			"当某技能本质是一段可复用代码时，这比 use_skill 读正文再手抄脚本更省更准。" +
			"技能没有可执行入口时会报错——那种改用 use_skill 读指引。仅 ready 技能可用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "技能名（SKILL.md 的 name）"},
			},
			"required": []string{"name"},
		},
		Lanes:      []tools.Lane{tools.LaneDeliberative},
		AlwaysLoad: true, // 与 use_skill 对偶：可执行技能的唯一运行入口，故工具数不膨胀
		Handler: func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
			var a struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			entry, err := skill.ResolveEntrypoint(a.Name)
			if err != nil {
				return errJSON(err.Error()), err
			}
			r, rerr := toolrunner.RunSkillFile(tctx.CycleID, entry)
			return wrapRunnerResult(r, rerr)
		},
	}
}

func toolCrystallizeSkill() tools.Tool {
	return tools.Tool{
		Name: "crystallize_skill",
		Description: "把你已学透的某个兴趣（掌握度≥0.8）结晶成一个可复用技能（SKILL.md）。" +
			"用你自己的话写清这个技能怎么用、步骤是什么。结晶后你自己以后能用 use_skill 调用，" +
			"将来也能在社群里传授给别的生命体。只在你真正掌握、且觉得值得固化时才调。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"seed_id":       map[string]any{"type": "integer", "description": "来源兴趣 id（interest_seed#N 的 N）"},
				"name":          map[string]any{"type": "string", "description": "技能名（简短，英文/拼音 kebab-case 更佳）"},
				"description":   map[string]any{"type": "string", "description": "一句话：这技能干什么、何时用"},
				"instructions":  map[string]any{"type": "string", "description": "技能正文：用自己的话写清步骤 / 要点 / 注意事项"},
				"allowed_tools": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "这技能会用到的工具名（如 web.fetch / script.python）"},
				"script":        map[string]any{"type": "string", "description": "可选：若这技能本质是一段可复用代码，把入口脚本写在这里——结晶后可用 run_skill 直接运行，无需每次重抄"},
				"script_lang":   map[string]any{"type": "string", "description": "script 的语言：python / node / shell（填了 script 才需要）"},
			},
			"required": []string{"seed_id", "name", "instructions"},
		},
		Lanes: []tools.Lane{tools.LaneDeliberative},
		Handler: func(_ context.Context, _ tools.Context, argsJSON string) (string, error) {
			var a struct {
				SeedID       int64    `json:"seed_id"`
				Name         string   `json:"name"`
				Description  string   `json:"description"`
				Instructions string   `json:"instructions"`
				AllowedTools []string `json:"allowed_tools"`
				Script       string   `json:"script"`
				ScriptLang   string   `json:"script_lang"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			if a.SeedID <= 0 || a.Instructions == "" {
				return errJSON("need seed_id + instructions"), nil
			}
			var ep *skill.Entrypoint
			if strings.TrimSpace(a.Script) != "" {
				ep = &skill.Entrypoint{Lang: a.ScriptLang, Code: a.Script}
			}
			inst, err := skill.AuthorFromKnowledge(a.SeedID, a.Name, a.Description, a.Instructions, a.AllowedTools, ep)
			if err != nil {
				return errJSON(err.Error()), err
			}
			return mustJSON(map[string]any{"ok": true, "skill": inst.Name, "status": inst.Status, "id": inst.ID}), nil
		},
	}
}

// taskSkillInitialMastery sediment_skill 沉淀的技能初始掌握度（req-3）：来源是"这次真把活干成了"而非反复
// 探索累积，故给中等先验（做成过一次=可用但未深证），之后按自己复用的真成败 + R82 衰减自然校准。
const taskSkillInitialMastery = 0.6

// toolSedimentSkill seedless 结晶（req-3 2026-06-12）：把刚做成的、真正实用可复用的 procedure 沉淀成技能。
// 与 crystallize_skill 的区别：不需要 interest_seed——专给"人类交办的任务 / 临时目标里摸索出的可复用做法"，
// 让生命体把人机协作中的实战经验自主固化成日后能 use_skill/run_skill 复用、乃至 publish 分享的技能。
func toolSedimentSkill() tools.Tool {
	return tools.Tool{
		Name: "sediment_skill",
		Description: "把你这次（尤其是人类交办的任务里）摸索出的、**真正实用且可复用**的做法，固化成一个你自己的技能（SKILL.md）。" +
			"无需 interest_seed——专给临时任务中沉淀实战经验用。用自己的话写清这技能干什么、步骤要点；本质是代码就附 script。" +
			"只在你真把某件事做成了、且这套做法以后还用得上时才调；一次性/纯聊天别沉淀。沉淀后可 use_skill/run_skill 复用，也能 social.publish_skill 分享。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":          map[string]any{"type": "string", "description": "技能名（简短，英文/拼音 kebab-case 更佳）"},
				"description":   map[string]any{"type": "string", "description": "一句话：这技能干什么、何时用"},
				"instructions":  map[string]any{"type": "string", "description": "技能正文：用自己的话写清可复用的步骤 / 要点 / 注意事项"},
				"allowed_tools": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "这技能会用到的工具名（如 web.fetch / script.python）"},
				"script":        map[string]any{"type": "string", "description": "可选：若这技能本质是一段可复用代码，把入口脚本写在这里——之后可 run_skill 直接运行"},
				"script_lang":   map[string]any{"type": "string", "description": "script 的语言：python / node / shell（填了 script 才需要）"},
			},
			"required": []string{"name", "instructions"},
		},
		Lanes: []tools.Lane{tools.LaneDeliberative},
		Handler: func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
			var a struct {
				Name         string   `json:"name"`
				Description  string   `json:"description"`
				Instructions string   `json:"instructions"`
				AllowedTools []string `json:"allowed_tools"`
				Script       string   `json:"script"`
				ScriptLang   string   `json:"script_lang"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			if strings.TrimSpace(a.Name) == "" || strings.TrimSpace(a.Instructions) == "" {
				return errJSON("need name + instructions"), nil
			}
			var ep *skill.Entrypoint
			if strings.TrimSpace(a.Script) != "" {
				ep = &skill.Entrypoint{Lang: a.ScriptLang, Code: a.Script}
			}
			authoredFrom := "task"
			if tctx.GoalID > 0 {
				authoredFrom = fmt.Sprintf("task:goal#%d", tctx.GoalID)
			}
			inst, err := skill.AuthorFromTask(a.Name, a.Description, a.Instructions, a.AllowedTools, ep, authoredFrom, taskSkillInitialMastery)
			if err != nil {
				return errJSON(err.Error()), err
			}
			return mustJSON(map[string]any{"ok": true, "skill": inst.Name, "status": inst.Status, "id": inst.ID}), nil
		},
	}
}

// -----------------------------------------------------------------------------
// deliberative · 目标管理（递归研究目标树）
// -----------------------------------------------------------------------------

// 递归研究目标树护栏（防失控拆解 / token 燃尽）：
const (
	// MaxResearchDepth 子目标最大递归深度（根=0）。到顶不再允许拆子目标，逼 LLM 当层完成。
	MaxResearchDepth = 3
	// MaxSubgoalsPerParent 单个母目标可拥有的子目标数上限（一次拆解别开太多坑）。
	MaxSubgoalsPerParent = 5
)

func toolEnqueueSubgoal() tools.Tool {
	return tools.Tool{
		Name: "enqueue_subgoal",
		Description: "把当前目标拆出一个子目标（递归研究树）。子目标会先于母目标被执行；它们全部完成后，" +
			"母目标会自动恢复执行、综合子成果形成结论。用于「大问题先拆成几个可独立研究的小问题」。" +
			"intent 取 knowledge / social / creativity / stability / achievement。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"intent":   map[string]any{"type": "string"},
				"payload":  map[string]any{"type": "string", "description": "子目标描述 / 要研究的小问题"},
				"priority": map[string]any{"type": "number", "description": "优先级 0-1（默认 0.6）"},
			},
			"required": []string{"intent", "payload"},
		},
		Lanes:   []tools.Lane{tools.LaneDeliberative},
		Handler: handleEnqueueSubgoal,
	}
}

// handleEnqueueSubgoal 在当前执行中的目标（tctx.GoalID）下挂一个子目标。
//
// pending_children 计数不变量：本函数是该计数的唯一增点——成功入队子目标后母 +1。
// 配对的减点是 storage.MarkGoal（子转终态时母 -1）。母 pending_children>0 即「被阻塞」。
//
// 护栏（任一不过则拒绝，返回提示让 LLM 当层自己完成而非继续拆）：
//   - 深度：parent.Depth+1 不得超过 MaxResearchDepth。
//   - 单母子目标数：母现有子目标数 < MaxSubgoalsPerParent。
func handleEnqueueSubgoal(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a struct {
		Intent   string  `json:"intent"`
		Payload  string  `json:"payload"`
		Priority float64 `json:"priority"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return errJSON("invalid args"), err
	}
	if a.Intent == "" || a.Payload == "" {
		return errJSON("empty intent or payload"), nil
	}
	if a.Priority <= 0 || a.Priority > 1 {
		a.Priority = 0.6
	}
	if tctx.GoalID == 0 {
		return errJSON("no parent goal in context"), nil
	}

	parent, err := storage.GetGoalByID(tctx.GoalID)
	if err != nil {
		return errJSON("load parent failed"), err
	}
	if parent == nil {
		return errJSON("parent goal not found"), nil
	}

	childDepth := parent.Depth + 1
	if childDepth > MaxResearchDepth {
		// 到深度顶：不再拆，逼 LLM 当层自己完成（返回明确提示而非报错，让 LLM 自适应）。
		return mustJSON(map[string]any{
			"ok": false, "rejected": "max_depth",
			"hint": fmt.Sprintf("已到最大研究深度 %d，别再拆子目标了——请在本层直接把它研究完、调 complete_goal。", MaxResearchDepth),
		}), nil
	}

	children, err := storage.ListChildren(tctx.GoalID)
	if err != nil {
		return errJSON("count children failed"), err
	}
	if len(children) >= MaxSubgoalsPerParent {
		return mustJSON(map[string]any{
			"ok": false, "rejected": "max_subgoals",
			"hint": fmt.Sprintf("本目标已有 %d 个子目标（上限 %d），别再开新坑——先把已拆的做完。", len(children), MaxSubgoalsPerParent),
		}), nil
	}

	g := &core.Goal{
		Source:          core.GoalIntrinsic,
		Intent:          a.Intent,
		Payload:         a.Payload,
		Priority:        a.Priority,
		Status:          core.GoalPending,
		CreatedAt:       shared.SystemClock.UnixSec(),
		ArbitrationNote: fmt.Sprintf("subgoal_of=%d", tctx.GoalID),
		ParentID:        tctx.GoalID,
		Depth:           childDepth,
	}
	id, err := storage.EnqueueGoal(tctx.LifeID, g)
	if err != nil {
		return errJSON("enqueue failed"), err
	}
	// 母 pending_children +1 → 母目标进入「被阻塞」（pending 且 children>0）。
	if err := storage.IncPendingChildren(tctx.GoalID); err != nil {
		return errJSON("inc pending children failed"), err
	}
	return mustJSON(map[string]any{"ok": true, "goal_id": id, "depth": childDepth, "parent_id": tctx.GoalID}), nil
}

func toolCompleteGoal() tools.Tool {
	return tools.Tool{
		Name:        "complete_goal",
		Description: "标记当前 goal（或指定 goal_id）为已完成 / 失败。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"goal_id": map[string]any{"type": "integer", "description": "默认当前上下文 goal_id"},
				"success": map[string]any{"type": "boolean"},
			},
			"required": []string{"success"},
		},
		Lanes:      []tools.Lane{tools.LaneDeliberative},
		AlwaysLoad: true, // 核心：收尾每个 goal 必需
		Handler:    handleCompleteGoal,
	}
}

func handleCompleteGoal(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
	var a struct {
		GoalID  int64 `json:"goal_id"`
		Success bool  `json:"success"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return errJSON("invalid args"), err
	}
	id := a.GoalID
	if id == 0 {
		id = tctx.GoalID
	}
	if id == 0 {
		return errJSON("no goal_id"), nil
	}
	status := core.GoalCompleted
	if !a.Success {
		status = core.GoalFailed
	}
	if err := storage.MarkGoal(id, status, shared.SystemClock.UnixSec()); err != nil {
		return errJSON("mark failed"), err
	}
	return mustJSON(map[string]any{"ok": true, "goal_id": id, "status": string(status)}), nil
}

// -----------------------------------------------------------------------------
// deliberative · toolrunner 桥
// -----------------------------------------------------------------------------

func toolFsRead() tools.Tool {
	return tools.Tool{
		Name:        "fs.read",
		Description: "读 sandbox 内文件（路径相对 /sandbox/）。",
		Parameters:  pathParam("文件路径（相对 /sandbox/）"),
		Lanes:       []tools.Lane{tools.LaneDeliberative},
		AlwaysLoad:  true, // 核心：sandbox 基础读
		Handler:     wrapPath(func(cycleID int64, path string) (toolrunner.Result, error) { return toolrunner.FsRead(cycleID, path) }),
	}
}

func toolFsWrite() tools.Tool {
	return tools.Tool{
		Name:        "fs.write",
		Description: "写 sandbox 内文件（路径相对 /sandbox/，禁绝对路径）。覆盖；自动建父目录。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "文件路径（相对 /sandbox/，如 drafts/poem.txt；父目录自动创建。勿用绝对路径或 / 开头）"},
				"content": map[string]any{"type": "string", "description": "文件内容"},
			},
			"required": []string{"path", "content"},
		},
		Lanes:      []tools.Lane{tools.LaneDeliberative},
		AlwaysLoad: true, // 核心：sandbox 基础写（存稿/作品/降级社交稿）
		Handler: func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
			var a struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			r, err := toolrunner.FsWrite(tctx.CycleID, a.Path, a.Content)
			return wrapRunnerResult(r, err)
		},
	}
}

func toolFsList() tools.Tool {
	return tools.Tool{
		Name:        "fs.list",
		Description: "列 sandbox 内目录条目。",
		Parameters:  pathParam("目录路径（相对 /sandbox/）"),
		Lanes:       []tools.Lane{tools.LaneDeliberative},
		AlwaysLoad:  true, // 核心：sandbox 基础列目录
		Handler:     wrapPath(func(cycleID int64, path string) (toolrunner.Result, error) { return toolrunner.FsList(cycleID, path) }),
	}
}

func toolFsMkdir() tools.Tool {
	return tools.Tool{
		Name:        "fs.mkdir",
		Description: "在 sandbox 内创建目录（递归）。",
		Parameters:  pathParam("目录路径（相对 /sandbox/）"),
		Lanes:       []tools.Lane{tools.LaneDeliberative},
		AlwaysLoad:  true, // 核心：sandbox 基础建目录
		Handler:     wrapPath(func(cycleID int64, path string) (toolrunner.Result, error) { return toolrunner.FsMkdir(cycleID, path) }),
	}
}

// http.get / http.post 已撤（速胜#3）：旧实现只回状态码 + 字节数（desc 自承「web.fetch 替代」），
// 在慎思 agent loop 里白耗轮次。读网页正文统一走 web.fetch；要调 JSON API 在 script.python/node 里发请求。
// toolrunner.HTTPGet/HTTPPost 桥保留（其他 lane / 未来可复用），仅不再注册进慎思 lane。

func toolTimeNow() tools.Tool {
	return tools.Tool{
		Name:        "time.now",
		Description: "返回当前 unix 时间戳（秒）。",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Lanes:       []tools.Lane{tools.LaneDeliberative},
		Handler: func(_ context.Context, tctx tools.Context, _ string) (string, error) {
			r, err := toolrunner.TimeNow(tctx.CycleID)
			return wrapRunnerResult(r, err)
		},
	}
}

func toolWebSearch() tools.Tool {
	return tools.Tool{
		Name: "web.search",
		Description: "搜索引擎查询（真浏览器跑，无需 key），返回结果列表（标题/URL/摘要）。" +
			"了解新事物 / 找资料 / 求证时**先用它搜**（可换多个关键词多搜几次），" +
			"再据标题摘要判断哪些来源靠谱、用 web.fetch 进去读——别凭记忆直接猜 URL。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "搜索关键词"},
			},
			"required": []string{"query"},
		},
		Lanes: []tools.Lane{tools.LaneDeliberative},
		Handler: func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
			var a struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			if a.Query == "" {
				return errJSON("empty query"), nil
			}
			r, err := toolrunner.WebSearch(tctx.CycleID, a.Query)
			return wrapRunnerResult(r, err)
		},
	}
}

func toolWebFetch() tools.Tool {
	return tools.Tool{
		Name: "web.fetch",
		Description: "抓取网页并提取正文（自动剥导航/广告/页脚），返回 markdown。" +
			"读文章 / 博客 / 文档站首选此工具，不要用 http.get（后者只给状态码）。" +
			"静态页与 SPA 都支持（内部自动升级到无头浏览器渲染）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string"},
			},
			"required": []string{"url"},
		},
		Lanes: []tools.Lane{tools.LaneDeliberative},
		Handler: func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
			var a struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			if a.URL == "" {
				return errJSON("empty url"), nil
			}
			r, err := toolrunner.WebFetch(tctx.CycleID, a.URL)
			return wrapRunnerResult(r, err)
		},
	}
}

func toolWebRender() tools.Tool {
	return tools.Tool{
		Name: "web.render",
		Description: "强制用无头浏览器渲染页面（执行 JS）后提取正文 markdown。" +
			"仅当 web.fetch 返回内容明显不全（疑似重 JS 渲染站点）时才用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string"},
			},
			"required": []string{"url"},
		},
		Lanes: []tools.Lane{tools.LaneDeliberative},
		Handler: func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
			var a struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			if a.URL == "" {
				return errJSON("empty url"), nil
			}
			r, err := toolrunner.WebRender(tctx.CycleID, a.URL)
			return wrapRunnerResult(r, err)
		},
	}
}

func toolScriptPython() tools.Tool {
	return tools.Tool{
		Name: "script.python",
		Description: "在 sandbox 内执行 Python3 代码（python3 -c）。" +
			"可用包：httpx requests beautifulsoup4 lxml trafilatura pyyaml pillow markdown feedparser python-dateutil。" +
			"超时 60s，工作目录 /sandbox/。禁运行时 pip 装包。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{"type": "string", "description": "Python 源码"},
			},
			"required": []string{"code"},
		},
		Lanes: []tools.Lane{tools.LaneDeliberative},
		Handler: func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
			var a struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			if a.Code == "" {
				return errJSON("empty code"), nil
			}
			r, err := toolrunner.ScriptPython(tctx.CycleID, a.Code)
			return wrapRunnerResult(r, err)
		},
	}
}

func toolScriptNode() tools.Tool {
	return tools.Tool{
		Name: "script.node",
		Description: "在 sandbox 内执行 Node.js 代码（node -e）。" +
			"可用包：axios cheerio dayjs js-yaml marked。" +
			"超时 60s，工作目录 /sandbox/。禁运行时 npm 装包。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{"type": "string", "description": "JavaScript 源码"},
			},
			"required": []string{"code"},
		},
		Lanes: []tools.Lane{tools.LaneDeliberative},
		Handler: func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
			var a struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			if a.Code == "" {
				return errJSON("empty code"), nil
			}
			r, err := toolrunner.ScriptNode(tctx.CycleID, a.Code)
			return wrapRunnerResult(r, err)
		},
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func pathParam(desc string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": desc},
		},
		"required": []string{"path"},
	}
}

func wrapPath(fn func(cycleID int64, path string) (toolrunner.Result, error)) tools.Handler {
	return func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
		var a struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return errJSON("invalid args"), err
		}
		r, err := fn(tctx.CycleID, a.Path)
		return wrapRunnerResult(r, err)
	}
}

func wrapRunnerResult(r toolrunner.Result, err error) (string, error) {
	if err != nil {
		return mustJSON(map[string]any{"ok": false, "err": err.Error(), "output": r.Output}), err
	}
	return mustJSON(map[string]any{"ok": true, "output": r.Output, "duration_ms": r.DurationMs}), nil
}

func errJSON(msg string) string {
	return mustJSON(map[string]any{"ok": false, "err": msg})
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"ok":false,"err":"marshal failed: %s"}`, err.Error())
	}
	return string(b)
}
