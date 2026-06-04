// Package memoryengine SQLite 存储 + 四层记忆访问。
//
// Phase 0.1 仅实现：
//   - 数据库打开 + schema 迁移加载
//   - Genome / LifeState / MentalState / Values / LifecycleState 读写
//   - RawTrail append
//
// 四层记忆完整实现在 Phase 0.2。
package memoryengine

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store SQLite 存储。
type Store struct {
	db *sql.DB
}

// Open 打开（或创建）SQLite 数据库并应用迁移。
func Open(path string) (*Store, error) {
	// 不在 DSN 强制 WAL：Windows bind-mount 下 modernc/sqlite 触发 atomic-write ioctl 失败 (err 4618)。
	// WAL 改在 schema 中按平台条件尝试，失败回退至 DELETE。
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close 关闭数据库。
func (s *Store) Close() error {
	return s.db.Close()
}

// DB 暴露原始 *sql.DB（包内其他文件用）。其他模块通过类型方法访问。
func (s *Store) DB() *sql.DB { return s.db }

// migrate 顺序应用 migrations/*.sql。冪等：用 schema_migrations 表跟踪。
func (s *Store) migrate() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		var applied int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE filename = ?`, f).Scan(&applied); err != nil {
			return fmt.Errorf("check applied %s: %w", f, err)
		}
		if applied > 0 {
			continue
		}
		content, err := fs.ReadFile(migrations, filepath.ToSlash("migrations/"+f))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := s.db.Exec(string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f, err)
		}
		if _, err := s.db.Exec(`INSERT INTO schema_migrations (filename, applied_at) VALUES (?, strftime('%s','now'))`, f); err != nil {
			return fmt.Errorf("mark applied %s: %w", f, err)
		}
	}
	return nil
}
