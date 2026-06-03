package memoryengine

// AppendToolAudit 写一条工具调用审计。仅 ToolRunner 应调用。
func (s *Store) AppendToolAudit(lifeID string, cycleID int64, toolName, argsSummary, resultSummary string,
	durationMs int64, success bool, errStr string, startedAt int64) error {
	succ := 0
	if success {
		succ = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO tool_audit_log (life_id, cycle_id, tool_name, args_summary, result_summary, duration_ms, success, error, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, cycleID, toolName, nullStr(argsSummary), nullStr(resultSummary), durationMs, succ, nullStr(errStr), startedAt)
	return err
}
