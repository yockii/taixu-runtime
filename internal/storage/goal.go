package storage

import "mindverse/internal/core"

func EnqueueGoal(lifeID string, g *core.Goal) (int64, error) {
	r, err := db.Exec(`
		INSERT INTO goal_queue (life_id, source, intent, payload, priority, status, created_at, arbitration_note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, string(g.Source), g.Intent, g.Payload, g.Priority, string(g.Status), g.CreatedAt, nullStr(g.ArbitrationNote))
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func NextPendingGoal(lifeID string, startedAt int64) (*core.Goal, error) {
	tx, err := db.Begin()
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

func MarkGoal(goalID int64, status core.GoalStatus, finishedAt int64) error {
	_, err := db.Exec(`UPDATE goal_queue SET status = ?, finished_at = ? WHERE id = ?`,
		string(status), finishedAt, goalID)
	return err
}

func CountPendingGoals(lifeID string) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM goal_queue WHERE life_id = ? AND status = 'pending'`, lifeID).Scan(&n)
	return n, err
}

// CountActiveOrPendingGoals 队列内"在飞"目标数：active + pending 总和。
// 用于 goal.Arbitrate 控制 backlog（R75）。
func CountActiveOrPendingGoals(lifeID string) (int, error) {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM goal_queue
		WHERE life_id = ? AND status IN ('pending','active')`, lifeID).Scan(&n)
	return n, err
}

// HasOpenGoalWithPayloadSubstring 判断是否存在 pending/active 且 payload 含 sub 的目标。
// 用于 dedup：interest_seed#N 已在飞时不重复派发（R74 / R75）。
func HasOpenGoalWithPayloadSubstring(lifeID, sub string) (bool, error) {
	if sub == "" {
		return false, nil
	}
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM goal_queue
		WHERE life_id = ? AND status IN ('pending','active') AND payload LIKE ?`,
		lifeID, "%"+sub+"%").Scan(&n)
	return n > 0, err
}
