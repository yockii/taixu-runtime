-- 生命体在世经济财富 $WEALTH（C10 slice1）：给 life_state 加 wealth + social_wealth_today。
--
-- wealth = 白皮书 5 资源之一（06 §1），生命体主权内部货币。**非 [0,1] 标量**——无界累积，floor 0。
-- 社交活动产出（06 §3.1.2）、未来技能/物品交易流通（06 §7）。与平台星屑物理隔离、不可兑换（06 §3.5 / R110）。
-- social_wealth_today = 当日社交产 wealth 累计，反刷递减用（R109），随日精力上限每日清零。
--
-- 纯 ALTER ADD COLUMN（对锁定生命安全：不重建表、不改 CHECK），默认 0 → 既有生命无缝获得初始 0 财富。
-- 两 ALTER 包事务原子化：ALTER ADD COLUMN 非幂等（无 IF NOT EXISTS），若崩溃于两者间、migration 未标
-- applied 而第一列已加 → 重跑撞「duplicate column」。BEGIN/COMMIT 保证「两列全加 or 全不加」。
BEGIN;
ALTER TABLE life_state ADD COLUMN wealth REAL NOT NULL DEFAULT 0;
ALTER TABLE life_state ADD COLUMN social_wealth_today REAL NOT NULL DEFAULT 0;
COMMIT;
