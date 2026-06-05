package lifepack

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// writeFile 测试辅助：建父目录 + 写内容。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExportImportRoundtrip(t *testing.T) {
	src := t.TempDir()
	dbPath := filepath.Join(src, "mindverse.db")
	wsDir := filepath.Join(src, "workspace")
	writeFile(t, dbPath, "FAKE-SQLITE-BYTES-\x00\x01\x02")
	writeFile(t, filepath.Join(wsDir, "skills", "regex-engine", "SKILL.md"), "# regex skill\n自创技能")
	writeFile(t, filepath.Join(wsDir, "sandbox", "note.txt"), "生命体的笔记")

	man := Manifest{AppVersion: "test", SchemaVersion: "7", LifeID: "local-abc", GenomeVersion: "v2", ExportedAt: 1780000000}

	var pkg bytes.Buffer
	if err := Export(&pkg, dbPath, wsDir, man, "correct horse battery staple"); err != nil {
		t.Fatalf("export: %v", err)
	}
	if pkg.Len() == 0 {
		t.Fatal("empty package")
	}

	dst := t.TempDir()
	dstDB := filepath.Join(dst, "mindverse.db")
	dstWS := filepath.Join(dst, "workspace")
	got, err := Import(bytes.NewReader(pkg.Bytes()), "correct horse battery staple", dstDB, dstWS)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	// manifest 还原
	if got.LifeID != "local-abc" || got.GenomeVersion != "v2" || got.Format != FormatVersion {
		t.Fatalf("manifest mismatch: %+v", got)
	}
	// db 字节一致
	assertFileEq(t, dbPath, dstDB)
	// workspace 文件一致
	assertFileEq(t, filepath.Join(wsDir, "skills", "regex-engine", "SKILL.md"), filepath.Join(dstWS, "skills", "regex-engine", "SKILL.md"))
	assertFileEq(t, filepath.Join(wsDir, "sandbox", "note.txt"), filepath.Join(dstWS, "sandbox", "note.txt"))
}

func TestImportWrongPassphrase(t *testing.T) {
	src := t.TempDir()
	dbPath := filepath.Join(src, "mindverse.db")
	writeFile(t, dbPath, "data")
	var pkg bytes.Buffer
	if err := Export(&pkg, dbPath, "", Manifest{LifeID: "x"}, "right-pass"); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if _, err := Import(bytes.NewReader(pkg.Bytes()), "wrong-pass", filepath.Join(dst, "db"), filepath.Join(dst, "ws")); err == nil {
		t.Fatal("expected decrypt failure with wrong passphrase")
	}
}

func TestImportBadMagic(t *testing.T) {
	dst := t.TempDir()
	if _, err := Import(bytes.NewReader([]byte("not a mvlife package at all............")), "p", filepath.Join(dst, "db"), filepath.Join(dst, "ws")); err == nil {
		t.Fatal("expected bad-magic rejection")
	}
}

func TestEmptyPassphraseRejected(t *testing.T) {
	if err := Export(&bytes.Buffer{}, "x", "", Manifest{}, ""); err == nil {
		t.Fatal("expected empty-passphrase rejection on export")
	}
}

func assertFileEq(t *testing.T, a, b string) {
	t.Helper()
	ba, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		t.Fatalf("read restored %s: %v", b, err)
	}
	if !bytes.Equal(ba, bb) {
		t.Fatalf("content mismatch: %s vs %s", a, b)
	}
}
