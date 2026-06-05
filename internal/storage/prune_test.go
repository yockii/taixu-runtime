package storage

import (
	"path/filepath"
	"testing"

	"mindverse/internal/core"
)

func countRows(t *testing.T, table, lifeID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE life_id = ?`, lifeID).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// TestPruneRawTrailBefore 验证只删 id < beforeID 的 raw_trail。
func TestPruneRawTrailBefore(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "p.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-prune"
	if err := InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	for i := 0; i < 10; i++ { // ids 1..10
		if err := AppendRawTrail(life, 1, "ev", "{}", int64(i)); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	n, err := PruneRawTrailBefore(life, 6) // 删 id 1..5
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 5 {
		t.Fatalf("deleted %d want 5", n)
	}
	if got := countRows(t, "raw_trail", life); got != 5 {
		t.Fatalf("remaining %d want 5", got)
	}
	// beforeID<=1 不删
	if n, _ := PruneRawTrailBefore(life, 1); n != 0 {
		t.Fatalf("beforeID=1 should delete 0, got %d", n)
	}
}

// TestPruneWorkingMemoryKeepRecent 验证只保留最近 keep 条。
func TestPruneWorkingMemoryKeepRecent(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "w.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-wm"
	if err := InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := AppendWorking(life, int64(i), "slot", "c", int64(i)); err != nil {
			t.Fatalf("append wm: %v", err)
		}
	}
	n, err := PruneWorkingMemoryKeepRecent(life, 3)
	if err != nil {
		t.Fatalf("prune wm: %v", err)
	}
	if n != 7 {
		t.Fatalf("deleted %d want 7", n)
	}
	if got := countRows(t, "working_memory", life); got != 3 {
		t.Fatalf("remaining %d want 3", got)
	}
}
