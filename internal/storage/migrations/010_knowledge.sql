-- 010_knowledge.sql
-- 知识库（用户 2026-06-06）：根研究目标完成时，把它自己 + 整棵子树的成果摘要综合成一篇
-- 结构化 dossier（标题 + 正文），按主题/研究组织、可经 /api/knowledge 浏览。
--
-- 与语义记忆的分工：
--   semantic_confirmed 是细碎、向量可检索的「知识点」（query_memory 用）；
--   knowledge_entry 是按一次完整研究聚合的「篇章式档案」（人读 / API 浏览用）。
--   二者互补，沉淀同时写两边（sedimentToSemantic 仍照旧跑）。
--
-- 纯新建表，不动任何既有表，对锁定生命零影响。
CREATE TABLE IF NOT EXISTS knowledge_entry (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id      TEXT NOT NULL,
    root_goal_id INTEGER,                 -- 综合自哪个根研究目标（可空，手工注入的留空）
    topic        TEXT NOT NULL,           -- dossier 标题 / 主题
    body         TEXT NOT NULL,           -- dossier 正文（结论 + 要点 + 来源）
    source_kind  TEXT NOT NULL DEFAULT 'research',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_knowledge_life_created ON knowledge_entry(life_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_root_goal ON knowledge_entry(root_goal_id);
