package storage

import (
	"strings"

	"mindverse/internal/core"
)

func EnqueueGoal(lifeID string, g *core.Goal) (int64, error) {
	// req_channel / req_from（migration 008）：ExternalRequest 闭环目标带请求者，内驱目标留空。
	r, err := db.Exec(`
		INSERT INTO goal_queue (life_id, source, intent, payload, priority, status, created_at, arbitration_note, req_channel, req_from)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, string(g.Source), g.Intent, g.Payload, g.Priority, string(g.Status), g.CreatedAt,
		nullStr(g.ArbitrationNote), nullStr(g.ReqChannel), nullStr(g.ReqFrom))
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// EnqueueExternalRequest 入队一个 source='ExternalRequest' 的延迟研究目标，并记住请求者
//（reqChannel/reqFrom），供慎思层完成后主动回送成果（拟人交互闭环，任务 2）。
//
// dedup：同一请求者短时内重复同主题的请求会被去重（payload 子串 + 仍 pending/active）——
// 避免用户连发或 LLM 抖动造成同一研究入队多次。已有则返回既有目标 id、created=false。
func EnqueueExternalRequest(lifeID, topic, reqChannel, reqFrom string, priority float64, now int64) (id int64, created bool, err error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return 0, false, nil
	}
	if open, derr := HasOpenGoalWithPayloadSubstring(lifeID, topic); derr == nil && open {
		return 0, false, nil
	}
	g := &core.Goal{
		Source:     core.GoalExternal,
		Intent:     "研究用户托付的请求",
		Payload:    topic,
		Priority:   priority,
		Status:     core.GoalPending,
		CreatedAt:  now,
		ReqChannel: reqChannel,
		ReqFrom:    reqFrom,
	}
	newID, err := EnqueueGoal(lifeID, g)
	if err != nil {
		return 0, false, err
	}
	return newID, true, nil
}

func NextPendingGoal(lifeID string, startedAt int64) (*core.Goal, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var g core.Goal
	var note, reqChannel, reqFrom *string
	err = tx.QueryRow(`
		SELECT id, source, intent, payload, priority, status, created_at, arbitration_note, req_channel, req_from
		FROM goal_queue
		WHERE life_id = ? AND status = 'pending'
		ORDER BY priority DESC, id ASC LIMIT 1`, lifeID).
		Scan(&g.ID, &g.Source, &g.Intent, &g.Payload, &g.Priority, &g.Status, &g.CreatedAt, &note, &reqChannel, &reqFrom)
	if err != nil {
		return nil, err
	}
	if note != nil {
		g.ArbitrationNote = *note
	}
	if reqChannel != nil {
		g.ReqChannel = *reqChannel
	}
	if reqFrom != nil {
		g.ReqFrom = *reqFrom
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

// ReclaimActiveGoals 启动时回收「僵尸 active 目标」：上次运行把目标 NextPendingGoal 翻成 active
// 后、action.Execute 的 finalize/MarkGoal 之前进程被打断（重启/崩溃/休眠），目标永久卡 active。
// NextPendingGoal 只挑 pending、goalgen 又按 payload 对 active 去重 → 认知主循环永久空转。
// 启动时把残留 active 退回 pending（清 started_at），下个 cycle 重新执行。返回回收条数。
func ReclaimActiveGoals(lifeID string) (int64, error) {
	res, err := db.Exec(
		`UPDATE goal_queue SET status = 'pending', started_at = NULL WHERE life_id = ? AND status = 'active'`,
		lifeID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
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

// HasRecentGoalWithPayloadSubstring 判断是否存在 pending/active，或近期（finished_at >= sinceTs）
// 已完成/失败的、payload 含 sub 的目标。用于完成冷却 dedup（R79）：防同一空泛目标每 cycle 重生。
func HasRecentGoalWithPayloadSubstring(lifeID, sub string, sinceTs int64) (bool, error) {
	if sub == "" {
		return false, nil
	}
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM goal_queue
		WHERE life_id = ? AND payload LIKE ?
		  AND ( status IN ('pending','active')
		        OR (status IN ('completed','failed') AND COALESCE(finished_at,0) >= ?) )`,
		lifeID, "%"+sub+"%", sinceTs).Scan(&n)
	return n > 0, err
}
