package storage

import (
	"path/filepath"
	"testing"
)

// TestEngagementDedup 验 C6：回应记录 + 窗内查重（窗外不算、别的生命隔离、UPSERT 不堆行）。
func TestEngagementDedup(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "m.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-c6"
	const key = "reply:cmt-123"

	// 未回应过 → 查窗内应 false。
	if done, err := EngagedSince(life, key, 1000); err != nil || done {
		t.Fatalf("初始应未回应, got done=%v err=%v", done, err)
	}

	// 在 t=2000 记一次回应。
	if err := RecordEngagement(life, key, 2000); err != nil {
		t.Fatalf("record: %v", err)
	}
	// 窗起点 1500（≤2000）→ 窗内已回应。
	if done, _ := EngagedSince(life, key, 1500); !done {
		t.Fatal("窗内应判已回应")
	}
	// 窗起点 2500（>2000）→ 窗外，不算（容次日真有新话续聊）。
	if done, _ := EngagedSince(life, key, 2500); done {
		t.Fatal("窗外不应判已回应")
	}
	// 别的生命体隔离。
	if done, _ := EngagedSince("other", key, 1500); done {
		t.Fatal("别的生命不应命中")
	}

	// 同对象再回应 → UPSERT 刷新 last_at，不堆行。
	if err := RecordEngagement(life, key, 5000); err != nil {
		t.Fatalf("re-record: %v", err)
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM social_engagement WHERE life_id=? AND target_key=?`, life, key).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 1 {
		t.Fatalf("同对象再回应应仍1行(UPSERT), 得 %d", rows)
	}
	// 刷新后 last_at=5000，窗起点 4000 内应命中。
	if done, _ := EngagedSince(life, key, 4000); !done {
		t.Fatal("UPSERT 刷新后窗内应命中")
	}
}
