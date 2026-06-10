-- C11 技能发布引导：给 skill_instance 加 published_at（技能发布到生命网络的时间戳，0=未发布）。
--
-- 发布引导 nudge 用此列去重：列「ready + 高掌握度(真成败验证) + published_at=0」的技能，
-- 在社交目标的慎思 prompt 里轻提示生命考虑 social.publish_skill 发布（纯可选、不强制）；
-- 发布成功后置时间戳 → 同技能不再被重复 nudge。
--
-- 纯 ALTER ADD COLUMN（对锁定生命安全：不重建表、不改既有列），默认 0 → 既有技能视为未发布（首次可被引导）。
ALTER TABLE skill_instance ADD COLUMN published_at INTEGER NOT NULL DEFAULT 0;
