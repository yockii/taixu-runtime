-- Mindverse Phase 0.5 · migration 006 · skill 血缘（知识→skill 结晶）
--
-- skill_instance 加 authored_from：记录该 skill 的来源。
--   空 / NULL  = 外部投放（用户放进 workspace/skills 或粘贴）
--   "interest_seed#N" = 生命体把自己学透的知识结晶成的 skill（self-authored）
--
-- 用途：UI 标记"自创"徽章；Phase 4 社群传授时溯源（这技能是它自己悟出来的还是学来的）。

PRAGMA foreign_keys = OFF;

ALTER TABLE skill_instance ADD COLUMN authored_from TEXT;

PRAGMA foreign_keys = ON;

INSERT OR REPLACE INTO schema_meta (key, value, updated_at)
VALUES ('version', '6', strftime('%s', 'now'));
