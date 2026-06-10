package storage

import (
	"path/filepath"
	"testing"

	"taixu.icu/runtime/internal/core"
)

// TestLatestEpisodeSummary 验 C6 内容闸依赖：取最近 episode 摘要；空表→空串无错。
func TestLatestEpisodeSummary(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "e.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-c6"
	mustGenome(t, life)

	// 空表 → ("", nil)，不报 ErrNoRows。
	if s, err := LatestEpisodeSummary(life); err != nil || s != "" {
		t.Fatalf("空表应 (\"\", nil), 得 (%q, %v)", s, err)
	}

	if _, err := InsertEpisode(life, &core.Episode{Summary: "第一段：探索交流协议", StartedAt: 1, EndedAt: 2, CreatedAt: 3}); err != nil {
		t.Fatalf("ep1: %v", err)
	}
	if _, err := InsertEpisode(life, &core.Episode{Summary: "第二段：创作短诗", StartedAt: 4, EndedAt: 5, CreatedAt: 6}); err != nil {
		t.Fatalf("ep2: %v", err)
	}
	// 取最近（id 最大）→ 第二段。
	if s, err := LatestEpisodeSummary(life); err != nil || s != "第二段：创作短诗" {
		t.Fatalf("应取最近=第二段, 得 (%q, %v)", s, err)
	}
	// 别的生命隔离。
	if s, _ := LatestEpisodeSummary("other"); s != "" {
		t.Fatalf("别的生命应空, 得 %q", s)
	}
}
