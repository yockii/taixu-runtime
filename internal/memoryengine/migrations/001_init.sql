-- Mindverse Phase 0 · SQLite schema v1
-- 依据：docs/02-glossary-and-domain-model.md（领域模型基石）
--       docs/03-life-cycle-and-state-machine.md（状态机）
--       docs/05-memory-architecture.md（四层记忆）
--       docs/06-resource-economics-and-ownership.md（资源账本）
--       docs/TECH-STACK.md §4.2（表清单）
--
-- 写权限隔离（TECH-STACK §13.2）：
--   genesis     -> genome (一次)
--   statemanager-> life_state / mental_state
--   reflection  -> values / reflection_memory
--   memoryengine-> raw_trail / episode / semantic_*
--   lifecyclemgr-> lifecycle_state
--   resourceldg -> resource_ledger
--   goalarb     -> goal_queue
--   actionexec  -> action_log
--   skillreg    -> skill_registry
--   toolrunner  -> tool_audit_log
--
-- v1 不启用：sqlite-vec 扩展、FTS5（留 Phase 0.2）

PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;

-- ============================================================
-- schema 元信息
-- ============================================================
CREATE TABLE IF NOT EXISTS schema_meta (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  INTEGER NOT NULL
);

INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
VALUES ('version', '1', strftime('%s', 'now')),
       ('phase', '0.1', strftime('%s', 'now'));

-- ============================================================
-- Genome · 出生即固定（一次写入，永不修改）
-- ============================================================
CREATE TABLE IF NOT EXISTS genome (
    life_id           TEXT PRIMARY KEY,
    curiosity         REAL NOT NULL CHECK (curiosity      BETWEEN 0.0 AND 1.0),
    sociability       REAL NOT NULL CHECK (sociability    BETWEEN 0.0 AND 1.0),
    creativity        REAL NOT NULL CHECK (creativity     BETWEEN 0.0 AND 1.0),
    persistence       REAL NOT NULL CHECK (persistence    BETWEEN 0.0 AND 1.0),
    risk_taking       REAL NOT NULL CHECK (risk_taking    BETWEEN 0.0 AND 1.0),
    empathy           REAL NOT NULL CHECK (empathy        BETWEEN 0.0 AND 1.0),
    born_at           INTEGER NOT NULL,
    genome_version    TEXT NOT NULL DEFAULT 'v1'
);

-- ============================================================
-- LifeState · 持续变化的生命状态
-- ============================================================
CREATE TABLE IF NOT EXISTS life_state (
    life_id            TEXT PRIMARY KEY REFERENCES genome(life_id),
    energy             REAL NOT NULL CHECK (energy         BETWEEN 0.0 AND 1.0),
    competence         REAL NOT NULL CHECK (competence     BETWEEN 0.0 AND 1.0),
    social_need        REAL NOT NULL CHECK (social_need    BETWEEN 0.0 AND 1.0),
    stress             REAL NOT NULL CHECK (stress         BETWEEN 0.0 AND 1.0),
    confidence         REAL NOT NULL CHECK (confidence     BETWEEN 0.0 AND 1.0),
    stability          REAL NOT NULL CHECK (stability      BETWEEN 0.0 AND 1.0),
    energy_daily_cap   REAL NOT NULL DEFAULT 1.0,
    energy_used_today  REAL NOT NULL DEFAULT 0.0,
    cap_reset_at       INTEGER NOT NULL,
    updated_at         INTEGER NOT NULL
);

-- ============================================================
-- MentalState · 情绪层
-- ============================================================
CREATE TABLE IF NOT EXISTS mental_state (
    life_id        TEXT PRIMARY KEY REFERENCES genome(life_id),
    motivation     REAL NOT NULL CHECK (motivation   BETWEEN 0.0 AND 1.0),
    satisfaction  REAL NOT NULL CHECK (satisfaction BETWEEN 0.0 AND 1.0),
    anxiety        REAL NOT NULL CHECK (anxiety      BETWEEN 0.0 AND 1.0),
    updated_at     INTEGER NOT NULL
);

