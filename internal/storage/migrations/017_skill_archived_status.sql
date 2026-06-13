-- Mindverse · migration 017 · 修 skill_instance CHECK 约束漏 'archived'
--
-- Bug：004 建表时 status CHECK 只允许 ('pending_approval','installing','ready','disabled','failed')，
-- 但 R88 引入的「遗忘衰减/归档」功能（loader.go decay + ListSkillsByStatus/ReactivateSkill）用 'archived'
-- 状态——归档 UPDATE 恒触发 CHECK 失败，每分钟刷 WARN「constraint failed: status IN (...)」，
-- 衰减机制实际坏掉（技能永不归档、无法被遗忘后重激活）。
-- SQLite 不支持 ALTER CHECK → 重建表（保留全部数据）。CHECK 补入 'archived'。
--
-- 注：FK 由 applyMigration 在事务外置 OFF（事务内 PRAGMA foreign_keys 为 no-op），本文件不写 PRAGMA。
-- 列集 = 004 基础 + 006 authored_from + 011 embedding + 016 published_at。

CREATE TABLE skill_instance_new (
    id            TEXT PRIMARY KEY,
    life_id       TEXT NOT NULL REFERENCES genome(life_id),
    name          TEXT NOT NULL,
    seed_ref      TEXT NOT NULL,
    seed_version  TEXT,
    description   TEXT,
    lanes         TEXT,
    allowed_tools TEXT,
    status        TEXT NOT NULL DEFAULT 'pending_approval'
                  CHECK (status IN ('pending_approval','installing','ready','disabled','failed','archived')),
    pending_deps  TEXT,
    mastery       REAL NOT NULL DEFAULT 0.0 CHECK (mastery BETWEEN 0.0 AND 1.0),
    used_count    INTEGER NOT NULL DEFAULT 0,
    last_used_at  INTEGER,
    install_path  TEXT,
    created_at    INTEGER NOT NULL,
    authored_from TEXT,
    embedding     BLOB,
    published_at  INTEGER NOT NULL DEFAULT 0,
    UNIQUE (life_id, name)
);

INSERT INTO skill_instance_new
    (id, life_id, name, seed_ref, seed_version, description, lanes, allowed_tools,
     status, pending_deps, mastery, used_count, last_used_at, install_path, created_at,
     authored_from, embedding, published_at)
SELECT
     id, life_id, name, seed_ref, seed_version, description, lanes, allowed_tools,
     status, pending_deps, mastery, used_count, last_used_at, install_path, created_at,
     authored_from, embedding, published_at
FROM skill_instance;

DROP TABLE skill_instance;
ALTER TABLE skill_instance_new RENAME TO skill_instance;

CREATE INDEX IF NOT EXISTS idx_skill_instance_status ON skill_instance(life_id, status);

INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
VALUES ('version', '17', strftime('%s', 'now'));
