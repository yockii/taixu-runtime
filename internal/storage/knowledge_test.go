package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestKnowledgeEntryCRUD 覆盖 dossier 入库 → 列表（含正文摘要截断）→ 单篇全文 → 按根目标去重。
func TestKnowledgeEntryCRUD(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "k.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-k"
	mkLife(t, life)

	longBody := "结论：" + strings.Repeat("要点。", 300) // 远超摘要 280 rune
	id, err := InsertKnowledgeEntry(life, &KnowledgeEntry{
		RootGoalID: 42, Topic: "Rust 异步运行时", Body: longBody, CreatedAt: 100, UpdatedAt: 100,
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id <= 0 {
		t.Fatalf("bad id %d", id)
	}

	// 列表：正文应被截断为摘要。
	list, err := ListKnowledge(life, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 entry, got %d", len(list))
	}
	if list[0].Topic != "Rust 异步运行时" || list[0].RootGoalID != 42 {
		t.Errorf("list entry wrong: %+v", list[0])
	}
	if !strings.HasSuffix(list[0].Body, "…") || len([]rune(list[0].Body)) > 281 {
		t.Errorf("list body should be truncated preview, got %d runes", len([]rune(list[0].Body)))
	}

	// 详情：全文不截断。
	full, err := GetKnowledge(life, id)
	if err != nil || full == nil {
		t.Fatalf("get: %v full=%v", err, full)
	}
	if full.Body != longBody {
		t.Errorf("detail body should be full text")
	}

	// 按根目标去重判定。
	has, _ := HasKnowledgeForRootGoal(life, 42)
	if !has {
		t.Errorf("should report existing dossier for root 42")
	}
	has, _ = HasKnowledgeForRootGoal(life, 99)
	if has {
		t.Errorf("should not report dossier for unknown root 99")
	}

	// 跨生命隔离：别的生命体读不到。
	if other, _ := GetKnowledge("life-other", id); other != nil {
		t.Errorf("knowledge leaked across lives")
	}
}
