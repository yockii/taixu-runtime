-- 009_goal_tree.sql
-- 递归研究目标树（用户 2026-06-06）：让慎思层把一个大目标拆成子目标、子目标全完后
-- 回到母目标综合，形成「研究 → 子研究 → 回归综合」的递归结构。
--
-- 安全铁律（锁定长跑观察生命 local-1b844e59 的历史目标行必须零损）：
--   只用纯 ALTER ADD COLUMN，绝不重建 goal_queue、绝不动 status CHECK 枚举。
--   SQLite 的 ALTER ADD COLUMN 是 O(1) 元数据操作，不重写既有行、不触发表重建——
--   旧目标行读出时新列即取默认值（parent_id/result_digest=NULL，depth/pending_children=0），
--   语义上等价「根目标、深度 0、无未结子目标」，与旧行为完全一致，故零损。
--
-- 「阻塞」不新增 status 值（避免改 CHECK 触发 SQLite 重建表的风险），改用累加计数列表达：
--   母目标「被阻塞」 == status='pending' AND pending_children > 0。
--   NextPendingGoal 只挑 pending AND pending_children=0 → 被阻塞的母目标不会被选中执行，
--   直到它的子目标全部完成把 pending_children 减到 0，母目标自然恢复可执行。

-- 父指针：子目标指向母目标 id；根目标（最初的研究目标）为 NULL。
ALTER TABLE goal_queue ADD COLUMN parent_id INTEGER;

-- 递归深度：根=0，每下一层 +1。用于 MaxResearchDepth 护栏，防无限拆解。
ALTER TABLE goal_queue ADD COLUMN depth INTEGER NOT NULL DEFAULT 0;

-- 成果摘要：该目标完成（或中间拆解）时写一段成果/进度摘要，供母目标回归综合 + 知识库 dossier。
ALTER TABLE goal_queue ADD COLUMN result_digest TEXT;

-- 未结子目标计数：>0 即「被阻塞」。enqueue_subgoal 建子目标时母 +1；
-- 子目标 MarkGoal(completed/failed) 时母 -1；减到 0 母目标解阻塞、下个 cycle 被重选。
ALTER TABLE goal_queue ADD COLUMN pending_children INTEGER NOT NULL DEFAULT 0;

-- 按父查子（回归时拉子目标 result_digest）+ 按根聚合（知识库 dossier 综合整棵树）。
CREATE INDEX IF NOT EXISTS idx_goal_parent ON goal_queue(parent_id);
