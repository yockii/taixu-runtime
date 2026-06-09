-- 技能检索精度作一等指标（C5）：每个终态目标记一行「这次注入了哪些技能 / 实际用了几个 /
-- 用到的有几个在注入集内(hit) / 用到却没注入的(miss=recall 缺口)」+ 目标成败。
-- 据此把"检索精度 vs 目标成败"关联起来，data-driven 调 SkillListThreshold（替固定 >8 启发）。
--
-- 纯新建表（对锁定长跑生命安全：不动既有表/CHECK）。append-only，由 action.finalize 在目标终态写。
-- filtered=1 表示当时 ready 技能数 > 阈值、真走了语义 top-k 过滤（此时 miss 才有意义）；
-- filtered=0 表示全列（技能少/无嵌入/检索失败降级），miss 恒 0。
CREATE TABLE IF NOT EXISTS skill_retrieval_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id     TEXT    NOT NULL,
    goal_id     INTEGER NOT NULL,
    ready_total INTEGER NOT NULL DEFAULT 0,  -- 当时 ready 技能总数
    injected    INTEGER NOT NULL DEFAULT 0,  -- 注入 prompt 的技能数（全列时=ready_total）
    filtered    INTEGER NOT NULL DEFAULT 0,  -- 1=走了语义 top-k 过滤；0=全列
    used        INTEGER NOT NULL DEFAULT 0,  -- 本目标实际 use/run 的技能数（去重）
    hit         INTEGER NOT NULL DEFAULT 0,  -- 用到且在注入集内（precision 信号）
    miss        INTEGER NOT NULL DEFAULT 0,  -- 用到却不在注入集（recall 缺口，仅 filtered 可>0）
    success     INTEGER NOT NULL DEFAULT 0,  -- 目标成败
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_retrieval_log_life ON skill_retrieval_log(life_id, created_at);
