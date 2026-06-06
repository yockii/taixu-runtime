-- 008_goal_requester.sql
-- 拟人交互闭环（用户 2026-06-06）：为 ExternalRequest 类目标记住"请求者"，
-- 让生命体把"用户托付的研究/慢工"做完后，能主动把成果回送给当初发起的人。
--
-- 背景：
--   - 用户在飞书/网页发"想法/研究请求" → 入队 source='ExternalRequest' 的目标。
--   - 但原 goal_queue 没有 requester 列 → 慎思层做完无从知道"该回给谁"，成果只落本地。
--   - 这里补 req_channel / req_from（均可空——内驱目标无请求者），
--     finalize 完成时据此经 lark.Send(req_from) 主动汇报。
--
-- 可空：仅 ExternalRequest 闭环目标会填；IntrinsicDrive / ReflectionGoal 留 NULL。
-- 不破坏既有行：ALTER ADD COLUMN 默认 NULL，旧目标读出即空，汇报路径自然跳过。

ALTER TABLE goal_queue ADD COLUMN req_channel TEXT;
ALTER TABLE goal_queue ADD COLUMN req_from TEXT;
