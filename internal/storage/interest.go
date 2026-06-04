package storage

import (
	"fmt"
	"math"
)

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
	Digest        string  `json:"digest,omitempty"`  // 学习摘要（R77）
	Mastery       float64 `json:"mastery"`           // 自评掌握度 0-1（R77）
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
		       COALESCE(decayed_at,0), created_at, last_seen_at, explored_count,
		       COALESCE(digest,''), mastery
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
			&s.CreatedAt, &s.LastSeenAt, &s.ExploredCount,
			&s.Digest, &s.Mastery); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetInterestSeed 按 id 取单个兴趣种子。未找到返 (nil, nil)。
func GetInterestSeed(id int64) (*InterestSeed, error) {
	var s InterestSeed
	err := db.QueryRow(`
		SELECT id, content, kind, strength, COALESCE(source_kind,''), COALESCE(source_ref,''),
		       COALESCE(decayed_at,0), created_at, last_seen_at, explored_count,
		       COALESCE(digest,''), mastery
		FROM interest_seed WHERE id = ?`, id).
		Scan(&s.ID, &s.Content, &s.Kind, &s.Strength, &s.SourceKind, &s.SourceRef,
			&s.DecayedAt, &s.CreatedAt, &s.LastSeenAt, &s.ExploredCount, &s.Digest, &s.Mastery)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// RecordLearning 回写一次学习成果（R77）：更新 digest + mastery（取较大 mastery，
// 学习只增不减掌握度）+ last_seen。由 deliberative record_learning tool 调。
func RecordLearning(id int64, digest string, mastery float64, ts int64) error {
	if mastery < 0 {
		mastery = 0
	}
	if mastery > 1 {
		mastery = 1
	}
	_, err := db.Exec(`
		UPDATE interest_seed
		SET digest = ?,
		    mastery = MAX(mastery, ?),
		    last_seen_at = ?
		WHERE id = ?`, digest, mastery, ts, id)
	return err
}

// ListAllInterestSeeds 全表（观察面板用）。
func ListAllInterestSeeds(lifeID string, limit int) ([]InterestSeed, error) {
	return ListInterestSeeds(lifeID, 0.0, limit)
}

// BumpInterestExplored 慎思层成功探索一次后调（引擎权威，不依赖 LLM 自觉调 record_learning）。
//
// 副作用（R83 修复"学不会→零技能"）：
//   - explored_count++
//   - mastery = MIN(0.9, mastery + masteryDelta)  引擎给掌握度地板：每次真探索都沉淀一点，
//     不再靠 LLM 自愿调 record_learning（实测 39 次工具调用里它一次没调，mastery 永 0）。
//     record_learning 仍可 MAX-merge 拔高到更准的值；引擎管下限，LLM 校上限。
//   - last_seen_at = ts
//
// 注意：**不再衰减 strength**。原 -0.15/次让 strength 两轮就跌破 0.4 门槛，
// 兴趣"没学会就先腻了"死掉，mastery 永远到不了 0.8 结晶门槛 → 零技能。
// 反重复改由 drives.Derive 的 exploreFactor(1/(1+0.3·count)) + masteryFactor(1-mastery)
// 节流优先级；学透后由 RetireInterestSeed 显式退役。
func BumpInterestExplored(id int64, masteryDelta float64, ts int64) error {
	if masteryDelta < 0 {
		masteryDelta = 0
	}
	_, err := db.Exec(`
		UPDATE interest_seed
		SET explored_count = explored_count + 1,
		    mastery = MIN(0.9, mastery + ?),
		    last_seen_at = ?
		WHERE id = ?`, masteryDelta, ts, id)
	return err
}

// RetireInterestSeed 学透并结晶后退役一个兴趣种子：strength 砍到 0.1（< drives 的 0.4 门槛），
// 使其退出慎思派生，不再反复刷同一已掌握的兴趣。保留行（digest/mastery 仍可查/可被对话重燃）。
func RetireInterestSeed(id int64, ts int64) error {
	_, err := db.Exec(`
		UPDATE interest_seed
		SET strength = MIN(strength, 0.1),
		    last_seen_at = ?
		WHERE id = ?`, ts, id)
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
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
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
			if _, err := db.Exec(`DELETE FROM interest_seed WHERE id = ?`, it.id); err != nil {
				return fmt.Errorf("decay delete seed %d: %w", it.id, err)
			}
			continue
		}
		if _, err := db.Exec(`UPDATE interest_seed SET strength = ?, decayed_at = ? WHERE id = ?`,
			newStrength, now, it.id); err != nil {
			return fmt.Errorf("decay update seed %d: %w", it.id, err)
		}
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
