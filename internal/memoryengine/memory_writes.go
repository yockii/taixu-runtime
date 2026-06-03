package memoryengine

import (
	"mindverse/internal/core"
)

// AppendWorking 持久化一条 WorkingMemory 槽。
func (s *Store) AppendWorking(lifeID string, cycleID int64, slot, content string, ts int64) (int64, error) {
	r, err := s.db.Exec(`
		INSERT INTO working_memory (life_id, cycle_id, slot, content, created_at)
		VALUES (?, ?, ?, ?, ?)`, lifeID, cycleID, slot, content, ts)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// ListRecentRawTrail 取最近 N 条 RawTrail（按 created_at desc）。
func (s *Store) ListRecentRawTrail(lifeID string, limit int) ([]core.RawTrailEntry, error) {
	rows, err := s.db.Query(`
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

// RawTrailSinceID 取 lastSealedID 之后的所有 raw_trail（Episode 聚合用）。
func (s *Store) RawTrailSinceID(lifeID string, lastID int64) ([]core.RawTrailEntry, error) {
	rows, err := s.db.Query(`
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

// InsertEpisode 写入一条 Episode。
func (s *Store) InsertEpisode(lifeID string, e *core.Episode) (int64, error) {
	r, err := s.db.Exec(`
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

// LatestEpisodeRawEndID 返回该生命体最近一条 Episode 包到的最大 raw_id（用于增量聚合）。
func (s *Store) LatestEpisodeRawEndID(lifeID string) (int64, error) {
	var v *int64
	err := s.db.QueryRow(`SELECT MAX(raw_end_id) FROM episode WHERE life_id = ?`, lifeID).Scan(&v)
	if err != nil {
		return 0, err
	}
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

// UpsertSemanticCandidate 加入或加权候选（按 content 完全相同合并 support_count）。
func (s *Store) UpsertSemanticCandidate(lifeID, content, sourceRef string, ts int64) error {
	res, err := s.db.Exec(`
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
	_, err = s.db.Exec(`
		INSERT INTO semantic_candidate (life_id, content, source_ref, support_count, confidence, created_at, last_seen_at)
		VALUES (?, ?, ?, 1, 0.5, ?, ?)`, lifeID, content, sourceRef, ts, ts)
	return err
}

// ListCandidatesAboveConfidence 列举高置信度候选，供 Reflection 浅审固化。
func (s *Store) ListCandidatesAboveConfidence(lifeID string, threshold float64, limit int) ([]core.SemanticCandidate, error) {
	rows, err := s.db.Query(`
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

// PromoteToConfirmed 把候选固化为 SemanticConfirmed。事务原子。
func (s *Store) PromoteToConfirmed(lifeID string, candidateID int64, content string, confidence float64, ts int64) error {
	tx, err := s.db.Begin()
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

// InsertReflection 写入一条反思成果。
func (s *Store) InsertReflection(lifeID string, m *core.ReflectionMemory) (int64, error) {
	r, err := s.db.Exec(`
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
