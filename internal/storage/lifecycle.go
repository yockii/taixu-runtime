package storage

import "taixu.icu/runtime/internal/core"

func LoadLifecycleState(lifeID string) (core.LifecycleState, int64, error) {
	var state string
	var enteredAt int64
	err := db.QueryRow(`SELECT state, entered_at FROM lifecycle_state WHERE life_id = ?`, lifeID).
		Scan(&state, &enteredAt)
	if err != nil {
		return "", 0, err
	}
	return core.LifecycleState(state), enteredAt, nil
}

func UpsertLifecycleState(lifeID string, from, to core.LifecycleState, enteredAt int64, reason string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		INSERT INTO lifecycle_state (life_id, state, entered_at, reason)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(life_id) DO UPDATE SET
		    state=excluded.state, entered_at=excluded.entered_at, reason=excluded.reason`,
		lifeID, string(to), enteredAt, reason); err != nil {
		return err
	}

	var fromStr any
	if from != "" {
		fromStr = string(from)
	}
	if _, err := tx.Exec(`
		INSERT INTO lifecycle_history (life_id, from_state, to_state, transitioned_at, reason)
		VALUES (?, ?, ?, ?, ?)`,
		lifeID, fromStr, string(to), enteredAt, reason); err != nil {
		return err
	}
	return tx.Commit()
}

func AppendRawTrail(lifeID string, cycleID int64, eventType, payload string, ts int64) error {
	_, err := db.Exec(`
		INSERT INTO raw_trail (life_id, cycle_id, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		lifeID, cycleID, eventType, payload, ts)
	return err
}
