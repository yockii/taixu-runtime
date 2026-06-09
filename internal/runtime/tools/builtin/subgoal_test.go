package builtin

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/runtime/tools"
	"taixu.icu/runtime/internal/storage"
)

func initStore(t *testing.T, life string) {
	t.Helper()
	if err := storage.Init(filepath.Join(t.TempDir(), "b.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
}

func mustParent(t *testing.T, life string, depth int) int64 {
	t.Helper()
	g := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "母", Status: core.GoalPending, CreatedAt: 1, Depth: depth}
	id, err := storage.EnqueueGoal(life, g)
	if err != nil {
		t.Fatalf("enqueue parent: %v", err)
	}
	return id
}

// TestEnqueueSubgoalParentLinkage 验证 enqueue_subgoal 工具：建真父子（parent_id/depth）+ 母 pending_children +1。
func TestEnqueueSubgoalParentLinkage(t *testing.T) {
	const life = "life-sg"
	initStore(t, life)
	pid := mustParent(t, life, 0)

	tctx := tools.Context{LifeID: life, GoalID: pid}
	out, err := handleEnqueueSubgoal(context.Background(), tctx, `{"intent":"knowledge","payload":"子问题"}`)
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	var r struct {
		OK     bool  `json:"ok"`
		GoalID int64 `json:"goal_id"`
		Depth  int   `json:"depth"`
		Parent int64 `json:"parent_id"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, out)
	}
	if !r.OK || r.GoalID == 0 {
		t.Fatalf("subgoal not created: %s", out)
	}
	if r.Parent != pid || r.Depth != 1 {
		t.Errorf("linkage wrong: parent=%d depth=%d (want %d/1)", r.Parent, r.Depth, pid)
	}

	child, _ := storage.GetGoalByID(r.GoalID)
	if child.ParentID != pid || child.Depth != 1 {
		t.Errorf("child row: parent=%d depth=%d", child.ParentID, child.Depth)
	}
	parent, _ := storage.GetGoalByID(pid)
	if parent.PendingChildren != 1 {
		t.Errorf("parent pending_children = %d, want 1", parent.PendingChildren)
	}
}

// TestEnqueueSubgoalDepthGuard 深度护栏：母 depth 已到 MaxResearchDepth 时拒绝建子目标（不增计数）。
func TestEnqueueSubgoalDepthGuard(t *testing.T) {
	const life = "life-depth"
	initStore(t, life)
	pid := mustParent(t, life, MaxResearchDepth) // 子将是 depth=MaxResearchDepth+1，超限

	tctx := tools.Context{LifeID: life, GoalID: pid}
	out, _ := handleEnqueueSubgoal(context.Background(), tctx, `{"intent":"knowledge","payload":"太深"}`)
	var r struct {
		OK       bool   `json:"ok"`
		Rejected string `json:"rejected"`
	}
	_ = json.Unmarshal([]byte(out), &r)
	if r.OK || r.Rejected != "max_depth" {
		t.Fatalf("should reject for max_depth, got %s", out)
	}
	parent, _ := storage.GetGoalByID(pid)
	if parent.PendingChildren != 0 {
		t.Errorf("rejected subgoal must not inc parent children, got %d", parent.PendingChildren)
	}
}

// TestEnqueueSubgoalCountGuard 单母子目标数护栏：达 MaxSubgoalsPerParent 后拒绝再建。
func TestEnqueueSubgoalCountGuard(t *testing.T) {
	const life = "life-count"
	initStore(t, life)
	pid := mustParent(t, life, 0)
	tctx := tools.Context{LifeID: life, GoalID: pid}

	for i := 0; i < MaxSubgoalsPerParent; i++ {
		out, err := handleEnqueueSubgoal(context.Background(), tctx, `{"intent":"knowledge","payload":"子"}`)
		if err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		var r struct {
			OK bool `json:"ok"`
		}
		_ = json.Unmarshal([]byte(out), &r)
		if !r.OK {
			t.Fatalf("subgoal %d should succeed: %s", i, out)
		}
	}
	// 第 MaxSubgoalsPerParent+1 个：拒绝。
	out, _ := handleEnqueueSubgoal(context.Background(), tctx, `{"intent":"knowledge","payload":"超额"}`)
	var r struct {
		OK       bool   `json:"ok"`
		Rejected string `json:"rejected"`
	}
	_ = json.Unmarshal([]byte(out), &r)
	if r.OK || r.Rejected != "max_subgoals" {
		t.Fatalf("should reject for max_subgoals, got %s", out)
	}
	parent, _ := storage.GetGoalByID(pid)
	if parent.PendingChildren != MaxSubgoalsPerParent {
		t.Errorf("parent children = %d, want %d (rejection must not inc)", parent.PendingChildren, MaxSubgoalsPerParent)
	}
}
