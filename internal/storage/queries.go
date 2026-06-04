// storage 查询函数（Phase 0.4 观察面板用）。
package storage

import "mindverse/internal/core"

// ListEpisodes 取近 N 段（按 started_at desc）。q 非空时模糊匹配 summary/title。
func ListEpisodes(lifeID, q string, limit, offset int) ([]core.Episode, error) {
	args := []any{lifeID}
	where := "life_id = ?"
	if q != "" {
		where += " AND (summary LIKE ? OR title LIKE ?)"
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	args = append(args, limit, offset)
	rows, err := db.Query(`
		SELECT id, COALESCE(title,''), summary, started_at, ended_at,
		       COALESCE(raw_start_id,0), COALESCE(raw_end_id,0),
		       salience, COALESCE(emotion_score,0), created_at, COALESCE(sealed_at,0)
		FROM episode WHERE `+where+`
		ORDER BY started_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []core.Episode{}
	for rows.Next() {
		var e core.Episode
		if err := rows.Scan(&e.ID, &e.Title, &e.Summary, &e.StartedAt, &e.EndedAt,
			&e.RawStartID, &e.RawEndID, &e.Salience, &e.EmotionScore, &e.CreatedAt, &e.SealedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListGoals 按 status 过滤；空 status 返回全部。limit 限上限。
func ListGoals(lifeID, status string, limit int) ([]core.Goal, error) {
	args := []any{lifeID}
	where := "life_id = ?"
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	args = append(args, limit)
	rows, err := db.Query(`
		SELECT id, source, intent, payload, priority, status,
		       created_at, COALESCE(started_at,0), COALESCE(finished_at,0),
		       COALESCE(arbitration_note,'')
		FROM goal_queue WHERE `+where+`
		ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []core.Goal{}
	for rows.Next() {
		var g core.Goal
		if err := rows.Scan(&g.ID, &g.Source, &g.Intent, &g.Payload, &g.Priority, &g.Status,
			&g.CreatedAt, &g.StartedAt, &g.FinishedAt, &g.ArbitrationNote); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ListReflections 近 N 条反思。
func ListReflections(lifeID string, limit int) ([]core.ReflectionMemory, error) {
	rows, err := db.Query(`
		SELECT id, kind, summary, COALESCE(insight,''), COALESCE(triggered_by,''), created_at
		FROM reflection_memory WHERE life_id = ?
		ORDER BY id DESC LIMIT ?`, lifeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []core.ReflectionMemory{}
	for rows.Next() {
		var r core.ReflectionMemory
		if err := rows.Scan(&r.ID, &r.Kind, &r.Summary, &r.Insight, &r.TriggeredBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ToolAuditEntry 工具调用审计一行。
type ToolAuditEntry struct {
	ID            int64  `json:"id"`
	CycleID       int64  `json:"cycle_id"`
	ToolName      string `json:"tool_name"`
	ArgsSummary   string `json:"args_summary"`
	ResultSummary string `json:"result_summary"`
	DurationMs    int64  `json:"duration_ms"`
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	StartedAt     int64  `json:"started_at"`
}

// ListToolAudit 近 N 条工具调用。
func ListToolAudit(lifeID string, limit int) ([]ToolAuditEntry, error) {
	rows, err := db.Query(`
		SELECT id, cycle_id, tool_name, COALESCE(args_summary,''), COALESCE(result_summary,''),
		       COALESCE(duration_ms,0), success, COALESCE(error,''), started_at
		FROM tool_audit_log WHERE life_id = ?
		ORDER BY id DESC LIMIT ?`, lifeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ToolAuditEntry{}
	for rows.Next() {
		var e ToolAuditEntry
		var succ int
		if err := rows.Scan(&e.ID, &e.CycleID, &e.ToolName, &e.ArgsSummary, &e.ResultSummary,
			&e.DurationMs, &succ, &e.Error, &e.StartedAt); err != nil {
			return nil, err
		}
		e.Success = succ == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

// LedgerEntry 账本一行。
type LedgerEntry struct {
	ID           int64   `json:"id"`
	Resource     string  `json:"resource"`
	Delta        float64 `json:"delta"`
	BalanceAfter float64 `json:"balance_after"`
	Reason       string  `json:"reason"`
	SourceKind   string  `json:"source_kind"`
	SourceRef    string  `json:"source_ref"`
	CreatedAt    int64   `json:"created_at"`
}

// ListLedger 按 resource 过滤；空返回全部。
func ListLedger(lifeID, resource string, limit int) ([]LedgerEntry, error) {
	args := []any{lifeID}
	where := "life_id = ?"
	if resource != "" {
		where += " AND resource = ?"
		args = append(args, resource)
	}
	args = append(args, limit)
	rows, err := db.Query(`
		SELECT id, resource, delta, balance_after, COALESCE(reason,''),
		       COALESCE(source_kind,''), COALESCE(source_ref,''), created_at
		FROM resource_ledger WHERE `+where+`
		ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LedgerEntry{}
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.ID, &e.Resource, &e.Delta, &e.BalanceAfter, &e.Reason,
			&e.SourceKind, &e.SourceRef, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ActionLogEntry 行动日志一行。
type ActionLogEntry struct {
	ID         int64  `json:"id"`
	GoalID     int64  `json:"goal_id"`
	CycleID    int64  `json:"cycle_id"`
	Plan       string `json:"plan"`
	Action     string `json:"action"`
	Result     string `json:"result"`
	Feedback   string `json:"feedback"`
	Success    bool   `json:"success"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
}

// ListActionLog 近 N 条行动。
func ListActionLog(lifeID string, limit int) ([]ActionLogEntry, error) {
	rows, err := db.Query(`
		SELECT id, COALESCE(goal_id,0), cycle_id, COALESCE(plan,''), action,
		       COALESCE(result,''), COALESCE(feedback,''), success,
		       started_at, COALESCE(finished_at,0)
		FROM action_log WHERE life_id = ?
		ORDER BY id DESC LIMIT ?`, lifeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ActionLogEntry{}
	for rows.Next() {
		var e ActionLogEntry
		var succ int
		if err := rows.Scan(&e.ID, &e.GoalID, &e.CycleID, &e.Plan, &e.Action,
			&e.Result, &e.Feedback, &succ, &e.StartedAt, &e.FinishedAt); err != nil {
			return nil, err
		}
		e.Success = succ == 1
		out = append(out, e)
	}
	return out, rows.Err()
}
