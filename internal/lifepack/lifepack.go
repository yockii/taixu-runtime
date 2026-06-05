// Package lifepack 数字生命体的加密导出 / 导入包（.mvlife）。
//
// 这是所有权宪法（docs/06：生命属于用户、可导出 / 迁移 / 离线）的落地件，也是 Phase 0
// 出关硬条件（PHASE-0-PRD §7.4「本地加密包 导出→导入 成功」）。
//
// 包结构（单文件 .mvlife）：
//
//	magic[8] | salt[16] | nonce[12] | AES-256-GCM( gzip( tar{ manifest.json + mindverse.db + workspace/** } ) )
//
// 密钥派生：scrypt(passphrase, salt)。AAD = magic（绑定格式版本，防降级 / 篡改）。
// 口令是唯一钥匙——丢了不可恢复（R17：Phase 0 不做托管 / 找回，导出时须告知用户）。
//
// 纯文件 + crypto，不依赖 storage（解耦）：导出方先把一致快照（VACUUM INTO）落到临时文件再传入。
package lifepack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// magic 包头魔数（含格式版本 \x01）。改格式即升此字节，旧包用旧 magic 校验失败 → 明确拒绝。
var magic = []byte("MVLIFE\x01\n")

const (
	saltLen  = 16
	nonceLen = 12
	keyLen   = 32 // AES-256

	// scrypt 参数（2026 桌面合理档；提高 N 增强抗暴破，代价是导出/导入各算一次）。
	scryptN = 1 << 15
	scryptR = 8
	scryptP = 1

	// maxEntrySize 单个 tar 条目解包上限（防恶意包炸内存 / 磁盘）。Phase 0 单库够用。
	maxEntrySize = 2 << 30 // 2 GiB
)

// Manifest 包内元信息（明文存于加密层内的 tar 里，解密后才可读）。
type Manifest struct {
	Format        int    `json:"format"`         // 包格式版本（当前 1）
	AppVersion    string `json:"app_version"`    // 导出时的运行时版本
	SchemaVersion string `json:"schema_version"` // 导出时 DB schema 版本（schema_meta.version）
	LifeID        string `json:"life_id"`
	GenomeVersion string `json:"genome_version"`
	ExportedAt    int64  `json:"exported_at"`
}

// FormatVersion 当前包格式版本。
const FormatVersion = 1

// Export 把一致 DB 快照 + workspace 打成加密 .mvlife 写入 w。
//
// dbSnapshotPath：调用方用 VACUUM INTO 产出的一致快照文件（不要直接传运行中的库）。
// workspaceDir：其下内容（skills/ sandbox/ 等）整体收入包；不存在则跳过。
func Export(w io.Writer, dbSnapshotPath, workspaceDir string, m Manifest, passphrase string) error {
	if passphrase == "" {
		return errors.New("lifepack: empty passphrase")
	}
	m.Format = FormatVersion

	// 1. 组 tar + gzip 到内存（GCM 需整段明文；Phase 0 单库 MB 级，可接受）。
	var plain bytes.Buffer
	gz := gzip.NewWriter(&plain)
	tw := tar.NewWriter(gz)

	manifestBytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := writeTarBytes(tw, "manifest.json", manifestBytes); err != nil {
		return err
	}
	if err := writeTarFile(tw, "mindverse.db", dbSnapshotPath); err != nil {
		return fmt.Errorf("pack db: %w", err)
	}
	if err := writeTarTree(tw, workspaceDir, "workspace"); err != nil {
		return fmt.Errorf("pack workspace: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}

	// 2. 派生密钥 + 加密。
	salt := make([]byte, saltLen)
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("rand salt: %w", err)
	}
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("rand nonce: %w", err)
	}
	gcm, err := newGCM(passphrase, salt)
	if err != nil {
		return err
	}
	ct := gcm.Seal(nil, nonce, plain.Bytes(), magic)

	// 3. 写包：magic | salt | nonce | ciphertext。
	for _, chunk := range [][]byte{magic, salt, nonce, ct} {
		if _, err := w.Write(chunk); err != nil {
			return fmt.Errorf("write package: %w", err)
		}
	}
	return nil
}

