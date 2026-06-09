package storage

import (
	"path/filepath"
	"testing"

	"taixu.icu/runtime/internal/core"
)

// TestSemanticCandidateConfidence 验证语义固化链修复：
// record_learning 的 digest 以 mastery 作初见置信入库，学透的（≥0.75）即可被 ShallowReflect 固化，
// 浅学的（<0.75）留候选区。digest 唯一、无法靠"重复 +0.1"升置信，故初见置信是唯一闸门。
func TestSemanticCandidateConfidence(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "m.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()

	const life = "life-sem"
	if err := InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}

	// 学透的 digest（mastery 0.83）→ 初见置信 0.83 ≥ 0.75
	if err := UpsertSemanticCandidateConf(life, "deep digest", "skill:record_learning", 100, 0.83); err != nil {
		t.Fatalf("upsert deep: %v", err)
	}
	// 浅学的 digest（mastery 0.34）→ 初见置信 0.34 < 0.75
	if err := UpsertSemanticCandidateConf(life, "shallow digest", "skill:record_learning", 100, 0.34); err != nil {
		t.Fatalf("upsert shallow: %v", err)
	}
	// 默认入口仍是 0.5
	if err := UpsertSemanticCandidate(life, "pattern", "extractor:v2", 100); err != nil {
		t.Fatalf("upsert default: %v", err)
	}

	above, err := ListCandidatesAboveConfidence(life, 0.75, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(above) != 1 {
		t.Fatalf("candidates >=0.75 = %d want 1 (only the deep digest)", len(above))
	}
	if above[0].Content != "deep digest" {
		t.Fatalf("promotable candidate = %q want 'deep digest'", above[0].Content)
	}

	// 旧死值 0.5 的回归：默认入口的候选不应越过阈值
	for _, c := range above {
		if c.Content == "pattern" || c.Content == "shallow digest" {
			t.Fatalf("%q should be below 0.75 threshold", c.Content)
		}
	}
}
