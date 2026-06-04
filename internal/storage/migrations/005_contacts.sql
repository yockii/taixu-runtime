-- Mindverse Phase 0.5 · migration 005 · 社交联系人（A: 对话↔社交联动 / B: 主动发消息前提）
--
-- contact：生命体对话过的对象。Phase 0 仅 IM（飞书）+ cli/web 注入。
--
-- 前瞻（Phase 4 联网生态）：
--   届时社交渠道远不止 IM —— Life Network、世界服务（学校/图书馆）、其他生命体。
--   生命体将**自主决策去哪参与社交**（不只被动等消息）。彼时 contact 应扩展为更通用的
--   "关系/在场"模型（peer 可以是用户、其他生命体、世界服务实体），channel 扩为多渠道枚举，
--   并接入 reputation / social 资源（06）与 Encounter/Relationship/Pact（07 §3）。
--   Phase 0 先记最小集：谁、哪个渠道、聊了多少、最近何时。

CREATE TABLE IF NOT EXISTS contact (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id     TEXT NOT NULL REFERENCES genome(life_id),
    channel     TEXT NOT NULL,                 -- 'feishu' / 'cli' / 'web'（Phase 4 扩多渠道）
    peer_id     TEXT NOT NULL,                 -- 对方标识（飞书 open_id / cli 空串归一为 'local'）
    peer_name   TEXT,                          -- 展示名（可选）
    msg_count   INTEGER NOT NULL DEFAULT 0,    -- 累计交互条数
    first_at    INTEGER NOT NULL,
    last_at     INTEGER NOT NULL,
    UNIQUE (life_id, channel, peer_id)
);

CREATE INDEX IF NOT EXISTS idx_contact_recent
    ON contact(life_id, last_at DESC);

INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
VALUES ('version', '5', strftime('%s', 'now'));
