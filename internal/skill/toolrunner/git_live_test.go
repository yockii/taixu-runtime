package toolrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taixu.icu/runtime/internal/storage"
)

// TestLiveGit 对真 Forgejo 跑 go-git clone→write→commit→push。
// 需 GIT_LIVE_CLONE_URL（含凭据的 clone URL）；缺则跳过。
func TestLiveGit(t *testing.T) {
	raw := os.Getenv("GIT_LIVE_CLONE_URL")
	if raw == "" {
		t.Skip("no GIT_LIVE_CLONE_URL; skip live git test")
	}
	tmp := t.TempDir()
	if err := storage.Init(filepath.Join(tmp, "t.db")); err != nil {
		t.Fatalf("storage.Init: %v", err)
	}
	defer storage.Close() // 先关 sqlite，免 Windows TempDir 清理时文件占用报错
	if err := Init("test-life", tmp); err != nil {
		t.Fatalf("Init: %v", err)
	}
	r, err := GitClone(1, raw, "work")
	if err != nil {
		t.Fatalf("GitClone: %v", err)
	}
	t.Logf("clone: %s", r.Output)
	// 写产物。
	if _, err := FsWrite(1, "work/deliverable.md", "# go-git 交付测试\n生命 push 验证。\n"); err != nil {
		t.Fatalf("FsWrite: %v", err)
	}
	r, err = GitCommitPush(1, "work", "go-git 交付测试", "测试生命", "t@life.taixu.icu")
	if err != nil {
		t.Fatalf("GitCommitPush: %v", err)
	}
	t.Logf("commit_push: %s", r.Output)
	if !strings.Contains(r.Output, "committed+pushed") {
		t.Fatalf("unexpected: %s", r.Output)
	}
}
