package skill

import (
	"os"
	"path/filepath"
	"testing"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/storage"
)

const testSkillMd = `---
name: test-skill
description: a test skill
---
do the thing
`

func mkSkillFolder(t *testing.T, root, dir, owner string) {
	t.Helper()
	folder := filepath.Join(root, dir)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "SKILL.md"), []byte(testSkillMd), 0o644); err != nil {
		t.Fatal(err)
	}
	if owner != "" {
		if err := os.WriteFile(filepath.Join(folder, ownerFile), []byte(owner), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func initLife(t *testing.T, id string) {
	t.Helper()
	if err := storage.InsertGenome(&core.Genome{LifeID: id, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("insert genome %s: %v", id, err)
	}
}

func skillCount(t *testing.T, lifeID string) int {
	t.Helper()
	all, err := storage.ListSkillInstances(lifeID, 100)
	if err != nil {
		t.Fatal(err)
	}
	return len(all)
}

// TestScanDirOwnership 验证 ghost-skill 修复：ScanDir 只收养属于当前生命体的技能文件夹，
// 前主遗留（清库换生命但 workspace 仍在）/ 无归属标记的文件夹不被静默收养。
func TestScanDirOwnership(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "s.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = storage.Close() }()

	root := t.TempDir()
	const lifeA = "life-a"
	const lifeB = "life-b"
	initLife(t, lifeA)
	initLife(t, lifeB)

	// 1) 无 owner 标记的遗留文件夹 → 当前生命不收养
	mkSkillFolder(t, root, "orphan", "")
	if err := Init(lifeA, root, false); err != nil {
		t.Fatal(err)
	}
	if n, err := ScanDir(); err != nil || n != 0 {
		t.Fatalf("orphan (no owner) should be skipped: loaded=%d err=%v", n, err)
	}
	if got := skillCount(t, lifeA); got != 0 {
		t.Fatalf("lifeA skills=%d want 0 (orphan must not be adopted)", got)
	}

	// 2) owner=lifeA 的文件夹 → lifeA 收养
	mkSkillFolder(t, root, "mine", lifeA)
	if n, err := ScanDir(); err != nil || n != 1 {
		t.Fatalf("owned folder should load: loaded=%d err=%v", n, err)
	}
	if got := skillCount(t, lifeA); got != 1 {
		t.Fatalf("lifeA skills=%d want 1", got)
	}

	// 3) 换生命 lifeB（同一 workspace 仍在）→ lifeA 的技能不被 lifeB 收养（ghost-skill 核心场景）
	if err := Init(lifeB, root, false); err != nil {
		t.Fatal(err)
	}
	if n, err := ScanDir(); err != nil || n != 0 {
		t.Fatalf("lifeB must adopt nothing from lifeA's workspace: loaded=%d err=%v", n, err)
	}
	if got := skillCount(t, lifeB); got != 0 {
		t.Fatalf("lifeB skills=%d want 0 (no ghost adoption)", got)
	}
}

// TestLoadFolderStampsOwner 验证经 loadFolder 采纳的文件夹会盖上当前生命体归属标记，
// 使下次 boot 扫描认得它。
func TestLoadFolderStampsOwner(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "s.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = storage.Close() }()

	root := t.TempDir()
	const life = "life-x"
	initLife(t, life)
	if err := Init(life, root, false); err != nil {
		t.Fatal(err)
	}

	// 模拟自创/粘贴采纳：先建文件夹（无 owner），再经 loadFolder 采纳 → 应盖章 + 入库。
	mkSkillFolder(t, root, "fresh", "")
	folder := filepath.Join(root, "fresh")
	content, _ := os.ReadFile(filepath.Join(folder, "SKILL.md"))
	if _, err := loadFolder(folder, string(content), Origin{}); err != nil {
		t.Fatalf("loadFolder: %v", err)
	}
	if got := folderOwner(folder); got != life {
		t.Fatalf("owner stamp=%q want %q", got, life)
	}
	// 盖章后 ScanDir 应认得它
	if n, err := ScanDir(); err != nil || n != 1 {
		t.Fatalf("after stamp ScanDir should load: loaded=%d err=%v", n, err)
	}
}
