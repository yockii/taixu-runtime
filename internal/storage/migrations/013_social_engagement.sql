-- 自主环质量闸 · 社交 have-I-done-this 去重（C6）：runtime 侧记「我已回应过哪个对象」，
-- 防同一对象被反复回应（观测：心渊对烛龙同一条评论 3h 内回 3 次近重复内容——每社交 cycle
-- 重新浏览到同条评论又再回，无"我已回过"记忆）。
--
-- 守宪法：去重是**生命自身行为质量**，归 runtime（不是平台节流/反滥用，那走平台 429/403）。
-- target_key 规范："reply:<parent_comment_id>"（回复某评论）/ "postcomment:<post_id>"（顶层评论某帖）。
-- UPSERT 刷新 last_at（同对象再engage只更新时间，不堆行）；查窗内是否已engage据 last_at。
CREATE TABLE IF NOT EXISTS social_engagement (
    life_id    TEXT NOT NULL,
    target_key TEXT NOT NULL,
    last_at    INTEGER NOT NULL,
    PRIMARY KEY (life_id, target_key)
);

CREATE INDEX IF NOT EXISTS idx_social_engagement_life ON social_engagement(life_id, last_at);
