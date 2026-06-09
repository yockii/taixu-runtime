package storage

import (
	"encoding/json"

	"taixu.icu/runtime/internal/core"
)

// DialogueTurn 一轮对话（用户或生命体的一句话），供 reflex 载入历史上下文 + 面板展示。
type DialogueTurn struct {
	Role    string `json:"role"` // "user"（用户）/ "assistant"（生命体）
	Content string `json:"content"`
	At      int64  `json:"at"`
}

// RecentDialogueTurns 取最近 limit 轮对话（按时间正序，跨全部会话），从 raw_trail 的
// reflex.received（用户）/ reflex.speak / reflex.proactive_reach（生命体）事件重建。
// 给面板做全局观察用。reflex 注入对话历史 / 主动消息须用 RecentDialogueTurnsForConvo 按会话隔离。
func RecentDialogueTurns(lifeID string, limit int) ([]DialogueTurn, error) {
	return dialogueTurns(lifeID, "", "", false, limit)
}

// RecentDialogueTurnsForConvo 取某一会话（channel + peer）的最近 limit 轮对话（按时间正序）。
//
// 关键（用户 2026-06-05）：未来多渠道（飞书/钉钉/slack）多会话并存，对话历史与"主动发了几条没回"
// 的计数必须**按会话隔离**——给 A 发了 1 条，不能因为给 B 发过而说成"我发了 2 条怎么没回"。
// 这里按事件 payload 的 channel + 对端（received 看 from / speak·proactive 看 to）过滤。
func RecentDialogueTurnsForConvo(lifeID, channel, peer string, limit int) ([]DialogueTurn, error) {
	return dialogueTurns(lifeID, channel, peerKey(peer), true, limit)
}

