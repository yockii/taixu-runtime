package storage

func AppendActionLog(lifeID string, goalID int64, cycleID int64, plan, action, result, feedback string,
	success bool, startedAt, finishedAt int64) error {
	succ := 0
	if success {
		succ = 1
	}
	_, err := db.Exec(`
		INSERT INTO action_log (life_id, goal_id, cycle_id, plan, action, result, feedback, success, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, nullInt(goalID), cycleID,
		nullStr(plan), action, nullStr(result), nullStr(feedback),
		succ, startedAt, finishedAt)
	return err
}

func AppendToolAudit(lifeID string, cycleID int64, toolName, argsSummary, resultSummary string,
	durationMs int64, success bool, errStr string, startedAt int64) error {
	succ := 0
	if success {
		succ = 1
	}
	_, err := db.Exec(`
		INSERT INTO tool_audit_log (life_id, cycle_id, tool_name, args_summary, result_summary, duration_ms, success, error, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, cycleID, toolName, nullStr(argsSummary), nullStr(resultSummary), durationMs, succ, nullStr(errStr), startedAt)
	return err
}
