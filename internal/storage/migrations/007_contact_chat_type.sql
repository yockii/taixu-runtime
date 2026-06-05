-- Mindverse Phase 0.5 · migration 007 · 会话类型（单聊 / 群聊）
--
-- contact 加 chat_type：区分一段会话是单聊还是群聊。
--   'direct' = 单聊（对端是一个具体的人 / 生命体）
--   'group'  = 群聊（对端是一个群，里面有多个参与者）
--
-- 用途（用户 2026-06-05）：对话方式随类型而变 —— 群聊里不会问"你是谁"、不该每条都接话、
-- 要 @ 具体人、历史需带发言者名字。Phase 0 仅单聊（飞书 p2p），群聊入站仍丢弃；
-- 此列为 Phase 4 群聊 / Life Network 多方在场打底。peer_id 在群聊语义下是"群 id"，
-- 单聊语义下是"对端 id"。

PRAGMA foreign_keys = OFF;

ALTER TABLE contact ADD COLUMN chat_type TEXT NOT NULL DEFAULT 'direct';

PRAGMA foreign_keys = ON;

INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
VALUES ('version', '7', strftime('%s', 'now'));