// dialogueTurns 内部实现：filterConvo=false 取全部；true 仅取 (channel, peer) 会话。
func dialogueTurns(lifeID, channel, peer string, filterConvo bool, limit int) ([]DialogueTurn, error) {
	// 过滤会话时事件经 payload 筛掉，故先多取一些候选再截断。
	scan := limit
	if filterConvo {
		scan = limit * 8
		if scan < 64 {
			scan = 64
		}
	}
	rows, err := db.Query(`
		SELECT event_type, payload, created_at FROM raw_trail
		WHERE life_id = ? AND event_type IN ('reflex.received','reflex.speak','reflex.proactive_reach')
		ORDER BY id DESC LIMIT ?`, lifeID, scan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rev []DialogueTurn
	for rows.Next() {
		var et, payload string
		var at int64
		if err := rows.Scan(&et, &payload, &at); err != nil {
			return nil, err
		}
		var p struct {
			Content string `json:"content"`
			Channel string `json:"channel"`
			From    string `json:"from"`
			To      string `json:"to"`
		}
		_ = json.Unmarshal([]byte(payload), &p)
		if p.Content == "" {
			continue
		}
		role := "assistant"
		evPeer := p.To // speak / proactive_reach：对端是收件人
		if et == "reflex.received" {
			role = "user"
			evPeer = p.From // received：对端是发件人
		}
		if filterConvo && (p.Channel != channel || peerKey(evPeer) != peer) {
			continue
		}
		rev = append(rev, DialogueTurn{Role: role, Content: p.Content, At: at})
		if len(rev) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 反转为时间正序
	out := make([]DialogueTurn, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return out, nil
}

func AppendWorking(lifeID string, cycleID int64, slot, content string, ts int64) (int64, error) {
	r, err := db.Exec(`
		INSERT INTO working_memory (life_id, cycle_id, slot, content, created_at)
		VALUES (?, ?, ?, ?, ?)`, lifeID, cycleID, slot, content, ts)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func ListRecentRawTrail(lifeID string, limit int) ([]core.RawTrailEntry, error) {
	rows, err := db.Query(`
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

func RawTrailSinceID(lifeID string, lastID int64) ([]core.RawTrailEntry, error) {
	rows, err := db.Query(`
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

func InsertEpisode(lifeID string, e *core.Episode) (int64, error) {
	r, err := db.Exec(`
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

func LatestEpisodeRawEndID(lifeID string) (int64, error) {
	var v *int64
	err := db.QueryRow(`SELECT MAX(raw_end_id) FROM episode WHERE life_id = ?`, lifeID).Scan(&v)
	if err != nil {
		return 0, err
	}
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

// UpsertSemanticCandidate 复现 +1 支持度；初见以默认置信 0.5 入库。
// 用于"重复模式"路径（extractor:v2 同一 tool.success 反复出现才升置信）。
func UpsertSemanticCandidate(lifeID, content, sourceRef string, ts int64) error {
	return UpsertSemanticCandidateConf(lifeID, content, sourceRef, ts, 0.5)
}

// UpsertSemanticCandidateConf 同上，但初见置信由调用方给定。
//
// 修语义固化链断点：record_learning 写入的 digest 每次都是不同长文，永远走 INSERT 新行、
// 卡在死值 0.5 < 0.75 promote 阈值 → 永不固化（sem_confirmed 恒 0）。改由来源 seed 的 mastery
// 作初见置信：学透的知识 digest 直接达阈值，经 ShallowReflect 沉淀进 semantic_confirmed；
// 浅学的 digest 仍留候选区，待掌握加深后的新 digest 再够格。固化的"反思闸"语义不变。
func UpsertSemanticCandidateConf(lifeID, content, sourceRef string, ts int64, initialConf float64) error {
	if initialConf < 0 {
		initialConf = 0
	}
	if initialConf > 1 {
		initialConf = 1
	}
	res, err := db.Exec(`
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
	_, err = db.Exec(`
		INSERT INTO semantic_candidate (life_id, content, source_ref, support_count, confidence, created_at, last_seen_at)
		VALUES (?, ?, ?, 1, ?, ?, ?)`, lifeID, content, sourceRef, initialConf, ts, ts)
	return err
}

func ListCandidatesAboveConfidence(lifeID string, threshold float64, limit int) ([]core.SemanticCandidate, error) {
	rows, err := db.Query(`
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

// ListSemanticConfirmed 查询已固化语义记忆；q 非空时模糊匹配 content。
func ListSemanticConfirmed(lifeID, q string, limit int) ([]core.SemanticConfirmed, error) {
	args := []any{lifeID}
	where := "life_id = ?"
	if q != "" {
		where += " AND content LIKE ?"
		args = append(args, "%"+q+"%")
	}
	args = append(args, limit)
	rows, err := db.Query(`
		SELECT id, content, confidence, COALESCE(promoted_from,0), confirmed_at
		FROM semantic_confirmed WHERE `+where+`
		ORDER BY confirmed_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.SemanticConfirmed
	for rows.Next() {
		var c core.SemanticConfirmed
		if err := rows.Scan(&c.ID, &c.Content, &c.Confidence, &c.PromotedFrom, &c.ConfirmedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PruneRawTrailBefore 删除 id < beforeID 的 raw_trail（已被 episode 封段 + 语义抽取消费的旧事件）。
// raw_trail 是 episode/semantic 的源，调用方必须保证 beforeID < 两游标，否则会删掉还没消费的事件。
// episode.raw_start_id/raw_end_id 非 FK（仅信息列），删源事件不破坏已封段 episode 的摘要。返回删除条数。
func PruneRawTrailBefore(lifeID string, beforeID int64) (int64, error) {
	if beforeID <= 1 {
		return 0, nil
	}
	res, err := db.Exec(`DELETE FROM raw_trail WHERE life_id = ? AND id < ?`, lifeID, beforeID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// PruneWorkingMemoryKeepRecent 只保留最近 keep 条 working_memory，删更旧的。
// working_memory 是每 tick 工作记忆的回放镜像（in-mem 每 tick 已清空），旧行纯历史、可放心剪。
func PruneWorkingMemoryKeepRecent(lifeID string, keep int) (int64, error) {
	if keep <= 0 {
		return 0, nil
	}
	res, err := db.Exec(`
		DELETE FROM working_memory
		WHERE life_id = ? AND id < (
			SELECT COALESCE(MIN(id), 0) FROM (
				SELECT id FROM working_memory WHERE life_id = ? ORDER BY id DESC LIMIT ?
			)
		)`, lifeID, lifeID, keep)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func PromoteToConfirmed(lifeID string, candidateID int64, content string, confidence float64, ts int64) error {
	return PromoteToConfirmedWithEmbedding(lifeID, candidateID, content, confidence, ts, nil)
}

// PromoteToConfirmedWithEmbedding 同上，但携带 content 的 doc 向量写入 embedding 列。
// embedding 为 nil（嵌入服务挂了 / 未配）时写 NULL，检索回退关键词召回——绝不阻塞固化。
func PromoteToConfirmedWithEmbedding(lifeID string, candidateID int64, content string, confidence float64, ts int64, embedding []byte) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// C3 复confirm即强化：同 content 已固化 → 升置信 + 刷新 confirmed_at（重置衰减钟），不再造重复行。
	// 反复被经历印证的知识自此留存，从不复现的经衰减撤回——治"假信念永久复利成垃圾"。
	res, err := tx.Exec(`
		UPDATE semantic_confirmed
		SET confidence = MIN(1.0, confidence + 0.1), confirmed_at = ?
		WHERE life_id = ? AND content = ?`, ts, lifeID, content)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := tx.Exec(`
			INSERT INTO semantic_confirmed (life_id, content, confidence, promoted_from, embedding, confirmed_at)
			VALUES (?, ?, ?, ?, ?, ?)`, lifeID, content, confidence, candidateID, embedding, ts); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM semantic_candidate WHERE id = ?`, candidateID); err != nil {
		return err
	}
	return tx.Commit()
}

// DecayConfirmedSemantic 固化知识的衰减纠错（C3 防假信念永久复利）：每次（日度维护）对所有固化知识
// 乘 perRunFactor 衰减置信；跌破 floor 即撤回（删除）——从不复现/复用的知识渐淡去，反复被印证的
// 经 PromoteToConfirmed 刷新置信/confirmed_at 而留存。按维护节拍近似时间衰减（不逐行读 confirmed_at，
// 避免"每次按全龄重复衰减"的复利 bug）。返回 (降信条数, 撤回条数)。
func DecayConfirmedSemantic(lifeID string, perRunFactor, floor float64) (int, int, error) {
	rows, err := db.Query(`SELECT id, confidence FROM semantic_confirmed WHERE life_id = ?`, lifeID)
	if err != nil {
		return 0, 0, err
	}
	type rec struct {
		id   int64
		conf float64
	}
	var recs []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.id, &r.conf); err != nil {
			rows.Close()
			return 0, 0, err
		}
		recs = append(recs, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	if perRunFactor <= 0 || perRunFactor >= 1 {
		perRunFactor = 0.97
	}
	// 事务化：整批衰减/撤回原子提交，任一失败回滚 + 上报（避免部分持久化、避免静默吞错）。
	tx, err := db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	decayed, retracted := 0, 0
	for _, r := range recs {
		nc := r.conf * perRunFactor
		if nc < floor {
			if _, err := tx.Exec(`DELETE FROM semantic_confirmed WHERE id = ?`, r.id); err != nil {
				return 0, 0, err
			}
			retracted++
			continue
		}
		if _, err := tx.Exec(`UPDATE semantic_confirmed SET confidence = ? WHERE id = ?`, nc, r.id); err != nil {
			return 0, 0, err
		}
		decayed++
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return decayed, retracted, nil
}

// DowngradeConfirmedSemantic 据冲突证据下调某条固化知识的置信（C3 主动反驳入口）。
// 反思/新证据与固化知识矛盾时调；降到 0 由下一次衰减撤回。content 精确匹配。
func DowngradeConfirmedSemantic(lifeID, content string, delta float64) error {
	if delta < 0 {
		delta = -delta
	}
	_, err := db.Exec(`UPDATE semantic_confirmed SET confidence = MAX(0.0, confidence - ?) WHERE life_id = ? AND content = ?`, delta, lifeID, content)
	return err
}

func InsertReflection(lifeID string, m *core.ReflectionMemory) (int64, error) {
	r, err := db.Exec(`
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
