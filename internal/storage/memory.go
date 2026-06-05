package storage

import (
	"encoding/json"

	"mindverse/internal/core"
)

// DialogueTurn 一轮对话（用户或生命体的一句话），供 reflex 载入历史上下文。
type DialogueTurn struct {
	Role    string // "user"（用户）/ "assistant"（生命体）
	Content string
	At      int64
}

// RecentDialogueTurns 取最近 limit 轮对话（按时间正序），从 raw_trail 的
// reflex.received（用户）/ reflex.speak（生命体）事件重建。
// 给 reflex 注入近期会话历史，避免大模型回复有失忆感（用户 2026-06-05）。
func RecentDialogueTurns(lifeID string, limit int) ([]DialogueTurn, error) {
	rows, err := db.Query(`
		SELECT event_type, payload, created_at FROM raw_trail
		WHERE life_id = ? AND event_type IN ('reflex.received','reflex.speak')
		ORDER BY id DESC LIMIT ?`, lifeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rev []DialogueTurn
	for rows.Next() {
		var et, payload string
		var at int64
		if err := rows.Scan(&et, &payload, &at); err != nil {
			return nil, err
		}
		var p struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal([]byte(payload), &p)
		if p.Content == "" {
			continue
		}
		role := "assistant"
		if et == "reflex.received" {
			role = "user"
		}
		rev = append(rev, DialogueTurn{Role: role, Content: p.Content, At: at})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 反转为时间正序
	out := make([]DialogueTurn, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return out, nil
}

func AppendWorking(lifeID string, cycleID int64, slot, content string, ts int64) (int64, error) {
	r, err := db.Exec(`
		INSERT INTO working_memory (life_id, cycle_id, slot, content, created_at)
		VALUES (?, ?, ?, ?, ?)`, lifeID, cycleID, slot, content, ts)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func ListRecentRawTrail(lifeID string, limit int) ([]core.RawTrailEntry, error) {
	rows, err := db.Query(`
		SELECT id, cycle_id, event_type, payload, created_at
		FROM raw_trail WHERE life_id = ?
		ORDER BY created_at DESC LIMIT ?`, lifeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.RawTrailEntry
	for rows.Next() {
		var e core.RawTrailEntry
		if err := rows.Scan(&e.ID, &e.CycleID, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func RawTrailSinceID(lifeID string, lastID int64) ([]core.RawTrailEntry, error) {
	rows, err := db.Query(`
		SELECT id, cycle_id, event_type, payload, created_at
		FROM raw_trail WHERE life_id = ? AND id > ?
		ORDER BY id ASC`, lifeID, lastID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.RawTrailEntry
	for rows.Next() {
		var e core.RawTrailEntry
		if err := rows.Scan(&e.ID, &e.CycleID, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func InsertEpisode(lifeID string, e *core.Episode) (int64, error) {
	r, err := db.Exec(`
		INSERT INTO episode (life_id, title, summary, started_at, ended_at,
		                    raw_start_id, raw_end_id, salience, emotion_score, embedding, created_at, sealed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, nullStr(e.Title), e.Summary, e.StartedAt, e.EndedAt,
		nullInt(e.RawStartID), nullInt(e.RawEndID), e.Salience, nullFloat(e.EmotionScore),
		e.Embedding, e.CreatedAt, nullInt(e.SealedAt))
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func LatestEpisodeRawEndID(lifeID string) (int64, error) {
	var v *int64
	err := db.QueryRow(`SELECT MAX(raw_end_id) FROM episode WHERE life_id = ?`, lifeID).Scan(&v)
	if err != nil {
		return 0, err
	}
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

func UpsertSemanticCandidate(lifeID, content, sourceRef string, ts int64) error {
	res, err := db.Exec(`
		UPDATE semantic_candidate
		SET support_count = support_count + 1,
		    last_seen_at = ?,
		    confidence = MIN(1.0, confidence + 0.1)
		WHERE life_id = ? AND content = ?`, ts, lifeID, content)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	_, err = db.Exec(`
		INSERT INTO semantic_candidate (life_id, content, source_ref, support_count, confidence, created_at, last_seen_at)
		VALUES (?, ?, ?, 1, 0.5, ?, ?)`, lifeID, content, sourceRef, ts, ts)
	return err
}

func ListCandidatesAboveConfidence(lifeID string, threshold float64, limit int) ([]core.SemanticCandidate, error) {
	rows, err := db.Query(`
		SELECT id, content, source_ref, support_count, confidence, created_at, last_seen_at
		FROM semantic_candidate WHERE life_id = ? AND confidence >= ?
		ORDER BY confidence DESC LIMIT ?`, lifeID, threshold, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.SemanticCandidate
	for rows.Next() {
		var c core.SemanticCandidate
		var srcRef *string
		if err := rows.Scan(&c.ID, &c.Content, &srcRef, &c.SupportCount, &c.Confidence, &c.CreatedAt, &c.LastSeenAt); err != nil {
			return nil, err
		}
		if srcRef != nil {
			c.SourceRef = *srcRef
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListSemanticConfirmed 查询已固化语义记忆；q 非空时模糊匹配 content。
func ListSemanticConfirmed(lifeID, q string, limit int) ([]core.SemanticConfirmed, error) {
	args := []any{lifeID}
	where := "life_id = ?"
	if q != "" {
		where += " AND content LIKE ?"
		args = append(args, "%"+q+"%")
	}
	args = append(args, limit)
	rows, err := db.Query(`
		SELECT id, content, confidence, COALESCE(promoted_from,0), confirmed_at
		FROM semantic_confirmed WHERE `+where+`
		ORDER BY confirmed_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.SemanticConfirmed
	for rows.Next() {
		var c core.SemanticConfirmed
		if err := rows.Scan(&c.ID, &c.Content, &c.Confidence, &c.PromotedFrom, &c.ConfirmedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func PromoteToConfirmed(lifeID string, candidateID int64, content string, confidence float64, ts int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`
		INSERT INTO semantic_confirmed (life_id, content, confidence, promoted_from, confirmed_at)
		VALUES (?, ?, ?, ?, ?)`, lifeID, content, confidence, candidateID, ts); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM semantic_candidate WHERE id = ?`, candidateID); err != nil {
		return err
	}
	return tx.Commit()
}

func InsertReflection(lifeID string, m *core.ReflectionMemory) (int64, error) {
	r, err := db.Exec(`
		INSERT INTO reflection_memory (life_id, kind, summary, insight, triggered_by, embedding, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		lifeID, string(m.Kind), m.Summary, nullStr(m.Insight), nullStr(m.TriggeredBy), m.Embedding, m.CreatedAt)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullFloat(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}
