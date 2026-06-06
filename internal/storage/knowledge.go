package storage

// knowledge_entry（migration 010）：按一次完整研究聚合的「篇章式档案」（dossier）。
// 与 semantic_confirmed（细碎、向量可检索的知识点）互补：这里是人读 / API 浏览的成篇知识。

// KnowledgeEntry 一篇知识库 dossier。
type KnowledgeEntry struct {
	ID         int64  `json:"id"`
	RootGoalID int64  `json:"root_goal_id,omitempty"`
	Topic      string `json:"topic"`
	Body       string `json:"body"`
	SourceKind string `json:"source_kind"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

// InsertKnowledgeEntry 落一篇 dossier，返回新 id。rootGoalID==0 写 NULL（手工注入无来源目标）。
func InsertKnowledgeEntry(lifeID string, e *KnowledgeEntry) (int64, error) {
	kind := e.SourceKind
	if kind == "" {
		kind = "research"
	}
	r, err := db.Exec(`
		INSERT INTO knowledge_entry (life_id, root_goal_id, topic, body, source_kind, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		lifeID, nullInt(e.RootGoalID), e.Topic, e.Body, kind, e.CreatedAt, e.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// HasKnowledgeForRootGoal 该根目标是否已生成过 dossier（防同一根目标重复综合入库）。
func HasKnowledgeForRootGoal(lifeID string, rootGoalID int64) (bool, error) {
	if rootGoalID == 0 {
		return false, nil
	}
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM knowledge_entry WHERE life_id = ? AND root_goal_id = ?`,
		lifeID, rootGoalID).Scan(&n)
	return n > 0, err
}

// ListKnowledge 分页列 dossier（topic + 正文摘要 + 时间 + root_goal_id），按创建时间倒序。
// 列表只返回 body 的前 bodyPreview 字符，避免一次拉全文。
func ListKnowledge(lifeID string, limit, offset int) ([]KnowledgeEntry, error) {
	rows, err := db.Query(`
		SELECT id, COALESCE(root_goal_id,0), topic, body, source_kind, created_at, updated_at
		FROM knowledge_entry WHERE life_id = ?
		ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, lifeID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KnowledgeEntry{}
	for rows.Next() {
		var e KnowledgeEntry
		if err := rows.Scan(&e.ID, &e.RootGoalID, &e.Topic, &e.Body, &e.SourceKind, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Body = previewBody(e.Body)
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetKnowledge 取单篇 dossier 全文。不存在返回 (nil, nil)。
func GetKnowledge(lifeID string, id int64) (*KnowledgeEntry, error) {
	var e KnowledgeEntry
	err := db.QueryRow(`
		SELECT id, COALESCE(root_goal_id,0), topic, body, source_kind, created_at, updated_at
		FROM knowledge_entry WHERE life_id = ? AND id = ?`, lifeID, id).
		Scan(&e.ID, &e.RootGoalID, &e.Topic, &e.Body, &e.SourceKind, &e.CreatedAt, &e.UpdatedAt)
	if err == ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// previewBody 列表摘要：取正文前 280 字符（按 rune 不截断多字节）。
func previewBody(s string) string {
	const max = 280
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
