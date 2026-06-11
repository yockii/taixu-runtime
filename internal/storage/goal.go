package storage

import (
	"strings"

	"taixu.icu/runtime/internal/core"
)

func EnqueueGoal(lifeID string, g *core.Goal) (int64, error) {
	// req_channel / req_from（migration 008）：ExternalRequest 闭环目标带请求者，内驱目标留空。
	// parent_id / depth / result_digest / pending_children（migration 009）：递归研究目标树。
	//   新建目标 pending_children 恒为 0（刚入队还没拆子目标）；母目标的计数由 IncPendingChildren 维护。
	//   ParentID==0 写 NULL（根目标）；result_digest 入队时一般为空。
	r, err := db.Exec(`
		INSERT INTO goal_queue
		  (life_id, source, intent, payload, priority, status, created_at, arbitration_note,
		   req_channel, req_from, parent_id, depth, result_digest)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, string(g.Source), g.Intent, g.Payload, g.Priority, string(g.Status), g.CreatedAt,
		nullStr(g.ArbitrationNote), nullStr(g.ReqChannel), nullStr(g.ReqFrom),
		nullInt(g.ParentID), g.Depth, nullStr(g.ResultDigest))
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
	if dupID, derr := openGoalIDWithPayloadSubstring(lifeID, topic); derr == nil && dupID != 0 {
		// 判重命中：返回既有目标的真实 id（而非 0），上层可据此接力/关联请求者。
		return dupID, false, nil
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
	var note, reqChannel, reqFrom, digest *string
	var parentID *int64
	// 阻塞语义（migration 009）：被阻塞的母目标 == status='pending' AND pending_children>0。
	// 此处 WHERE 排除 pending_children>0，让被阻塞的母目标不会被选中执行——直到它的子目标
	// 全部完成把 pending_children 减回 0，下个 cycle 才会自然重新选中它（回归综合）。
	err = tx.QueryRow(`
		SELECT id, source, intent, payload, priority, status, created_at, arbitration_note, req_channel, req_from,
		       parent_id, COALESCE(depth,0), result_digest, COALESCE(pending_children,0)
		FROM goal_queue
		WHERE life_id = ? AND status = 'pending' AND COALESCE(pending_children,0) = 0
		ORDER BY priority DESC, id ASC LIMIT 1`, lifeID).
		Scan(&g.ID, &g.Source, &g.Intent, &g.Payload, &g.Priority, &g.Status, &g.CreatedAt, &note, &reqChannel, &reqFrom,
			&parentID, &g.Depth, &digest, &g.PendingChildren)
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
	if parentID != nil {
		g.ParentID = *parentID
	}
	if digest != nil {
		g.ResultDigest = *digest
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

// MarkGoal 把一个目标置为终态（completed/failed/...）。
//
// 递归研究目标树（migration 009）解阻塞不变量：
//
//	若该目标是子目标（parent_id 非空）且被置为「终态」（completed 或 failed——研究路径只有这两种），
//	则母目标的 pending_children 自动 -1（不减到负）。母 pending_children 减到 0 时，它就从
//	「pending 且 children>0（被阻塞）」变成「pending 且 children=0（可执行）」，下个 cycle 被
//	NextPendingGoal 重选 → 母目标恢复执行、综合子成果。这是「子完成→解阻塞母→回归」的唯一减点。
//
// 整个操作在一个事务里完成：先读子目标的 parent_id，再更新子状态，再减母计数——保证
// 「子转终态」与「母计数 -1」原子，避免并发/崩溃下计数与实际未结子目标数偏离。
func MarkGoal(goalID int64, status core.GoalStatus, finishedAt int64) error {
	terminal := status == core.GoalCompleted || status == core.GoalFailed ||
		status == core.GoalRejected || status == core.GoalExpired

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var parentID *int64
	if terminal {
		// 读旧 parent_id；同时确认目标存在。
		if err := tx.QueryRow(`SELECT parent_id FROM goal_queue WHERE id = ?`, goalID).Scan(&parentID); err != nil {
			if err == ErrNoRows {
				// 目标不存在：照旧 best-effort（与旧行为一致，UPDATE 影响 0 行）。
				parentID = nil
			} else {
				return err
			}
		}
	}

	// 幂等守卫：终态化只允许从 pending/active 出发。重复对同一目标终态化（重试/并发）时
	// UPDATE 影响 0 行 → 跳过母计数扣减等一切副作用，避免 parent pending_children 被多减。
	query := `UPDATE goal_queue SET status = ?, finished_at = ? WHERE id = ?`
	if terminal {
		query += ` AND status IN ('pending','active')`
	}
	res, err := tx.Exec(query, string(status), finishedAt, goalID)
	if err != nil {
		return err
	}
	if terminal {
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			// 目标不存在或已是终态：无副作用，直接提交（no-op）。
			return tx.Commit()
		}
	}

	if terminal && parentID != nil {
		// 母 pending_children -1（地板 0，绝不为负——防计数漂移把母目标永久卡阻塞）。
		if _, err := tx.Exec(
			`UPDATE goal_queue SET pending_children = MAX(0, COALESCE(pending_children,0) - 1) WHERE id = ?`,
			*parentID); err != nil {
			return err
		}
	}
	return tx.Commit()
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

// -----------------------------------------------------------------------------
// 递归研究目标树（migration 009）查询 / 维护
// -----------------------------------------------------------------------------

// GetGoalByID 读单个目标全字段（含树字段）。不存在返回 (nil, nil)。
func GetGoalByID(goalID int64) (*core.Goal, error) {
	var g core.Goal
	var note, reqChannel, reqFrom, digest *string
	var parentID *int64
	var started, finished *int64
	err := db.QueryRow(`
		SELECT id, source, intent, payload, priority, status, created_at,
		       started_at, finished_at, arbitration_note, req_channel, req_from,
		       parent_id, COALESCE(depth,0), result_digest, COALESCE(pending_children,0)
		FROM goal_queue WHERE id = ?`, goalID).
		Scan(&g.ID, &g.Source, &g.Intent, &g.Payload, &g.Priority, &g.Status, &g.CreatedAt,
			&started, &finished, &note, &reqChannel, &reqFrom,
			&parentID, &g.Depth, &digest, &g.PendingChildren)
	if err == ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if started != nil {
		g.StartedAt = *started
	}
	if finished != nil {
		g.FinishedAt = *finished
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
	if parentID != nil {
		g.ParentID = *parentID
	}
	if digest != nil {
		g.ResultDigest = *digest
	}
	return &g, nil
}

// IncPendingChildren 母目标未结子目标计数 +1（enqueue_subgoal 建一个子目标时调）。
// 这是 pending_children 唯一的增点——与 MarkGoal 的减点（子转终态 -1）成对维护计数不变量。
func IncPendingChildren(parentID int64) error {
	_, err := db.Exec(
		`UPDATE goal_queue SET pending_children = COALESCE(pending_children,0) + 1 WHERE id = ?`, parentID)
	return err
}

// SetResultDigest 写某目标的成果/进度摘要（完成或中间拆解时）。供母目标回归综合 + 知识库 dossier。
func SetResultDigest(goalID int64, digest string) error {
	_, err := db.Exec(`UPDATE goal_queue SET result_digest = ? WHERE id = ?`, nullStr(digest), goalID)
	return err
}

// SetGoalPending 把目标置回 pending（清 finished_at）。
//
// 用于「母目标本次执行期间拆出了子目标 → 不完成、回到 pending 等子目标」：此时母 pending_children>0，
// 置回 pending 后即满足「pending 且 children>0 == 被阻塞」，NextPendingGoal 不会选中它，直到
// 子目标全完。注意不动 pending_children（IncPendingChildren 已在建子目标时加过）。
func SetGoalPending(goalID int64) error {
	_, err := db.Exec(
		`UPDATE goal_queue SET status = 'pending', finished_at = NULL WHERE id = ?`, goalID)
	return err
}

// ListChildren 列某母目标的直接子目标（按 id 升序，即创建顺序）。回归综合时拉各子 result_digest。
func ListChildren(parentID int64) ([]core.Goal, error) {
	rows, err := db.Query(`
		SELECT id, source, intent, payload, priority, status, created_at,
		       COALESCE(depth,0), COALESCE(result_digest,''), COALESCE(pending_children,0)
		FROM goal_queue WHERE parent_id = ? ORDER BY id ASC`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []core.Goal{}
	for rows.Next() {
		var g core.Goal
		if err := rows.Scan(&g.ID, &g.Source, &g.Intent, &g.Payload, &g.Priority, &g.Status, &g.CreatedAt,
			&g.Depth, &g.ResultDigest, &g.PendingChildren); err != nil {
			return nil, err
		}
		g.ParentID = parentID
		out = append(out, g)
	}
	return out, rows.Err()
}

// CountIncompleteChildren 母目标尚未结的子目标数（status 仍 pending/active）。
// 与 pending_children 计数互为校验：正常情况下二者一致。供测试 / 诊断用。
func CountIncompleteChildren(parentID int64) (int, error) {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM goal_queue
		WHERE parent_id = ? AND status IN ('pending','active')`, parentID).Scan(&n)
	return n, err
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
// 用 instr 做纯子串匹配——LIKE 拼接不转义 %/_ 时，sub 含 % 会通配匹配任意目标（误吞托付）。
func HasOpenGoalWithPayloadSubstring(lifeID, sub string) (bool, error) {
	if sub == "" {
		return false, nil
	}
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM goal_queue
		WHERE life_id = ? AND status IN ('pending','active') AND instr(payload, ?) > 0`,
		lifeID, sub).Scan(&n)
	return n > 0, err
}

// openGoalIDWithPayloadSubstring 返回最早一条 pending/active 且 payload 含 sub 的目标 id；
// 无命中返回 0。供 EnqueueExternalRequest 判重命中时把既有目标 id 真实返回给上层接力。
func openGoalIDWithPayloadSubstring(lifeID, sub string) (int64, error) {
	if sub == "" {
		return 0, nil
	}
	var id int64
	err := db.QueryRow(`
		SELECT id FROM goal_queue
		WHERE life_id = ? AND status IN ('pending','active') AND instr(payload, ?) > 0
		ORDER BY id ASC LIMIT 1`,
		lifeID, sub).Scan(&id)
	if err == ErrNoRows {
		return 0, nil
	}
	return id, err
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
		WHERE life_id = ? AND instr(payload, ?) > 0
		  AND ( status IN ('pending','active')
		        OR (status IN ('completed','failed') AND COALESCE(finished_at,0) >= ?) )`,
		lifeID, sub, sinceTs).Scan(&n)
	return n > 0, err
}
