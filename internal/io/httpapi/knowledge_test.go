package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/storage"
)

// TestKnowledgeAPI 验证 /api/knowledge 列表 + /api/knowledge/{id} 详情端点。
func TestKnowledgeAPI(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "h.db")); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	defer func() { _ = storage.Close() }()
	const life = "life-h"
	if err := storage.InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	lifeID = life // 包内 var

	id, err := storage.InsertKnowledgeEntry(life, &storage.KnowledgeEntry{
		RootGoalID: 5, Topic: "测试主题", Body: "正文结论与要点。", CreatedAt: 10, UpdatedAt: 10,
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// 用与生产相同的路由模式注册（确保 {id} 路径参数生效）。
	mux := http.NewServeMux()
	mux.HandleFunc("/api/knowledge", apiKnowledgeList)
	mux.HandleFunc("/api/knowledge/{id}", apiKnowledgeDetail)

	// 列表。
	{
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/knowledge", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("list status = %d", rec.Code)
		}
		var list []storage.KnowledgeEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatalf("list decode: %v", err)
		}
		if len(list) != 1 || list[0].Topic != "测试主题" {
			t.Fatalf("list wrong: %+v", list)
		}
	}

	// 详情。
	{
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/knowledge/"+strconv.FormatInt(id, 10), nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("detail status = %d", rec.Code)
		}
		var e storage.KnowledgeEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
			t.Fatalf("detail decode: %v", err)
		}
		if e.Body != "正文结论与要点。" || e.RootGoalID != 5 {
			t.Fatalf("detail wrong: %+v", e)
		}
	}

	// 不存在 → 404。
	{
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/knowledge/9999", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("missing detail status = %d, want 404", rec.Code)
		}
	}
}
