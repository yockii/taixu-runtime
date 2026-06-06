package action

import (
	"path/filepath"
	"testing"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/storage"
)

// TestMaybeReportToRequester 验证完成后主动汇报路径（拟人交互闭环任务 3）：
//   - 带请求者的已完成 ExternalRequest 目标 → 发布 bus.ResearchReported（路由到飞书等 io 层）
//   - LLM 未配时退化为朴素模板，仍能汇报（不连真实飞书）
//   - 防重复：同一目标只汇报一次（meta 标志）
//
// 绝不连真实飞书：这里只断言 bus 事件被发布，io 层（lark.Send）由 main 订阅、不在测试范围。
func TestMaybeReportToRequester(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "rep.db")); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	const life = "life-report"
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	if err := memory.Init(life); err != nil {
		t.Fatalf("memory init: %v", err)
	}
	lifeID = life // 包内直接置，避开 action.Init 副作用（同 sediment_test）

	// 捕获 bus 事件（不接真实飞书）。
	bus.Reset()
	defer bus.Reset()
	var reports []bus.ResearchReported
	bus.Subscribe(bus.ResearchReported{}, func(e bus.Event) {
		reports = append(reports, e.(bus.ResearchReported))
	})

	g := &core.Goal{
		ID:         42,
		Source:     core.GoalExternal,
		Payload:    "研究下午睡的最佳时长",
		ReqChannel: "feishu",
		ReqFrom:    "ou_requester",
	}
	res := Result{Output: "20 分钟左右最不易进入深睡、醒来不昏沉。", Success: true}

	// 首次：应发布一条汇报。
	maybeReportToRequester(g, res, 1000)
	if len(reports) != 1 {
		t.Fatalf("want 1 report published, got %d", len(reports))
	}
	r := reports[0]
	if r.To != "ou_requester" || r.Channel != "feishu" {
		t.Errorf("report routed wrong: channel=%q to=%q", r.Channel, r.To)
	}
	if r.GoalID != 42 {
		t.Errorf("report goal id = %d, want 42", r.GoalID)
	}
	if r.Content == "" {
		t.Errorf("report content empty")
	}

	// 防重复：再调一次，不应再发（meta 标志已落）。
	maybeReportToRequester(g, res, 1001)
	if len(reports) != 1 {
		t.Errorf("duplicate report should be suppressed, got %d total", len(reports))
	}
}

// TestMaybeReportToRequester_NoRequester 内驱目标（无请求者）不汇报。
func TestMaybeReportToRequester_NoRequester(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "rep2.db")); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	const life = "life-report2"
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	if err := memory.Init(life); err != nil {
		t.Fatalf("memory init: %v", err)
	}
	lifeID = life

	bus.Reset()
	defer bus.Reset()
	var n int
	bus.Subscribe(bus.ResearchReported{}, func(e bus.Event) { n++ })

	// 内驱目标：source=IntrinsicDrive，无 ReqFrom。
	g := &core.Goal{ID: 7, Source: core.GoalIntrinsic, Payload: "interest_seed#3 学点天文"}
	maybeReportToRequester(g, Result{Output: "看了些资料", Success: true}, 1000)
	if n != 0 {
		t.Errorf("intrinsic goal should not report, got %d", n)
	}

	// 带请求者但 source 非 External（理论上不会出现）也不发。
	g2 := &core.Goal{ID: 8, Source: core.GoalIntrinsic, ReqFrom: "ou_x", Payload: "x"}
	maybeReportToRequester(g2, Result{Output: "y", Success: true}, 1000)
	if n != 0 {
		t.Errorf("non-external goal should not report, got %d", n)
	}
}
