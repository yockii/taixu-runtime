-- Mindverse Phase 0.5 · migration 003 · Reflex 通道 + 兴趣种子
--
-- 改动：
--   1. action_log 加 kind 列，区分 deliberate（慎思）/ reflex（反射）/ reflex_canned（敷衍）
--      旧行 kind 默认 'deliberate'。
--   2. 新表 interest_seed：reflex 对话中识别的兴趣点，后续被 DriveCuriosity 派生为内驱
--   3. 新表 reflex_log（可选，Phase 0.5 暂用 action_log + kind 区分；保留扩展位）

PRAGMA foreign_keys = OFF;

-- 1) action_log.kind
ALTER TABLE action_log ADD COLUMN kind TEXT NOT NULL DEFAULT 'deliberate'
    CHECK (kind IN ('deliberate', 'reflex', 'reflex_canned'));

CREATE INDEX IF NOT EXISTS idx_action_log_kind
    ON action_log(life_id, kind, started_at DESC);

-- 2) interest_seed
CREATE TABLE IF NOT EXISTS interest_seed (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id      TEXT NOT NULL REFERENCES genome(life_id),
    content      TEXT NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('skill', 'knowledge', 'topic', 'experience')),
    strength     REAL NOT NULL DEFAULT 0.5 CHECK (strength BETWEEN 0.0 AND 1.0),
    source_kind  TEXT,                       -- 'reflex' / 'reflect' / 'external'
    source_ref   TEXT,                       -- 来源 raw_trail id 或 action_log id 字符串
    decayed_at   INTEGER,                    -- 上次衰减时间
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    explored_count INTEGER NOT NULL DEFAULT 0, -- 慎思层探索次数
    UNIQUE (life_id, content, kind)
);

CREATE INDEX IF NOT EXISTS idx_interest_seed_strength
    ON interest_seed(life_id, strength DESC, last_seen_at DESC);

PRAGMA foreign_keys = ON;

INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
VALUES ('version', '3', strftime('%s', 'now'));