// Import 解密 .mvlife 并把 mindverse.db + workspace 还原到目标路径，返回 Manifest。
//
// destDBPath：还原后的库文件路径。destWorkspaceDir：workspace 内容还原到此目录下。
// 调用方须保证目标为空 / 可覆盖（boot 时仅在无活体库时调用）。
func Import(r io.Reader, passphrase, destDBPath, destWorkspaceDir string) (Manifest, error) {
	var m Manifest
	if passphrase == "" {
		return m, errors.New("lifepack: empty passphrase")
	}
	blob, err := io.ReadAll(r)
	if err != nil {
		return m, fmt.Errorf("read package: %w", err)
	}
	head := len(magic) + saltLen + nonceLen
	if len(blob) < head {
		return m, errors.New("lifepack: package too short / not a .mvlife")
	}
	if !bytes.Equal(blob[:len(magic)], magic) {
		return m, errors.New("lifepack: bad magic (wrong format or not a .mvlife)")
	}
	salt := blob[len(magic) : len(magic)+saltLen]
	nonce := blob[len(magic)+saltLen : head]
	ct := blob[head:]

	gcm, err := newGCM(passphrase, salt)
	if err != nil {
		return m, err
	}
	plain, err := gcm.Open(nil, nonce, ct, magic)
	if err != nil {
		return m, errors.New("lifepack: decrypt failed (wrong passphrase or corrupted package)")
	}

	gz, err := gzip.NewReader(bytes.NewReader(plain))
	if err != nil {
		return m, fmt.Errorf("gunzip: %w", err)
	}
	tr := tar.NewReader(gz)
	gotManifest, gotDB := false, false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		switch {
		case hdr.Name == "manifest.json":
			b, err := readLimited(tr)
			if err != nil {
				return m, err
			}
			if err := json.Unmarshal(b, &m); err != nil {
				return m, fmt.Errorf("parse manifest: %w", err)
			}
			if m.Format > FormatVersion {
				return m, fmt.Errorf("lifepack: package format v%d newer than supported v%d (upgrade runtime)", m.Format, FormatVersion)
			}
			gotManifest = true
		case hdr.Name == "mindverse.db":
			if err := writeOutFile(destDBPath, tr); err != nil {
				return m, fmt.Errorf("restore db: %w", err)
			}
			gotDB = true
		case strings.HasPrefix(hdr.Name, "workspace/"):
			rel := strings.TrimPrefix(hdr.Name, "workspace/")
			dest, err := safeJoin(destWorkspaceDir, rel)
			if err != nil {
				return m, err
			}
			if err := writeOutFile(dest, tr); err != nil {
				return m, fmt.Errorf("restore %s: %w", hdr.Name, err)
			}
		}
	}
	if !gotManifest || !gotDB {
		return m, errors.New("lifepack: package missing manifest or db (corrupted)")
	}
	return m, nil
}

func newGCM(passphrase string, salt []byte) (cipher.AEAD, error) {
	key, err := scrypt.Key([]byte(passphrase), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return nil, fmt.Errorf("scrypt: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	return cipher.NewGCM(block)
}

func writeTarBytes(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func writeTarFile(tw *tar.Writer, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: fi.Size(), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

// writeTarTree 把 root 下所有普通文件以 prefix/<rel> 收入 tar；root 不存在则静默跳过。
func writeTarTree(tw *tar.Writer, root, prefix string) error {
	if root == "" {
		return nil
	}
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := prefix + "/" + filepath.ToSlash(rel)
		return writeTarFile(tw, name, path)
	})
}

func readLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxEntrySize))
}

// writeOutFile 把 r 内容写到 path（建父目录、限大小、覆盖）。
func writeOutFile(path string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, io.LimitReader(r, maxEntrySize))
	return err
}

// safeJoin 防 tar 路径穿越（../ / 绝对路径）：结果必须仍在 base 内。
func safeJoin(base, rel string) (string, error) {
	clean := filepath.Clean("/" + filepath.FromSlash(rel)) // 归一后以 / 开头，吃掉 ../
	dest := filepath.Join(base, clean)
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	if absDest != absBase && !strings.HasPrefix(absDest, absBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("lifepack: unsafe path in package: %s", rel)
	}
	return dest, nil
}
