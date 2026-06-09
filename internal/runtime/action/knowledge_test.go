package action

import (
	"path/filepath"
	"strings"
	"testing"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/storage"
)

// TestMaybeSedimentKnowledge 验证根研究目标完成 → 综合整棵子树成果 → 生成知识库 dossier。
// LLM 未配（测试不连真实 LLM），走朴素拼接兜底；断言 dossier 含根结论 + 子成果，且按根目标去重。
func TestMaybeSedimentKnowledge(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "kn.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = storage.Close() }()
	const life = "life-kn"
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	lifeID = life // 包内直接置（与 sediment_test 同手法）

	// 根知识研究目标 + 一个子目标，各有成果摘要。
	root := &core.Goal{Source: core.GoalExternal, Intent: "knowledge", Payload: "研究 X 主题",
		Status: core.GoalCompleted, CreatedAt: 1}
	rid, _ := storage.EnqueueGoal(life, root)
	_ = storage.SetResultDigest(rid, "X 主题的综合结论：A 因 B。")
	root.ID = rid
	root.ResultDigest = "X 主题的综合结论：A 因 B。"

	child := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "子问题 A",
		Status: core.GoalCompleted, CreatedAt: 2, ParentID: rid, Depth: 1}
	cid, _ := storage.EnqueueGoal(life, child)
	_ = storage.SetResultDigest(cid, "子问题 A 的发现：C。")

	// 触发沉淀（LLM 未配 → 朴素拼接）。
	maybeSedimentKnowledge(root, 1000)

	list, err := storage.ListKnowledge(life, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 dossier, got %d", len(list))
	}
	full, _ := storage.GetKnowledge(life, list[0].ID)
	if full.RootGoalID != rid {
		t.Errorf("dossier root_goal_id = %d, want %d", full.RootGoalID, rid)
	}
	if !strings.Contains(full.Body, "综合结论") {
		t.Errorf("dossier missing root conclusion: %q", full.Body)
	}
	if !strings.Contains(full.Body, "子问题 A 的发现") {
		t.Errorf("dossier missing subgoal finding: %q", full.Body)
	}

	// 幂等：再触发一次不应重复入库（同根目标去重）。
	maybeSedimentKnowledge(root, 1001)
	list, _ = storage.ListKnowledge(life, 10, 0)
	if len(list) != 1 {
		t.Errorf("dossier should not be duplicated, got %d", len(list))
	}
}

// TestMaybeSedimentKnowledgeSkipsNonRoot 非根目标（有 parent）不单独生成 dossier。
func TestMaybeSedimentKnowledgeSkipsNonRoot(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "kn2.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = storage.Close() }()
	const life = "life-kn2"
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	lifeID = life

	child := &core.Goal{ID: 7, Source: core.GoalIntrinsic, Intent: "knowledge",
		Payload: "子", Status: core.GoalCompleted, ParentID: 1, Depth: 1, ResultDigest: "x"}
	maybeSedimentKnowledge(child, 100)
	list, _ := storage.ListKnowledge(life, 10, 0)
	if len(list) != 0 {
		t.Errorf("non-root goal must not create dossier, got %d", len(list))
	}
}
