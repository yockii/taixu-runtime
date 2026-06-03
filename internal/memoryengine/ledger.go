package memoryengine

// AppendLedger 追加一条资源账本流水。仅 ResourceLedger 应调用。
func (s *Store) AppendLedger(lifeID, resource string, delta, balanceAfter float64, reason, sourceKind, sourceRef string, ts int64) error {
	_, err := s.db.Exec(`
		INSERT INTO resource_ledger (life_id, resource, delta, balance_after, reason, source_kind, source_ref, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		lifeID, resource, delta, balanceAfter, reason, sourceKind, sourceRef, ts)
	return err
}

// SumLedger 返回某生命体某资源所有 delta 之和（再现 balance）。
func (s *Store) SumLedger(lifeID, resource string) (float64, error) {
	var sum *float64
	err := s.db.QueryRow(`
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
