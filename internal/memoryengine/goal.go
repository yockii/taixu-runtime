package memoryengine

import "mindverse/internal/core"

// EnqueueGoal 入队一个目标（GoalArbitrator 仲裁后调用）。
func (s *Store) EnqueueGoal(lifeID string, g *core.Goal) (int64, error) {
	r, err := s.db.Exec(`
		INSERT INTO goal_queue (life_id, source, intent, payload, priority, status, created_at, arbitration_note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, string(g.Source), g.Intent, g.Payload, g.Priority, string(g.Status), g.CreatedAt, nullStr(g.ArbitrationNote))
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// NextPendingGoal 取下一个最高优先级 pending 目标，置为 active。
func (s *Store) NextPendingGoal(lifeID string, startedAt int64) (*core.Goal, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var g core.Goal
	var note *string
	err = tx.QueryRow(`
		SELECT id, source, intent, payload, priority, status, created_at, arbitration_note
		FROM goal_queue
		WHERE life_id = ? AND status = 'pending'
		ORDER BY priority DESC, id ASC LIMIT 1`, lifeID).
		Scan(&g.ID, &g.Source, &g.Intent, &g.Payload, &g.Priority, &g.Status, &g.CreatedAt, &note)
	if err != nil {
		return nil, err
	}
	if note != nil {
		g.ArbitrationNote = *note
	}
	if _, err := tx.Exec(`UPDATE goal_queue SET status = 'active', started_at = ? WHERE id = ?`, startedAt, g.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	g.Status = core.GoalActive
	g.StartedAt = startedAt
	return &g, nil
}

// MarkGoal 设置目标终态（completed / failed / rejected / expired）。
func (s *Store) MarkGoal(goalID int64, status core.GoalStatus, finishedAt int64) error {
	_, err := s.db.Exec(`UPDATE goal_queue SET status = ?, finished_at = ? WHERE id = ?`,
		string(status), finishedAt, goalID)
	return err
}

// CountPendingGoals 当前 pending 数。
func (s *Store) CountPendingGoals(lifeID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM goal_queue WHERE life_id = ? AND status = 'pending'`, lifeID).Scan(&n)
	return n, err
}
