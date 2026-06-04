package storage

import "math"

// InterestSeed 兴趣种子（reflex 对话识别到的待探索目标）。
type InterestSeed struct {
	ID            int64   `json:"id"`
	Content       string  `json:"content"`
	Kind          string  `json:"kind"` // skill / knowledge / topic / experience
	Strength      float64 `json:"strength"`
	SourceKind    string  `json:"source_kind"`
	SourceRef     string  `json:"source_ref,omitempty"`
	DecayedAt     int64   `json:"decayed_at,omitempty"`
	CreatedAt     int64   `json:"created_at"`
	LastSeenAt    int64   `json:"last_seen_at"`
	ExploredCount int64   `json:"explored_count"`
}

// UpsertInterestSeed 加入或加权一个兴趣种子。
// 已存在则 strength 累加（封顶 1.0）+ last_seen 推进；不存在则插入。
//
// strength 公式：existing + 0.15 * deltaStrength（避免单轮 LLM 报夸张分数把权重拉满）。
func UpsertInterestSeed(lifeID, content, kind, sourceKind, sourceRef string, addStrength float64, ts int64) error {
	res, err := db.Exec(`
		UPDATE interest_seed
		SET strength = MIN(1.0, strength + ?),
		    last_seen_at = ?,
		    source_kind = COALESCE(NULLIF(source_kind, ''), ?)
		WHERE life_id = ? AND content = ? AND kind = ?`,
		0.15*clampStrength(addStrength), ts, sourceKind, lifeID, content, kind)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	_, err = db.Exec(`
		INSERT INTO interest_seed (life_id, content, kind, strength, source_kind, source_ref, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, content, kind, clampStrength(addStrength), sourceKind, sourceRef, ts, ts)
	return err
}

// ListInterestSeeds 取最高强度的 N 条（供 DriveDerive 用）。
func ListInterestSeeds(lifeID string, minStrength float64, limit int) ([]InterestSeed, error) {
	rows, err := db.Query(`
		SELECT id, content, kind, strength, COALESCE(source_kind,''), COALESCE(source_ref,''),
		       COALESCE(decayed_at,0), created_at, last_seen_at, explored_count
		FROM interest_seed
		WHERE life_id = ? AND strength >= ?
		ORDER BY strength DESC, last_seen_at DESC LIMIT ?`, lifeID, minStrength, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InterestSeed{}
	for rows.Next() {
		var s InterestSeed
		if err := rows.Scan(&s.ID, &s.Content, &s.Kind, &s.Strength,
			&s.SourceKind, &s.SourceRef, &s.DecayedAt,
			&s.CreatedAt, &s.LastSeenAt, &s.ExploredCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListAllInterestSeeds 全表（观察面板用）。
func ListAllInterestSeeds(lifeID string, limit int) ([]InterestSeed, error) {
	return ListInterestSeeds(lifeID, 0.0, limit)
}

// BumpInterestExplored 慎思层探索一次后调，++ explored_count。
func BumpInterestExplored(id int64, ts int64) error {
	_, err := db.Exec(`UPDATE interest_seed SET explored_count = explored_count + 1, last_seen_at = ? WHERE id = ?`, ts, id)
	return err
}

// DecayInterests 按指数衰减：未触及 ≥ thresholdSec 的种子，strength *= dailyFactor^(days)
// 公式简化：每次 cycle 调；按经过时间分段衰减。
func DecayInterests(lifeID string, now int64, halfLifeDays float64) error {
	// half life T_h → factor per day = 0.5^(1/T_h) ≈ exp(-ln2/T_h)
	dailyFactor := math.Exp(-math.Ln2 / halfLifeDays)

	rows, err := db.Query(`
		SELECT id, strength, COALESCE(decayed_at, created_at) FROM interest_seed
		WHERE life_id = ? AND strength > 0.0`, lifeID)
	if err != nil {
		return err
	}
	type item struct {
		id        int64
		strength  float64
		decayedAt int64
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.strength, &it.decayedAt); err != nil {
			_ = rows.Close()
			return err
		}
		items = append(items, it)
	}
	_ = rows.Close()

	const day = float64(24 * 3600)
	for _, it := range items {
		elapsedDays := float64(now-it.decayedAt) / day
		if elapsedDays < 1.0 {
			continue
		}
		newStrength := it.strength * math.Pow(dailyFactor, elapsedDays)
		if newStrength < 0.01 {
			_, _ = db.Exec(`DELETE FROM interest_seed WHERE id = ?`, it.id)
			continue
		}
		_, _ = db.Exec(`UPDATE interest_seed SET strength = ?, decayed_at = ? WHERE id = ?`,
			newStrength, now, it.id)
	}
	return nil
}

func clampStrength(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
