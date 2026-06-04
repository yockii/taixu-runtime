package storage

// 行动日志 kind 常量。
const (
	ActionKindDeliberate   = "deliberate"   // 慎思层（GoalArbitrator → ActionExecutor）
	ActionKindReflex       = "reflex"       // 反射层（LLM 响应对话）
	ActionKindReflexCanned = "reflex_canned" // 反射层敷衍模式（低能量/高焦虑短路）
)

// AppendActionLog 慎思路径写入。kind 固定 deliberate（兼容旧调用）。
func AppendActionLog(lifeID string, goalID int64, cycleID int64, plan, action, result, feedback string,
	success bool, startedAt, finishedAt int64) error {
	return AppendActionLogKind(lifeID, goalID, cycleID, ActionKindDeliberate,
		plan, action, result, feedback, success, startedAt, finishedAt)
}

// AppendActionLogKind 通用写入，kind 显式。
func AppendActionLogKind(lifeID string, goalID int64, cycleID int64, kind string,
	plan, action, result, feedback string, success bool, startedAt, finishedAt int64) error {
	succ := 0
	if success {
		succ = 1
	}
	_, err := db.Exec(`
		INSERT INTO action_log (life_id, goal_id, cycle_id, kind, plan, action, result, feedback, success, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, nullInt(goalID), cycleID, kind,
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
