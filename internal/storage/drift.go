package storage

// 价值观漂移仪表（C8）：记录每次价值观权重调整 + 聚合，量化漂移方向/幅度/是否随机游走。
// 见 migration 014。

// InsertValueDrift 追加一条价值观漂移记录（reflect.RunDeep 在每次权重调整后调）。
func InsertValueDrift(lifeID, name string, oldW, newW float64, triggeredBy string, ts int64) error {
	_, err := db.Exec(`
		INSERT INTO value_drift_log (life_id, value_name, old_weight, new_weight, delta, triggered_by, created_at)
		VALUES (?,?,?,?,?,?,?)`,
		lifeID, name, oldW, newW, newW-oldW, nullStr(triggeredBy), ts)
	return err
}

// ValueDrift 单个价值观在某窗内的漂移聚合。
type ValueDrift struct {
	Name     string
	NetDelta float64 // 净位移（往哪个方向、走了多远）
	AbsDelta float64 // 累计绝对位移（动了多少，含来回）
	Changes  int     // 调整次数
}

// Purposefulness 方向一致度 = |净位移| / 累计绝对位移 ∈ [0,1]。
// 接近 1 = 一路朝一个方向（有目的的演化）；接近 0 = 来回抖（随机游走/震荡）。
func (d ValueDrift) Purposefulness() float64 {
	if d.AbsDelta == 0 {
		return 0
	}
	net := d.NetDelta
	if net < 0 {
		net = -net
	}
	return net / d.AbsDelta
}

// ValueDriftSince 汇总自 sinceTs 起本生命各价值观的漂移（sinceTs<=0 取全量）。
func ValueDriftSince(lifeID string, sinceTs int64) ([]ValueDrift, error) {
	rows, err := db.Query(`
		SELECT value_name,
		       COALESCE(SUM(delta),0)      AS net,
		       COALESCE(SUM(ABS(delta)),0) AS abs_total,
		       COUNT(*)                    AS changes
		FROM value_drift_log
		WHERE life_id = ? AND created_at >= ?
		GROUP BY value_name
		ORDER BY abs_total DESC`,
		lifeID, sinceTs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ValueDrift
	for rows.Next() {
		var d ValueDrift
		if err := rows.Scan(&d.Name, &d.NetDelta, &d.AbsDelta, &d.Changes); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
