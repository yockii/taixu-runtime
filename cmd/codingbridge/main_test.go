package main

import (
	"path/filepath"
	"testing"
)

// TestJailWorkdir 验 C7：工作目录强制落 workroot 内，越界(绝对/..)拒。
func TestJailWorkdir(t *testing.T) {
	root := t.TempDir()
	cfg := config{workRoot: root}

	// 空 → default 子目录。
	if got, err := cfg.jailWorkdir(""); err != nil || filepath.Base(got) != "default" {
		t.Fatalf("空应=default 子目录, got=%q err=%v", got, err)
	}
	// 普通子目录 OK。
	if got, err := cfg.jailWorkdir("proj1"); err != nil || got != filepath.Join(root, "proj1") {
		t.Fatalf("proj1 应在 root 内, got=%q err=%v", got, err)
	}
	// 绝对路径拒（跨平台：用另一个 temp 目录作绝对外部路径，两平台 IsAbs 均 true）。
	outside := t.TempDir()
	if _, err := cfg.jailWorkdir(outside); err == nil {
		t.Fatal("绝对路径应拒")
	}
	// .. 越界拒。
	if _, err := cfg.jailWorkdir("../escape"); err == nil {
		t.Fatal("../ 越界应拒")
	}
	if _, err := cfg.jailWorkdir("a/../../escape"); err == nil {
		t.Fatal("嵌套 .. 越界应拒")
	}
}

// TestSubtleEqual 验常量时间比较正确性（鉴权用）。
func TestSubtleEqual(t *testing.T) {
	if !subtleEqual("Bearer abc", "Bearer abc") {
		t.Fatal("相同应 true")
	}
	if subtleEqual("Bearer abc", "Bearer abd") {
		t.Fatal("不同应 false")
	}
	if subtleEqual("a", "ab") {
		t.Fatal("不等长应 false")
	}
	if subtleEqual("", "x") {
		t.Fatal("空 vs 非空应 false")
	}
}

// TestAtoiDefault 验超时解析容错。
func TestAtoiDefault(t *testing.T) {
	if atoiDefault("120", 0) != 120 {
		t.Fatal("120 应解析")
	}
	if atoiDefault("", 5) != 5 {
		t.Fatal("空应回默认")
	}
	if atoiDefault("12x", 5) != 5 {
		t.Fatal("非数字应回默认")
	}
}
