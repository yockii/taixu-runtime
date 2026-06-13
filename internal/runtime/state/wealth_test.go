package state

import (
	"math"
	"path/filepath"
	"testing"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/storage"
)

// TestWealthCache 验 2026-06-12 平台权威化：life.Wealth 降级为平台余额显示缓存，
// SetWealthCache 绝对写入（微财富→wealth）、非负 clamp、持久化、不被 [0,1] clamp（可超 1）。
func TestWealthCache(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "w.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = storage.Close() }()
	const life = "life-c10"
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("seed genome: %v", err)
	}
	if err := storage.UpsertLifeState(&core.LifeState{LifeID: life, Energy: 1, EnergyDailyCap: 1}); err != nil {
		t.Fatalf("seed life_state: %v", err)
	}
	if err := storage.UpsertMentalState(&core.MentalState{LifeID: life}); err != nil {
		t.Fatalf("seed mental: %v", err)
	}
	if err := Init(life); err != nil {
		t.Fatalf("state init: %v", err)
	}

	// 平台余额 3_500_000 micro = 3.5 wealth（>1，绝不被 clamp——wealth 非 [0,1] 标量）。
	if err := SetWealthCache(3_500_000); err != nil {
		t.Fatalf("set cache: %v", err)
	}
	if s, _ := Snapshot(); !approx(s.Wealth, 3.5) {
		t.Fatalf("缓存应 3.5(超1不clamp), 得 %.4f", s.Wealth)
	}

	// 绝对写入（非增量）：再设较小值 → 直接覆盖为 1.0，与平台对齐不漂移。
	if err := SetWealthCache(1_000_000); err != nil {
		t.Fatalf("set cache 2: %v", err)
	}
	if s, _ := Snapshot(); !approx(s.Wealth, 1.0) {
		t.Fatalf("绝对写入应覆盖为 1.0, 得 %.4f", s.Wealth)
	}

	// 负值 clamp 到 0。
	if err := SetWealthCache(-5); err != nil {
		t.Fatalf("set neg: %v", err)
	}
	if s, _ := Snapshot(); s.Wealth != 0 {
		t.Fatalf("负值应 clamp 0, 得 %.4f", s.Wealth)
	}

	// 持久化：reload 应见缓存值。
	if err := SetWealthCache(2_000_000); err != nil {
		t.Fatalf("set cache 3: %v", err)
	}
	reloaded, _ := storage.LoadLifeState(life)
	if !approx(reloaded.Wealth, 2.0) {
		t.Fatalf("持久化缓存应 2.0, 得 %.4f", reloaded.Wealth)
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
