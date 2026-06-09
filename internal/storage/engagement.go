package storage

// 社交 have-I-done-this 去重（C6）：记录生命已回应过的对象，供下次回应前查重，
// 防同一对象被反复回应（见 migration 013）。

// RecordEngagement UPSERT 一条「已回应」记录（同对象再回只刷新 last_at，不堆行）。
func RecordEngagement(lifeID, targetKey string, ts int64) error {
	_, err := db.Exec(`
		INSERT INTO social_engagement (life_id, target_key, last_at)
		VALUES (?,?,?)
		ON CONFLICT(life_id, target_key) DO UPDATE SET last_at = excluded.last_at`,
		lifeID, targetKey, ts)
	return err
}

// EngagedSince 该生命是否在 sinceTs 之后已回应过 targetKey（窗内查重）。
func EngagedSince(lifeID, targetKey string, sinceTs int64) (bool, error) {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM social_engagement
		WHERE life_id = ? AND target_key = ? AND last_at >= ?`,
		lifeID, targetKey, sinceTs).Scan(&n)
	return n > 0, err
}
