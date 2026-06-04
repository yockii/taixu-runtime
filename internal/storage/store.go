// Package storage 单例 SQLite 存储层。
//
// 设计：包级单例（一进程一 DB）。Init/Close + 全部 r/w 函数为顶级 API；
// 不暴露 *Store 结构给调用方。多实例（多生命体）在 Phase 1+ 重构。
//
// 写权限隔离的强制由调用方约定（state/reflect/lifecycle 等独占各表，docs/TECH-STACK §13.2）。
package storage

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

var (
	db *sql.DB
)

// Init 打开（或创建）SQLite 数据库并应用迁移。
func Init(path string) error {
	if db != nil {
		return errors.New("storage: already initialized")
	}
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)", path)
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	if err := d.Ping(); err != nil {
		_ = d.Close()
		return fmt.Errorf("ping sqlite: %w", err)
	}
	db = d
	if err := migrate(); err != nil {
		_ = db.Close()
		db = nil
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Close 关闭数据库。
func Close() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}

// DB 暴露原始 *sql.DB（仅 storage 包内部使用；外部不应取）。
func DB() *sql.DB { return db }

// ErrNoRows re-export 便于调用方 errors.Is 判断。
var ErrNoRows = sql.ErrNoRows

// migrate 顺序应用 migrations/*.sql。冪等：用 schema_migrations 跟踪。
func migrate() error {
	if _, err := db.Exec(`
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
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE filename = ?`, f).Scan(&applied); err != nil {
			return fmt.Errorf("check applied %s: %w", f, err)
		}
		if applied > 0 {
			continue
		}
		content, err := fs.ReadFile(migrations, filepath.ToSlash("migrations/"+f))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (filename, applied_at) VALUES (?, strftime('%s','now'))`, f); err != nil {
			return fmt.Errorf("mark applied %s: %w", f, err)
		}
	}
	return nil
}
