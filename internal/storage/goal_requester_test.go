package storage

import (
	"path/filepath"
	"testing"

	"mindverse/internal/core"
)

// TestGoalRequesterRoundTrip 验证 migration 008 的请求者列存取往返：
// EnqueueGoal 写入 req_channel/req_from → NextPendingGoal 原样读回（拟人交互闭环任务 2）。
func TestGoalRequesterRoundTrip(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "g.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()

	const life = "life-goalreq"
	if err := InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}

	g := &core.Goal{
		Source:     core.GoalExternal,
		Intent:     "研究用户托付的请求",
		Payload:    "研究 Rust 异步运行时",
		Priority:   0.7,
		Status:     core.GoalPending,
		CreatedAt:  100,
		ReqChannel: "feishu",
		ReqFrom:    "ou_user_xyz",
	}
	id, err := EnqueueGoal(life, g)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if id <= 0 {
		t.Fatalf("bad id: %d", id)
	}

	got, err := NextPendingGoal(life, 200)
	if err != nil {
		t.Fatalf("next pending: %v", err)
	}
	if got.ReqChannel != "feishu" {
		t.Errorf("req_channel = %q, want feishu", got.ReqChannel)
	}
	if got.ReqFrom != "ou_user_xyz" {
		t.Errorf("req_from = %q, want ou_user_xyz", got.ReqFrom)
	}
	if got.Source != core.GoalExternal {
		t.Errorf("source = %q, want ExternalRequest", got.Source)
	}
}

// TestEnqueueExternalRequest 验证 defer_research 入队助手：带请求者入队 + 同主题去重。
func TestEnqueueExternalRequest(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "g2.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()

	const life = "life-ext"
	if err := InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}

	// 首次入队：created=true，带请求者。
	id1, created1, err := EnqueueExternalRequest(life, "研究 X 主题", "feishu", "ou_a", 0.7, 100)
	if err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	if !created1 || id1 <= 0 {
		t.Fatalf("first enqueue should create: created=%v id=%d", created1, id1)
	}

	// 同主题在飞 → 去重，不重复入队。
	_, created2, err := EnqueueExternalRequest(life, "研究 X 主题", "feishu", "ou_a", 0.7, 110)
	if err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}
	if created2 {
		t.Errorf("duplicate topic should not create a second goal")
	}

	// 空主题：不入队、不报错。
	_, created3, err := EnqueueExternalRequest(life, "   ", "feishu", "ou_a", 0.7, 120)
	if err != nil {
		t.Fatalf("enqueue empty: %v", err)
	}
	if created3 {
		t.Errorf("empty topic should not create a goal")
	}

	// 读回首条，确认请求者落库正确。
	got, err := NextPendingGoal(life, 200)
	if err != nil {
		t.Fatalf("next pending: %v", err)
	}
	if got.ReqFrom != "ou_a" || got.ReqChannel != "feishu" {
		t.Errorf("requester not persisted: channel=%q from=%q", got.ReqChannel, got.ReqFrom)
	}
}
