-- Mindverse Phase 0.5 · migration 004 · 知识掌握度（R77 地基）+ Skill 系统（D.2）
--
-- 改动：
--   1. interest_seed 加 digest（学习摘要）+ mastery（自评掌握度 0-1）
--      → drives.Derive 纳入 mastery 衰减；record_learning tool 回写。
--   2. 新表 skill_instance：装载的 SKILL.md 种子实例化为有状态对象（mastery/使用记录）
--   3. 新表 skill_dependency：skill 装的依赖包审计（包名/版本/hash/装载方式）
--      详见 docs/SKILLS-AND-TOOLS §3 §5。

PRAGMA foreign_keys = OFF;

-- 1) interest_seed 知识掌握度（R77）
ALTER TABLE interest_seed ADD COLUMN digest TEXT;
ALTER TABLE interest_seed ADD COLUMN mastery REAL NOT NULL DEFAULT 0.0
    CHECK (mastery BETWEEN 0.0 AND 1.0);

-- 2) skill_instance（D.2）— SKILL.md 种子在本生命体内的有状态实例
CREATE TABLE IF NOT EXISTS skill_instance (
    id            TEXT PRIMARY KEY,             -- seed_hash 或 uuid
    life_id       TEXT NOT NULL REFERENCES genome(life_id),
    name          TEXT NOT NULL,
    seed_ref      TEXT NOT NULL,                -- SKILL.md 内容 sha256
    seed_version  TEXT,
    description   TEXT,
    lanes         TEXT,                         -- JSON: ["reflex","deliberative"]
    allowed_tools TEXT,                         -- JSON: ["web.fetch",...]
    status        TEXT NOT NULL DEFAULT 'pending_approval'
                  CHECK (status IN ('pending_approval','installing','ready','disabled','failed')),
    pending_deps  TEXT,                         -- JSON: 待批准依赖列表
    mastery       REAL NOT NULL DEFAULT 0.0 CHECK (mastery BETWEEN 0.0 AND 1.0),
    used_count    INTEGER NOT NULL DEFAULT 0,
    last_used_at  INTEGER,
    install_path  TEXT,                         -- /skills/<id>/
    created_at    INTEGER NOT NULL,
    UNIQUE (life_id, name)
);

CREATE INDEX IF NOT EXISTS idx_skill_instance_status
    ON skill_instance(life_id, status);

-- 3) skill_dependency（D.2）— append-only 依赖装载审计
CREATE TABLE IF NOT EXISTS skill_dependency (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id     TEXT NOT NULL REFERENCES skill_instance(id),
    runtime      TEXT NOT NULL CHECK (runtime IN ('python','node')),
    package      TEXT NOT NULL,
    version      TEXT NOT NULL,
    install_hash TEXT,
    installed_by TEXT,                          -- 'user_approve' / 'auto_approve' / 'bundle'
    installed_at INTEGER NOT NULL,
    UNIQUE (skill_id, runtime, package)
);

PRAGMA foreign_keys = ON;

INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
VALUES ('version', '4', strftime('%s', 'now'));
