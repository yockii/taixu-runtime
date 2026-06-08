-- 技能描述向量（B 的延伸 / 技能按需装载）：给 skill_instance 加 embedding 列，
-- 用于「按当前目标语义检索最相关的 top-k 技能」注入慎思 prompt，而非每 cycle 列全部技能
-- （技能一多 token 线性膨胀、多数与当前目标无关）。复用既有嵌入设施（embed + 暴力 cosine）。
--
-- 纯 ALTER ADD COLUMN（对锁定长跑生命安全：不重建表、不改 CHECK）。embedding 为空 → 该技能
-- 退回「全列」兜底，绝不因无向量而消失。
ALTER TABLE skill_instance ADD COLUMN embedding BLOB;
