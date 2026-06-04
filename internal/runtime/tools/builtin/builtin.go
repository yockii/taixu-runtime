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

	"mindverse/internal/core"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/runtime/tools"
	"mindverse/internal/shared"
	"mindverse/internal/skill/toolrunner"
	"mindverse/internal/storage"
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
		// --- deliberative · 记忆 / 反思 / 兴趣 ---
		toolQueryMemory(),
		toolSealEpisode(),
		toolExploreInterestSeed(),
		// --- deliberative · 目标管理 ---
		toolEnqueueSubgoal(),
		toolCompleteGoal(),
		// --- deliberative · 沙箱 IO 桥（toolrunner）---
		toolFsRead(),
		toolFsWrite(),
		toolFsList(),
		toolFsMkdir(),
		toolHTTPGet(),
		toolHTTPPost(),
		toolTimeNow(),
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

func handleQueryMemory(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
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

func toolExploreInterestSeed() tools.Tool {
	return tools.Tool{
		Name:        "explore_interest_seed",
		Description: "标记一个兴趣种子已被探索过一次（推进 explored_count）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"seed_id": map[string]any{"type": "integer"},
			},
			"required": []string{"seed_id"},
		},
		Lanes:   []tools.Lane{tools.LaneDeliberative},
		Handler: handleExploreInterestSeed,
	}
}

func handleExploreInterestSeed(_ context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		SeedID int64 `json:"seed_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return errJSON("invalid args"), err
	}
	if a.SeedID <= 0 {
		return errJSON("invalid seed_id"), nil
	}
	if err := storage.BumpInterestExplored(a.SeedID, shared.SystemClock.UnixSec()); err != nil {
		return errJSON("bump failed"), err
	}
	return `{"ok":true}`, nil
}

// -----------------------------------------------------------------------------
// deliberative · 目标管理
// -----------------------------------------------------------------------------

func toolEnqueueSubgoal() tools.Tool {
	return tools.Tool{
		Name:        "enqueue_subgoal",
		Description: "入队一个子目标供后续 cycle 处理。intent 取 knowledge / social / creativity / stability / achievement。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"intent":   map[string]any{"type": "string"},
				"payload":  map[string]any{"type": "string", "description": "目标描述 / 起因"},
				"priority": map[string]any{"type": "number", "description": "优先级 0-1（默认 0.6）"},
			},
			"required": []string{"intent", "payload"},
		},
		Lanes:   []tools.Lane{tools.LaneDeliberative},
		Handler: handleEnqueueSubgoal,
	}
}

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
	g := &core.Goal{
		Source:          core.GoalIntrinsic,
		Intent:          a.Intent,
		Payload:         a.Payload,
		Priority:        a.Priority,
		Status:          core.GoalPending,
		CreatedAt:       shared.SystemClock.UnixSec(),
		ArbitrationNote: fmt.Sprintf("subgoal_of=%d", tctx.GoalID),
	}
	id, err := storage.EnqueueGoal(tctx.LifeID, g)
	if err != nil {
		return errJSON("enqueue failed"), err
	}
	return mustJSON(map[string]any{"ok": true, "goal_id": id}), nil
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
		Lanes:   []tools.Lane{tools.LaneDeliberative},
		Handler: handleCompleteGoal,
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
		Handler:     wrapPath(func(cycleID int64, path string) (toolrunner.Result, error) { return toolrunner.FsRead(cycleID, path) }),
	}
}

func toolFsWrite() tools.Tool {
	return tools.Tool{
		Name:        "fs.write",
		Description: "写 sandbox 内文件。覆盖；自动建目录。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			},
			"required": []string{"path", "content"},
		},
		Lanes: []tools.Lane{tools.LaneDeliberative},
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
		Handler:     wrapPath(func(cycleID int64, path string) (toolrunner.Result, error) { return toolrunner.FsList(cycleID, path) }),
	}
}

func toolFsMkdir() tools.Tool {
	return tools.Tool{
		Name:        "fs.mkdir",
		Description: "在 sandbox 内创建目录（递归）。",
		Parameters:  pathParam("目录路径（相对 /sandbox/）"),
		Lanes:       []tools.Lane{tools.LaneDeliberative},
		Handler:     wrapPath(func(cycleID int64, path string) (toolrunner.Result, error) { return toolrunner.FsMkdir(cycleID, path) }),
	}
}

func toolHTTPGet() tools.Tool {
	return tools.Tool{
		Name:        "http.get",
		Description: "HTTP GET 请求。返回状态码与 body 字节数（Phase 0；后续 web.fetch 替代）。",
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
			r, err := toolrunner.HTTPGet(tctx.CycleID, a.URL)
			return wrapRunnerResult(r, err)
		},
	}
}

func toolHTTPPost() tools.Tool {
	return tools.Tool{
		Name:        "http.post",
		Description: "HTTP POST 请求。body 以 application/json 发出。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":  map[string]any{"type": "string"},
				"body": map[string]any{"type": "string"},
			},
			"required": []string{"url", "body"},
		},
		Lanes: []tools.Lane{tools.LaneDeliberative},
		Handler: func(_ context.Context, tctx tools.Context, argsJSON string) (string, error) {
			var a struct {
				URL  string `json:"url"`
				Body string `json:"body"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
				return errJSON("invalid args"), err
			}
			if a.URL == "" {
				return errJSON("empty url"), nil
			}
			r, err := toolrunner.HTTPPost(tctx.CycleID, a.URL, a.Body)
			return wrapRunnerResult(r, err)
		},
	}
}

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
