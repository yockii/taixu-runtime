package skill

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDetectEntrypoint 验 C4：技能目录含可执行入口时被探测到，否则空。
func TestDetectEntrypoint(t *testing.T) {
	dir := t.TempDir()
	if got := detectEntrypoint(dir); got != "" {
		t.Fatalf("空目录应无入口, 得 %q", got)
	}
	if got := detectEntrypoint(""); got != "" {
		t.Fatalf("空路径应空, 得 %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.py"), []byte("print(1)"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := detectEntrypoint(dir); filepath.Base(got) != "run.py" {
		t.Fatalf("应测到 run.py, 得 %q", got)
	}
	// 目录同名不算入口。
	sub := t.TempDir()
	if err := os.Mkdir(filepath.Join(sub, "run.sh"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := detectEntrypoint(sub); got != "" {
		t.Fatalf("同名目录不应算入口, 得 %q", got)
	}
}

// TestEntrypointFilename 验 C4：结晶脚本语言 → 落盘入口文件名映射；未知语言不落盘。
func TestEntrypointFilename(t *testing.T) {
	cases := map[string]string{
		"python": "run.py", "py": "run.py", "python3": "run.py", "PYTHON": "run.py",
		"node": "run.js", "js": "run.js", "javascript": "run.js",
		"shell": "run.sh", "sh": "run.sh", "bash": "run.sh",
		"":     "", "ruby": "", "go": "",
	}
	for lang, want := range cases {
		if got := entrypointFilename(lang); got != want {
			t.Errorf("entrypointFilename(%q) = %q, 期 %q", lang, got, want)
		}
	}
}
