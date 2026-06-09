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

// RecordLearning 回写一次学习成果（R77 / R84）：更新 digest + mastery + last_seen。
// 由 deliberative record_learning tool 调。
//
// mastery 改**权威 SET**（不再 MAX-merge）：生命体的自评是对自己掌握程度最真实的判断，
// 它说了算——可以比引擎地板累积的值更低（"我以为懂了，其实没那么透"）。引擎地板
// （BumpInterestExplored）只在生命体不自评时保证非零进度，一旦它开口自评就以自评为准。
// 否则会出现"面板 0.9 但生命体自评 0.65"的割裂（用户 2026-06-05 发现）。
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
		    mastery = ?,
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
//   - mastery 递减收益累积：mastery += masteryDelta·(1-mastery)，封顶 0.95（R84 调整）。
//     每次真探索都沉淀一点，不再靠 LLM 自愿调 record_learning（实测它几乎不调，mastery 永 0）。
//     **递减**（越接近掌握涨得越慢，asymptotic）避免 3 轮粗暴顶到 0.9 而压过生命体自评——
//     原 MIN(0.9, +delta) 会出现"面板 0.9 但生命体自评 0.65"的割裂（用户 2026-06-05 发现）。
//     record_learning 现以生命体自评**权威覆盖**此地板值；引擎只在它不自评时保证非零进度。
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
		    mastery = MIN(0.95, mastery + ? * (1.0 - mastery)),
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
		// 触底不删，保留休眠 ember（dormantFloor）：彻底清退交给 PruneInterests 的总量淘汰，
		// 让淡掉的兴趣仍可被 ReviveDormantInterest 复燃（喜新厌旧之后又回归旧爱）。
		if newStrength < dormantFloor {
			newStrength = dormantFloor
		}
		if _, err := db.Exec(`UPDATE interest_seed SET strength = ?, decayed_at = ? WHERE id = ?`,
			newStrength, now, it.id); err != nil {
			return fmt.Errorf("decay update seed %d: %w", it.id, err)
		}
	}
	return nil
}

// 兴趣生命周期常量（2026-06 用户设计：有上限、喜新厌旧、可复燃、可替换）。
const (
	dormantFloor    = 0.05 // 衰减触底保留的休眠 ember（仍可复燃，不直接删）
	driveThreshold  = 0.4  // ≥ 此值才驱动目标（与 drives.Derive 一致）
	maxInterestRows = 20   // 兴趣总量上限：超了淘汰最弱，腾位给新兴趣（替换）
)

// PruneInterests 总量封顶（替换）：兴趣行数超 maxInterestRows 时，删掉最弱的几条
// （strength 升序、再 last_seen 升序——最弱且最久没碰的先走），但保护当前活跃(≥driveThreshold)的。
// 让「不再感兴趣的」被清退、新兴趣有位置进来。
func PruneInterests(lifeID string, now int64) error {
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM interest_seed WHERE life_id = ?`, lifeID).Scan(&total); err != nil {
		return err
	}
	excess := total - maxInterestRows
	if excess <= 0 {
		return nil
	}
	// 只从非活跃池里淘汰（strength < driveThreshold），避免误删正驱动的兴趣。
	_, err := db.Exec(`
		DELETE FROM interest_seed
		WHERE id IN (
			SELECT id FROM interest_seed
			WHERE life_id = ? AND strength < ?
			ORDER BY strength ASC, last_seen_at ASC
			LIMIT ?
		)`, lifeID, driveThreshold, excess)
	return err
}

// ReviveDormantInterest 复燃：把一个休眠最久的兴趣（dormant 区间、且曾有些掌握/探索过）
// 强度回升到驱动阈值之上，让生命体「过段时间又重拾某个旧兴趣关注点」。返回被复燃的内容（无则 ""）。
func ReviveDormantInterest(lifeID string, now int64) (string, error) {
	var id int64
	var content string
	err := db.QueryRow(`
		SELECT id, content FROM interest_seed
		WHERE life_id = ? AND strength >= ? AND strength < ?
		ORDER BY last_seen_at ASC
		LIMIT 1`, lifeID, dormantFloor, driveThreshold).Scan(&id, &content)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return "", nil
		}
		return "", err
	}
	// 回升到阈值之上（0.5），并刷新 last_seen / decayed_at（重新计时）。
	if _, err := db.Exec(`
		UPDATE interest_seed SET strength = 0.5, last_seen_at = ?, decayed_at = ? WHERE id = ?`,
		now, now, id); err != nil {
		return "", err
	}
	return content, nil
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
