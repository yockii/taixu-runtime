package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestSnapshotInto 验证 VACUUM INTO 在 modernc 驱动下产出可打开的一致快照。
func TestSnapshotInto(t *testing.T) {
	dir := t.TempDir()
	if err := Init(filepath.Join(dir, "live.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()

	// 写一点数据，确认快照含之。
	if err := SetMeta("snap_probe", "alive"); err != nil {
		t.Fatalf("setmeta: %v", err)
	}

	snap := filepath.Join(dir, "snap.db") // 必须不存在
	if err := SnapshotInto(snap); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if fi, err := os.Stat(snap); err != nil || fi.Size() == 0 {
		t.Fatalf("snapshot file missing/empty: %v", err)
	}

	// 快照可独立打开且数据在。
	sdb, err := sql.Open("sqlite", "file:"+snap+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer sdb.Close()
	var v string
	if err := sdb.QueryRow(`SELECT value FROM schema_meta WHERE key='snap_probe'`).Scan(&v); err != nil {
		t.Fatalf("query snapshot: %v", err)
	}
	if v != "alive" {
		t.Fatalf("snapshot missing probe data: got %q", v)
	}
}