-- ============================================================
-- Values · 价值观权重表（key/value pairs）
-- Phase 0 仅 ShallowReflect 可读写；不修改权重
-- Phase 2 DeepReflect 启用后才会真正调整
-- ============================================================
CREATE TABLE IF NOT EXISTS life_values (
    life_id     TEXT NOT NULL REFERENCES genome(life_id),
    name        TEXT NOT NULL,
    weight      REAL NOT NULL CHECK (weight BETWEEN 0.0 AND 1.0),
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (life_id, name)
);

-- ============================================================
-- LifecycleState · 宏观状态机（7 态 Phase 0，无 Transferred）
-- ============================================================
CREATE TABLE IF NOT EXISTS lifecycle_state (
    life_id      TEXT PRIMARY KEY REFERENCES genome(life_id),
    state        TEXT NOT NULL CHECK (state IN (
        'Embryonic', 'Active', 'LowPower', 'Dormant',
        'Archived', 'Detached', 'Memorial'
    )),
    entered_at   INTEGER NOT NULL,
    reason       TEXT
);

CREATE TABLE IF NOT EXISTS lifecycle_history (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id      TEXT NOT NULL REFERENCES genome(life_id),
    from_state   TEXT,
    to_state     TEXT NOT NULL,
    transitioned_at INTEGER NOT NULL,
    reason       TEXT
);

CREATE INDEX IF NOT EXISTS idx_lifecycle_history_life
    ON lifecycle_history(life_id, transitioned_at DESC);

-- ============================================================
-- 记忆系统四层（05 文档基石）
-- ============================================================

-- 1) WorkingMemory · 短期工作记忆（每循环可清，但 v1 持久化以便回放）
CREATE TABLE IF NOT EXISTS working_memory (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id      TEXT NOT NULL REFERENCES genome(life_id),
    cycle_id     INTEGER NOT NULL,
    slot         TEXT NOT NULL,
    content      TEXT NOT NULL,
    created_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_wm_cycle ON working_memory(life_id, cycle_id);

-- 2) RawTrail · 事件原始流水
CREATE TABLE IF NOT EXISTS raw_trail (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id      TEXT NOT NULL REFERENCES genome(life_id),
    cycle_id     INTEGER NOT NULL,
    event_type   TEXT NOT NULL,
    payload      TEXT NOT NULL,
    created_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_raw_trail_life_time
    ON raw_trail(life_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_raw_trail_cycle
    ON raw_trail(life_id, cycle_id);

-- 3) Episode · 事件记忆（后台聚合 RawTrail）
CREATE TABLE IF NOT EXISTS episode (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id         TEXT NOT NULL REFERENCES genome(life_id),
    title           TEXT,
    summary         TEXT NOT NULL,
    started_at      INTEGER NOT NULL,
    ended_at        INTEGER NOT NULL,
    raw_start_id    INTEGER,
    raw_end_id      INTEGER,
    salience        REAL NOT NULL DEFAULT 0.5,
    emotion_score   REAL,
    embedding       BLOB,
    created_at      INTEGER NOT NULL,
    sealed_at       INTEGER
);

CREATE INDEX IF NOT EXISTS idx_episode_life_time
    ON episode(life_id, started_at DESC);

-- 4) SemanticCandidate · 候选知识（待固化）
CREATE TABLE IF NOT EXISTS semantic_candidate (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id      TEXT NOT NULL REFERENCES genome(life_id),
    content      TEXT NOT NULL,
    source_ref   TEXT,
    support_count INTEGER NOT NULL DEFAULT 1,
    confidence   REAL NOT NULL DEFAULT 0.5,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sc_life ON semantic_candidate(life_id, last_seen_at DESC);

-- 5) SemanticConfirmed · 固化知识
CREATE TABLE IF NOT EXISTS semantic_confirmed (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id         TEXT NOT NULL REFERENCES genome(life_id),
    content         TEXT NOT NULL,
    confidence      REAL NOT NULL,
    promoted_from   INTEGER REFERENCES semantic_candidate(id),
    embedding       BLOB,
    confirmed_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sk_life ON semantic_confirmed(life_id, confirmed_at DESC);

-- 6) ReflectionMemory · 反思成果
CREATE TABLE IF NOT EXISTS reflection_memory (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id         TEXT NOT NULL REFERENCES genome(life_id),
    kind            TEXT NOT NULL CHECK (kind IN ('Shallow', 'Deep')),
    summary         TEXT NOT NULL,
    insight         TEXT,
    triggered_by    TEXT,
    embedding       BLOB,
    created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reflection_life
    ON reflection_memory(life_id, created_at DESC);

-- ============================================================
-- 目标系统
-- ============================================================
CREATE TABLE IF NOT EXISTS goal_queue (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id         TEXT NOT NULL REFERENCES genome(life_id),
    source          TEXT NOT NULL CHECK (source IN (
        'IntrinsicDrive', 'ExternalRequest', 'ReflectionGoal'
    )),
    intent          TEXT NOT NULL,
    payload         TEXT NOT NULL,
    priority        REAL NOT NULL DEFAULT 0.5,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'active', 'completed', 'rejected', 'expired', 'failed'
    )),
    created_at      INTEGER NOT NULL,
    started_at      INTEGER,
    finished_at     INTEGER,
    arbitration_note TEXT
);

