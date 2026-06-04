-- Mindverse Phase 0 · migration 002
--
-- 修 R65：semantic_confirmed.promoted_from 是 FK REFERENCES semantic_candidate(id)，
-- 配合 PromoteToConfirmed 内 "INSERT confirmed → DELETE candidate" 的事务，COMMIT 时
-- FK 检查失败 → 静默 rollback。43 次 ShallowReflect 全部未实际固化。
--
-- 处理：重建 semantic_confirmed，去掉 FK，promoted_from 退化为信息列（保留谱系但不强引用）。
--
-- 修 R66：ExtractSemantic v1 重复扫描固定窗口，无位置游标 → support_count 虚高。
-- 加 schema_meta 主键不变，新增运行时 cursor key（不需 schema 改）。
-- 代码层用 GetMeta/SetMeta 维护 last_semantic_extract_raw_id。

PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS semantic_confirmed_new (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id         TEXT NOT NULL REFERENCES genome(life_id),
    content         TEXT NOT NULL,
    confidence      REAL NOT NULL,
    promoted_from   INTEGER, -- 仅信息列，不再 REFERENCES semantic_candidate
    embedding       BLOB,
    confirmed_at    INTEGER NOT NULL
);

INSERT INTO semantic_confirmed_new (id, life_id, content, confidence, promoted_from, embedding, confirmed_at)
SELECT id, life_id, content, confidence, promoted_from, embedding, confirmed_at FROM semantic_confirmed;

DROP TABLE semantic_confirmed;
ALTER TABLE semantic_confirmed_new RENAME TO semantic_confirmed;

CREATE INDEX IF NOT EXISTS idx_sk_life ON semantic_confirmed(life_id, confirmed_at DESC);

PRAGMA foreign_keys = ON;

INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
VALUES ('version', '2', strftime('%s', 'now'));
