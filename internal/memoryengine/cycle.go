package memoryengine

// MaxCycleID 返回该生命体 raw_trail 中最大的 cycle_id。
// 用于跨重启 cycle_id 续接。无记录返回 0。
func (s *Store) MaxCycleID(lifeID string) (int64, error) {
	var maxID *int64
	err := s.db.QueryRow(`SELECT MAX(cycle_id) FROM raw_trail WHERE life_id = ?`, lifeID).Scan(&maxID)
	if err != nil {
		return 0, err
	}
	if maxID == nil {
		return 0, nil
	}
	return *maxID, nil
}
