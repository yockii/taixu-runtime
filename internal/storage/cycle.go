package storage

func MaxCycleID(lifeID string) (int64, error) {
	var maxID *int64
	err := db.QueryRow(`SELECT MAX(cycle_id) FROM raw_trail WHERE life_id = ?`, lifeID).Scan(&maxID)
	if err != nil {
		return 0, err
	}
	if maxID == nil {
		return 0, nil
	}
	return *maxID, nil
}
