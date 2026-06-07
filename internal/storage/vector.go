package storage

// 向量检索 / 回填的存储侧支持（配合 internal/io/embed 的暴力 cosine）。
//
// 三张记忆表带 embedding BLOB 列：episode / semantic_confirmed / reflection_memory。
// 这里提供：
//   - 取「带非空 embedding」候选行（id + blob + 展示文本）供暴力 cosine top-k；
//   - 取「embedding 为空」的行 + 回写 embedding，供历史回填。
// 全部为薄查询，向量编解码 / 相似度计算在 embed 包。

// VectorRow 一条参与向量检索的候选：id、文本（用于回显）、向量字节。
type VectorRow struct {
	ID   int64
	Text string
	Blob []byte
}

// embeddedLayer 把 layer 名映射到 (表名, 文本列)。layer 非法返回 ok=false。
// 文本列即语义检索回显与回填求嵌入所用的源文本。
func embeddedLayer(layer string) (table, textCol string, ok bool) {
	switch layer {
	case "episodic":
		return "episode", "summary", true
	case "semantic":
		return "semantic_confirmed", "content", true
	case "reflection":
		return "reflection_memory", "summary", true
	default:
		return "", "", false
	}
}

// ListEmbeddedRows 取某记忆层中 embedding 非空的候选行（最近 limit 条），供暴力 cosine 召回。
// q 非空时先按文本列模糊预筛（缩小暴力扫描集；纯关键词 + 向量混合召回）。
func ListEmbeddedRows(lifeID, layer, q string, limit int) ([]VectorRow, error) {
	table, textCol, ok := embeddedLayer(layer)
	if !ok {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	args := []any{lifeID}
	where := "life_id = ? AND embedding IS NOT NULL"
	if q != "" {
		where += " AND " + textCol + " LIKE ?"
		args = append(args, "%"+q+"%")
	}
	args = append(args, limit)
	rows, err := db.Query(`
		SELECT id, COALESCE(`+textCol+`,''), embedding
		FROM `+table+` WHERE `+where+`
		ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VectorRow
	for rows.Next() {
		var r VectorRow
		if err := rows.Scan(&r.ID, &r.Text, &r.Blob); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListRowsMissingEmbedding 取某层 embedding 为空（含文本非空）的行（最旧 limit 条），供回填求向量。
// 按 id 升序——回填从最老的历史记忆开始，配合游标可分批可重入。
func ListRowsMissingEmbedding(lifeID, layer string, afterID int64, limit int) ([]VectorRow, error) {
	table, textCol, ok := embeddedLayer(layer)
	if !ok {
		return nil, nil
	}
	if limit <= 0 {
		limit = 32
	}
	rows, err := db.Query(`
		SELECT id, COALESCE(`+textCol+`,''), embedding
		FROM `+table+`
		WHERE life_id = ? AND embedding IS NULL AND `+textCol+` IS NOT NULL AND `+textCol+` <> '' AND id > ?
		ORDER BY id ASC LIMIT ?`, lifeID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VectorRow
	for rows.Next() {
		var r VectorRow
		if err := rows.Scan(&r.ID, &r.Text, &r.Blob); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// EmbeddingCoverage 统计三记忆层「已嵌入 / 可嵌入」行数（面板进度提示）。
// 可嵌入 = 文本列非空的行；已嵌入 = 其中 embedding 非空。跨全部生命（Phase 0 单生命）。
func EmbeddingCoverage() (embedded, total int64) {
	for _, layer := range []string{"episodic", "semantic", "reflection"} {
		table, textCol, ok := embeddedLayer(layer)
		if !ok {
			continue
		}
		var e, t int64
		_ = db.QueryRow(`SELECT
			COUNT(*) FILTER (WHERE embedding IS NOT NULL),
			COUNT(*)
			FROM `+table+` WHERE `+textCol+` IS NOT NULL AND `+textCol+` <> ''`).Scan(&e, &t)
		embedded += e
		total += t
	}
	return embedded, total
}

// UpdateEmbedding 回写某层某行的 embedding（回填用）。blob 为 nil 时不写（避免把已有向量清空）。
func UpdateEmbedding(layer string, id int64, blob []byte) error {
	table, _, ok := embeddedLayer(layer)
	if !ok || len(blob) == 0 {
		return nil
	}
	_, err := db.Exec(`UPDATE `+table+` SET embedding = ? WHERE id = ?`, blob, id)
	return err
}
