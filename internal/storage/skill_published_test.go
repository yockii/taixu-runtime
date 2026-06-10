package storage

import (
	"path/filepath"
	"testing"

	"taixu.icu/runtime/internal/core"
)

// TestC11UnpublishedReadySkills 验 C11 发布引导候选查询 + 标记去重：
// 只取 ready+mastery>=floor+未发布；标记发布后剔除；按 mastery 降序；不串生命。
func TestC11UnpublishedReadySkills(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "m.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-c11"
	if err := InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
	mk := func(id, name, status string, mastery float64) {
		if err := UpsertSkillInstance(&SkillInstance{
			ID: id, LifeID: life, Name: name, SeedRef: "ref-" + id,
			Status: status, Mastery: mastery, CreatedAt: 100,
		}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("s1", "高掌握ready", "ready", 0.85)          // 入选
	mk("s2", "半生不熟ready", "ready", 0.50)         // mastery 低 → 不选
	mk("s3", "未ready", "pending_approval", 0.90)   // 非 ready → 不选
	mk("s4", "刚好门槛", "ready", 0.72)              // > floor → 入选

	got, err := ListUnpublishedReadySkills(life, nudgeFloorTest, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("应 2 个(高掌握ready+刚好门槛), 得 %d: %+v", len(got), got)
	}
	if got[0].Name != "高掌握ready" { // 按 mastery 降序
		t.Fatalf("应按 mastery 降序首个=高掌握ready, 得 %s", got[0].Name)
	}

	// 标记 s1 已发布 → 从候选剔除。
	if err := MarkSkillPublishedByName(life, "高掌握ready", 12345); err != nil {
		t.Fatalf("mark: %v", err)
	}
	got2, err := ListUnpublishedReadySkills(life, nudgeFloorTest, 10)
	if err != nil {
		t.Fatalf("list2: %v", err)
	}
	if len(got2) != 1 || got2[0].Name != "刚好门槛" {
		t.Fatalf("发布后应剩 1(刚好门槛), 得 %d: %+v", len(got2), got2)
	}

	// 别的生命的技能不串。
	if g3, _ := ListUnpublishedReadySkills("other-life", nudgeFloorTest, 10); len(g3) != 0 {
		t.Fatalf("别的生命应空, 得 %d", len(g3))
	}
}

const nudgeFloorTest = 0.7
