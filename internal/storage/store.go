// Package storage 单例 SQLite 存储层。
//
// 设计：包级单例（一进程一 DB）。Init/Close + 全部 r/w 函数为顶级 API；
// 不暴露 *Store 结构给调用方。多实例（多生命体）在 Phase 1+ 重构。
//
// 写权限隔离的强制由调用方约定（state/reflect/lifecycle 等独占各表，docs/TECH-STACK §13.2）。
package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// db 是 *sqlx.DB：内嵌 *sql.DB，故既有 Query/Exec/QueryRow 原样可用，
// 同时提供 Get/Select 按 struct `db:"col"` tag 扫描，省去手写 Scan 列表（用户 2026-06-05 选型）。
// 迁移仍走 migrations/*.sql 文件（可审 / 带设计理由 / 可回滚）——不引入 ORM AutoMigrate。
var db *sqlx.DB

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
	db = sqlx.NewDb(d, "sqlite")
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
func DB() *sql.DB { return db.DB }

// SnapshotInto 用 VACUUM INTO 产出一致的库快照到 path（WAL 已合并，无需停写）。
// path 必须不存在（VACUUM INTO 要求目标为新文件）。供 lifepack 导出取一致镜像。
func SnapshotInto(path string) error {
	_, err := db.Exec("VACUUM INTO ?", path)
	return err
}

// ErrNoRows re-export 便于调用方 errors.Is 判断。
var ErrNoRows = sql.ErrNoRows

// migrate 顺序应用 migrations/*.sql。冪等：用 schema_migrations 跟踪。
//
// 原子性（库即生命终身记忆，半写态不可恢复——崩溃后 ALTER 重放撞 duplicate column /
// 表重建中断 no such table 都是永久启动失败）：每个迁移在 Go 侧单事务执行——
// BEGIN → 迁移文件全部语句 → INSERT schema_migrations → COMMIT。崩溃则整体回滚，
// 重启从干净态重放。为此：
//
//  1. 迁移文件内自带的 BEGIN;/COMMIT; 在执行前剥除（如 015），避免与外层事务嵌套冲突。
//  2. PRAGMA foreign_keys 在事务内是 no-op（SQLite 规定）——文件内的该 PRAGMA 一并剥除，
//     改为在事务外、同一连接上统一 OFF（覆盖 002/003/004/006/007 的表重建），COMMIT 后恢复 ON。
//  3. 事务与 PRAGMA 必须落在同一物理连接上才有效，故用 db.Conn 钉住单连接执行全程。
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

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("pin migration connection: %w", err)
	}
	defer conn.Close()

	for _, f := range files {
		var applied int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE filename = ?`, f).Scan(&applied); err != nil {
			return fmt.Errorf("check applied %s: %w", f, err)
		}
		if applied > 0 {
			continue
		}
		content, err := fs.ReadFile(migrations, filepath.ToSlash("migrations/"+f))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if err := applyMigration(ctx, conn, f, sanitizeMigration(string(content))); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration 在钉住的连接上以单事务应用一个迁移并标记 schema_migrations。
// FK 开关在事务外（事务内 PRAGMA foreign_keys 是 no-op），恢复 ON 用 defer 保证错误路径也复位。
func applyMigration(ctx context.Context, conn *sql.Conn, name, body string) (retErr error) {
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("fk off for %s: %w", name, err)
	}
	defer func() {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil && retErr == nil {
			retErr = fmt.Errorf("fk on after %s: %w", name, err)
		}
	}()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (filename, applied_at) VALUES (?, strftime('%s','now'))`, name); err != nil {
		return fmt.Errorf("mark applied %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}
	return nil
}

// sanitizeMigration 剥除迁移文件里与「Go 侧外层事务」冲突的整行语句：
//   - BEGIN; / BEGIN TRANSACTION; / COMMIT; —— 嵌套事务报错（015 自带）。
//   - PRAGMA foreign_keys ...; —— 事务内是 no-op，由 applyMigration 在事务外统一管理。
//
// 只匹配「独占一行且以分号结尾」的语句，CREATE TRIGGER 体内的裸 BEGIN（无分号）不受影响。
func sanitizeMigration(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		norm := strings.ToUpper(strings.Join(strings.Fields(line), " "))
		switch {
		case norm == "BEGIN;", norm == "BEGIN TRANSACTION;", norm == "COMMIT;":
			continue
		case strings.HasPrefix(norm, "PRAGMA FOREIGN_KEYS") && strings.HasSuffix(norm, ";"):
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