CREATE INDEX IF NOT EXISTS idx_goal_status ON goal_queue(life_id, status, priority DESC);

-- ============================================================
-- 行动日志（ActionExecutor）
-- ============================================================
CREATE TABLE IF NOT EXISTS action_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id         TEXT NOT NULL REFERENCES genome(life_id),
    goal_id         INTEGER REFERENCES goal_queue(id),
    cycle_id        INTEGER NOT NULL,
    plan            TEXT,
    action          TEXT NOT NULL,
    result          TEXT,
    feedback        TEXT,
    success         INTEGER NOT NULL DEFAULT 0,
    started_at      INTEGER NOT NULL,
    finished_at     INTEGER
);

CREATE INDEX IF NOT EXISTS idx_action_log_life ON action_log(life_id, started_at DESC);

-- ============================================================
-- 技能注册
-- ============================================================
CREATE TABLE IF NOT EXISTS skill_registry (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id         TEXT NOT NULL REFERENCES genome(life_id),
    name            TEXT NOT NULL,
    category        TEXT NOT NULL,
    stage           TEXT NOT NULL DEFAULT 'novice' CHECK (stage IN (
        'novice', 'apprentice', 'proficient', 'expert'
    )),
    proficiency     REAL NOT NULL DEFAULT 0.0,
    use_count       INTEGER NOT NULL DEFAULT 0,
    last_used_at    INTEGER,
    registered_at   INTEGER NOT NULL,
    UNIQUE (life_id, name)
);

-- ============================================================
-- 工具审计日志（ToolRunner）
-- ============================================================
CREATE TABLE IF NOT EXISTS tool_audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id         TEXT NOT NULL REFERENCES genome(life_id),
    cycle_id        INTEGER NOT NULL,
    tool_name       TEXT NOT NULL,
    args_summary    TEXT,
    result_summary  TEXT,
    duration_ms     INTEGER,
    success         INTEGER NOT NULL DEFAULT 0,
    error           TEXT,
    started_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tool_audit_life ON tool_audit_log(life_id, started_at DESC);

-- ============================================================
-- 资源账本（Phase 0 仅 energy / knowledge）
-- ============================================================
CREATE TABLE IF NOT EXISTS resource_ledger (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id         TEXT NOT NULL REFERENCES genome(life_id),
    resource        TEXT NOT NULL CHECK (resource IN (
        'energy', 'knowledge', 'wealth', 'reputation', 'social'
    )),
    delta           REAL NOT NULL,
    balance_after   REAL NOT NULL,
    reason          TEXT,
    source_kind     TEXT,
    source_ref      TEXT,
    created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ledger_life_resource
    ON resource_ledger(life_id, resource, created_at DESC);
