package storage

func AppendLedger(lifeID, resource string, delta, balanceAfter float64, reason, sourceKind, sourceRef string, ts int64) error {
	_, err := db.Exec(`
		INSERT INTO resource_ledger (life_id, resource, delta, balance_after, reason, source_kind, source_ref, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, resource, delta, balanceAfter, reason, sourceKind, sourceRef, ts)
	return err
}

func SumLedger(lifeID, resource string) (float64, error) {
	var sum *float64
	err := db.QueryRow(`
		SELECT SUM(delta) FROM resource_ledger
		WHERE life_id = ? AND resource = ?`, lifeID, resource).Scan(&sum)
	if err != nil {
		return 0, err
	}
	if sum == nil {
		return 0, nil
	}
	return *sum, nil
}
