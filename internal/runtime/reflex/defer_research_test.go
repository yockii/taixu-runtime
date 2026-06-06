package reflex

import (
	"context"
	"path/filepath"
	"testing"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/runtime/tools"
	"mindverse/internal/storage"
)

// TestHandlerDeferResearch 验证 defer_research 工具（拟人交互闭环任务 2）：
//   - 入队 source='ExternalRequest' 目标并带上请求者（tctx.Channel/From）
//   - 引擎替生命体发布一条拟人确认 ReplyEvent（经 bus → 飞书/SSE）
//   - 同主题去重：第二次相同 topic 不重复入队，但仍回确认
func TestHandlerDeferResearch(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "dr.db")); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	const life = "life-defer"
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	if err := memory.Init(life); err != nil {
		t.Fatalf("memory init: %v", err)
	}
	// 包内直接置（避开 reflex.Init 注册 tool 的全局副作用）；rng 供 pickDeferAck。
	lifeID = life
	rng = seededRNG()

	bus.Reset()
	defer bus.Reset()
	var acks []ReplyEvent
	bus.Subscribe(ReplyEvent{}, func(e bus.Event) { acks = append(acks, e.(ReplyEvent)) })

	tctx := tools.Context{LifeID: life, Channel: "feishu", From: "ou_boss"}

	// 首次：入队 + 回确认。
	out, err := handlerDeferResearch(context.Background(), tctx, `{"topic":"研究下一代记忆压缩算法","why":"用户在意"}`)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if out == "" {
		t.Fatalf("empty handler result")
	}

	// 验证目标入队 + 请求者落库。
	g, err := storage.NextPendingGoal(life, 500)
	if err != nil {
		t.Fatalf("next pending: %v", err)
	}
	if g.Source != core.GoalExternal {
		t.Errorf("source = %q want ExternalRequest", g.Source)
	}
	if g.ReqChannel != "feishu" || g.ReqFrom != "ou_boss" {
		t.Errorf("requester not tracked: channel=%q from=%q", g.ReqChannel, g.ReqFrom)
	}

	// 验证发布了一条拟人确认（路由到请求者）。
	if len(acks) != 1 {
		t.Fatalf("want 1 ack ReplyEvent, got %d", len(acks))
	}
	if acks[0].To != "ou_boss" || acks[0].Channel != "feishu" {
		t.Errorf("ack routed wrong: channel=%q to=%q", acks[0].Channel, acks[0].To)
	}
	if acks[0].Content == "" {
		t.Errorf("ack content empty")
	}
}

// TestHandlerDeferResearch_Dedup 同主题在飞时不重复入队（NextPendingGoal 已把首条置 active，
// HasOpenGoalWithPayloadSubstring 仍判其 active → 第二次相同主题去重）。
func TestHandlerDeferResearch_Dedup(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "dr2.db")); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	const life = "life-defer2"
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	if err := memory.Init(life); err != nil {
		t.Fatalf("memory init: %v", err)
	}
	lifeID = life
	rng = seededRNG()
	bus.Reset()
	defer bus.Reset()

	tctx := tools.Context{LifeID: life, Channel: "web", From: "u1"}
	if _, err := handlerDeferResearch(context.Background(), tctx, `{"topic":"同一个主题 AAA"}`); err != nil {
		t.Fatalf("handler 1: %v", err)
	}
	if _, err := handlerDeferResearch(context.Background(), tctx, `{"topic":"同一个主题 AAA"}`); err != nil {
		t.Fatalf("handler 2: %v", err)
	}

	// 只应有一条 pending/active 目标。
	n, err := storage.CountActiveOrPendingGoals(life)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("dedup failed: want 1 open goal, got %d", n)
	}
}
