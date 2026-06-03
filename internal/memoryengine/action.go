package memoryengine

// AppendActionLog 写一条 ActionExecutor 行动记录。
func (s *Store) AppendActionLog(lifeID string, goalID int64, cycleID int64, plan, action, result, feedback string,
	success bool, startedAt, finishedAt int64) error {
	succ := 0
	if success {
		succ = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO action_log (life_id, goal_id, cycle_id, plan, action, result, feedback, success, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, nullInt(goalID), cycleID,
		nullStr(plan), action, nullStr(result), nullStr(feedback),
		succ, startedAt, finishedAt)
	return err
}
