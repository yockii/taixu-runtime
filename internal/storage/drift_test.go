package storage

import (
	"math"
	"path/filepath"
	"testing"
)

// TestValueDriftStats 验 C8：漂移落表 + 聚合（净/绝对位移、方向一致度、生命隔离、时间窗）。
func TestValueDriftStats(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "d.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-c8"

	// growth：三次同向上调（+0.1,+0.1,+0.1）→ 净=绝对=0.3，方向一致度=1（有目的）。
	_ = InsertValueDrift(life, "growth", 0.50, 0.60, "deep_reflect", 100)
	_ = InsertValueDrift(life, "growth", 0.60, 0.70, "deep_reflect", 200)
	_ = InsertValueDrift(life, "growth", 0.70, 0.80, "deep_reflect", 300)
	// safety：来回抖（+0.2,-0.2）→ 净=0、绝对=0.4，方向一致度=0（随机游走）。
	_ = InsertValueDrift(life, "safety", 0.50, 0.70, "deep_reflect", 150)
	_ = InsertValueDrift(life, "safety", 0.70, 0.50, "deep_reflect", 250)
	// 别的生命隔离。
	_ = InsertValueDrift("other", "growth", 0.1, 0.9, "x", 100)

	drifts, err := ValueDriftSince(life, 0)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	m := map[string]ValueDrift{}
	for _, d := range drifts {
		m[d.Name] = d
	}
	if len(m) != 2 {
		t.Fatalf("应2个价值观(隔离 other), 得 %d", len(m))
	}

	g := m["growth"]
	if !approx(g.NetDelta, 0.3) || !approx(g.AbsDelta, 0.3) || g.Changes != 3 {
		t.Errorf("growth 应 net=abs=0.3 changes=3, 得 %+v", g)
	}
	if !approx(g.Purposefulness(), 1.0) {
		t.Errorf("growth 同向应方向一致度=1, 得 %.3f", g.Purposefulness())
	}

	s := m["safety"]
	if !approx(s.NetDelta, 0.0) || !approx(s.AbsDelta, 0.4) {
		t.Errorf("safety 应 net=0 abs=0.4, 得 %+v", s)
	}
	if !approx(s.Purposefulness(), 0.0) {
		t.Errorf("safety 来回抖应方向一致度=0(随机游走), 得 %.3f", s.Purposefulness())
	}

	// 时间窗：仅取 created_at>=250 → growth 1 次(300)、safety 1 次(250)。
	win, _ := ValueDriftSince(life, 250)
	wm := map[string]ValueDrift{}
	for _, d := range win {
		wm[d.Name] = d
	}
	if wm["growth"].Changes != 1 {
		t.Errorf("窗250后 growth 应1次, 得 %d", wm["growth"].Changes)
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
