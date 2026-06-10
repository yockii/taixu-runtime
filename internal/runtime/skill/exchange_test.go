package skill

import (
	"os"
	"path/filepath"
	"testing"

	"taixu.icu/runtime/internal/storage"
)

const bundleSkillMd = `---
name: adder
description: adds two numbers
---
运行 run.py 做加法
`

// TestSkillBundleRoundTrip 验 C9 切片1：导出带验证 mastery 的技能 bundle → 另一生命折扣先验导入。
func TestSkillBundleRoundTrip(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "x.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = storage.Close() }()
	const lifeA, lifeB = "life-a", "life-b"
	initLife(t, lifeA)
	initLife(t, lifeB)
	rootA, rootB := t.TempDir(), t.TempDir()

	// --- 发布者 lifeA：建一个带可执行入口 + 已验证 mastery 的技能 ---
	if err := Init(lifeA, rootA, false); err != nil {
		t.Fatal(err)
	}
	instA, err := LoadFrom(bundleSkillMd, Origin{})
	if err != nil {
		t.Fatalf("loadA: %v", err)
	}
	if err := os.WriteFile(filepath.Join(instA.InstallPath, "run.py"), []byte("print(1+1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := storage.SetSkillMastery(instA.ID, 0.8); err != nil { // 模拟 C2 验证出的高 mastery
		t.Fatal(err)
	}

	b, err := ExportBundle("adder")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if b.VerifiedMastery != 0.8 {
		t.Errorf("bundle 应带验证 mastery 0.8, 得 %.3f", b.VerifiedMastery)
	}
	if b.EntrypointLang != "python" || b.EntrypointCode != "print(1+1)" {
		t.Errorf("bundle 应含 python 入口, 得 lang=%q code=%q", b.EntrypointLang, b.EntrypointCode)
	}
	b.PublisherDID = "didA"

	// --- 导入者 lifeB：折扣先验导入 ---
	if err := Init(lifeB, rootB, false); err != nil {
		t.Fatal(err)
	}
	instB, err := ImportBundle(b, 0.5)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	// 折扣先验：0.8 × 0.5 = 0.4（信任但验证，剩下靠 C2 自校准）。
	if instB.Mastery < 0.39 || instB.Mastery > 0.41 {
		t.Errorf("导入 mastery 应≈0.4(折扣先验), 得 %.3f", instB.Mastery)
	}
	if instB.AuthoredFrom != "import:didA" {
		t.Errorf("血缘应 import:didA, 得 %q", instB.AuthoredFrom)
	}
	// 可执行入口随之落盘。
	if ep := detectEntrypoint(instB.InstallPath); filepath.Base(ep) != "run.py" {
		t.Errorf("导入应带 run.py 入口, 得 %q", ep)
	}
	if got, _ := os.ReadFile(filepath.Join(instB.InstallPath, "run.py")); string(got) != "print(1+1)" {
		t.Errorf("入口内容应保留, 得 %q", got)
	}

	// 默认折扣（传 0 → DefaultTrustDiscount=0.5）。
	instB2, err := ImportBundle(b, 0)
	if err != nil {
		t.Fatalf("import default: %v", err)
	}
	if instB2.Mastery < 0.39 || instB2.Mastery > 0.41 {
		t.Errorf("默认折扣 mastery 应≈0.4, 得 %.3f", instB2.Mastery)
	}
}
