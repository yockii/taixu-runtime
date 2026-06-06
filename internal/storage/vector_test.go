package storage

import (
	"path/filepath"
	"testing"

	"mindverse/internal/core"
)

// mustGenome 插入一条出生记录，满足各记忆表 life_id REFERENCES genome 的 FK。
func mustGenome(t *testing.T, life string) {
	t.Helper()
	if err := InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("insert genome: %v", err)
	}
}

// TestVectorRowsAndBackfillIdempotent 验证向量检索候选取行 + 回填幂等：
//   - InsertEpisode 不带向量 → 出现在 ListRowsMissingEmbedding、不出现在 ListEmbeddedRows；
//   - UpdateEmbedding 回写后 → 反转（已嵌过的不再出现在 missing，幂等：第二次回填取不到）。
func TestVectorRowsAndBackfillIdempotent(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "v.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-x"
	mustGenome(t, life)

	// 两条 episode，均无向量。
	id1, err := InsertEpisode(life, &core.Episode{Summary: "学习 Go 并发", StartedAt: 1, EndedAt: 2, CreatedAt: 3})
	if err != nil {
		t.Fatalf("insert ep1: %v", err)
	}
	if _, err := InsertEpisode(life, &core.Episode{Summary: "读了一篇论文", StartedAt: 4, EndedAt: 5, CreatedAt: 6}); err != nil {
		t.Fatalf("insert ep2: %v", err)
	}

	// 初始：两条都缺向量，零条带向量。
	missing, err := ListRowsMissingEmbedding(life, "episodic", 0, 100)
	if err != nil {
		t.Fatalf("list missing: %v", err)
	}
	if len(missing) != 2 {
		t.Fatalf("missing = %d, want 2", len(missing))
	}
	embedded, err := ListEmbeddedRows(life, "episodic", "", 100)
	if err != nil {
		t.Fatalf("list embedded: %v", err)
	}
	if len(embedded) != 0 {
		t.Fatalf("embedded = %d, want 0", len(embedded))
	}

	// 回填 id1。
	if err := UpdateEmbedding("episodic", id1, []byte{1, 2, 3, 4}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// 现在：缺向量 1 条、带向量 1 条。幂等：再查 missing 不含 id1。
	missing2, _ := ListRowsMissingEmbedding(life, "episodic", 0, 100)
	if len(missing2) != 1 {
		t.Fatalf("missing after backfill = %d, want 1", len(missing2))
	}
	if missing2[0].ID == id1 {
		t.Error("already-embedded row reappeared in missing (not idempotent)")
	}
	embedded2, _ := ListEmbeddedRows(life, "episodic", "", 100)
	if len(embedded2) != 1 || embedded2[0].ID != id1 {
		t.Fatalf("embedded after backfill = %+v, want [id1]", embedded2)
	}

	// UpdateEmbedding 传 nil 不应清空已有向量（避免误清）。
	if err := UpdateEmbedding("episodic", id1, nil); err != nil {
		t.Fatalf("update nil: %v", err)
	}
	if e, _ := ListEmbeddedRows(life, "episodic", "", 100); len(e) != 1 {
		t.Error("nil update should not clear existing embedding")
	}

	// 非法 layer → 安全空返回，不报错。
	if rows, err := ListEmbeddedRows(life, "bogus", "", 10); err != nil || rows != nil {
		t.Errorf("bogus layer = %v, %v; want nil,nil", rows, err)
	}
}

// TestSemanticEmbeddingThroughPromotion 验证语义固化携带向量：
// PromoteToConfirmedWithEmbedding 把 blob 写进 semantic_confirmed.embedding，可被向量召回取到。
func TestSemanticEmbeddingThroughPromotion(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "s.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-y"
	mustGenome(t, life)

	if err := UpsertSemanticCandidateConf(life, "天空是蓝色的", "test", 100, 0.9); err != nil {
		t.Fatalf("upsert candidate: %v", err)
	}
	cands, _ := ListCandidatesAboveConfidence(life, 0.75, 10)
	if len(cands) != 1 {
		t.Fatalf("candidates = %d, want 1", len(cands))
	}
	if err := PromoteToConfirmedWithEmbedding(life, cands[0].ID, cands[0].Content, cands[0].Confidence, 200, []byte{9, 8, 7, 6}); err != nil {
		t.Fatalf("promote: %v", err)
	}
	embedded, _ := ListEmbeddedRows(life, "semantic", "", 10)
	if len(embedded) != 1 {
		t.Fatalf("embedded semantic = %d, want 1", len(embedded))
	}
	if string(embedded[0].Blob) != string([]byte{9, 8, 7, 6}) {
		t.Errorf("embedding blob = %v, want [9 8 7 6]", embedded[0].Blob)
	}
}
