-- 价值观漂移仪表（C8）：每次深反思调整一条价值观权重就记一行（旧→新+增量+触发源）。
-- 据此量化漂移：累计 |delta|（动得多不多）、净 delta（往哪个方向）、净/绝对比（方向一致=有目的演化，
-- 来回抖=随机游走）。配合地板告警（核心价值衰到近零=人格侵蚀）让"价值观演化"从黑箱变可观测。
--
-- 纯新建表（对锁定生命安全）。append-only，由 reflect.RunDeep 在每次 UpsertValue 成功后写。
CREATE TABLE IF NOT EXISTS value_drift_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    life_id      TEXT    NOT NULL,
    value_name   TEXT    NOT NULL,
    old_weight   REAL    NOT NULL,
    new_weight   REAL    NOT NULL,
    delta        REAL    NOT NULL,
    triggered_by TEXT,
    created_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_value_drift_life ON value_drift_log(life_id, created_at);
