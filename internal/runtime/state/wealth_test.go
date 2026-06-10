package state

import (
	"math"
	"path/filepath"
	"testing"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/storage"
)

// TestSocialWealth 验 C10 slice1：社交产 wealth 递减反刷 + 无 [0,1] clamp(可超1) + spend + 日清 + 持久化。
func TestSocialWealth(t *testing.T) {
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

	// 第一次连接型(base=1.0)：awarded=1.0/(1+0)=1.0。
	if w := EarnSocialWealth(1.0); !approx(w, 1.0) {
		t.Fatalf("首次应产 1.0, 得 %.4f", w)
	}
	// 第二次(base=1.0)：递减 awarded=1.0/(1+0.5×1.0)=0.667。
	if w := EarnSocialWealth(1.0); !approx(w, 1.0/1.5) {
		t.Fatalf("第二次应递减到 %.4f, 得 %.4f", 1.0/1.5, w)
	}
	ls, _ := Snapshot()
	// wealth 累计 1.0+0.667=1.667 > 1 —— **关键：wealth 非 [0,1] 标量，绝不被 clamp**。
	if !approx(ls.Wealth, 1.0+1.0/1.5) {
		t.Fatalf("wealth 应累计 %.4f(超1不clamp), 得 %.4f", 1.0+1.0/1.5, ls.Wealth)
	}
	if ls.Wealth <= 1.0 {
		t.Fatal("wealth 应能超过 1.0（无上界 clamp）")
	}

	// base<=0 不产。
	if w := EarnSocialWealth(0); w != 0 {
		t.Fatalf("base0 不应产, 得 %.4f", w)
	}

	// 持久化：reload 应见累计 wealth。
	reloaded, _ := storage.LoadLifeState(life)
	if !approx(reloaded.Wealth, ls.Wealth) {
		t.Fatalf("持久化 wealth 应 %.4f, 得 %.4f", ls.Wealth, reloaded.Wealth)
	}

	// spend：扣款 + 余额不足拒。
	before := ls.Wealth
	if err := SpendWealth(0.5); err != nil {
		t.Fatalf("spend 0.5: %v", err)
	}
	if s, _ := Snapshot(); !approx(s.Wealth, before-0.5) {
		t.Fatalf("spend 后应 %.4f, 得 %.4f", before-0.5, s.Wealth)
	}
	if err := SpendWealth(999); err == nil {
		t.Fatal("余额不足应拒")
	}

	// 日清：ResetEnergyDailyCap 清 social_wealth_today（递减重置），wealth 不动。
	wealthBefore, _ := Snapshot()
	if err := ResetEnergyDailyCap(1.0, 9999); err != nil {
		t.Fatalf("reset: %v", err)
	}
	after, _ := Snapshot()
	if after.SocialWealthToday != 0 {
		t.Fatalf("日清后 social_wealth_today 应 0, 得 %.4f", after.SocialWealthToday)
	}
	if !approx(after.Wealth, wealthBefore.Wealth) {
		t.Fatalf("日清不应动 wealth 余额, 得 %.4f", after.Wealth)
	}
	// 日清后再产，递减重置回满额(base/(1+0))。
	if w := EarnSocialWealth(1.0); !approx(w, 1.0) {
		t.Fatalf("日清后递减重置应产满 1.0, 得 %.4f", w)
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
