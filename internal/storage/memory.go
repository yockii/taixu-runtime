package storage

import "mindverse/internal/core"

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
