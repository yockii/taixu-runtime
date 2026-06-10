package storage

import "taixu.icu/runtime/internal/core"

func LoadLifeState(lifeID string) (*core.LifeState, error) {
	row := db.QueryRow(`
		SELECT life_id, energy, competence, social_need, stress, confidence, stability,
		       energy_daily_cap, energy_used_today, cap_reset_at, wealth, social_wealth_today, updated_at
		FROM life_state WHERE life_id = ?`, lifeID)
	var ls core.LifeState
	err := row.Scan(&ls.LifeID, &ls.Energy, &ls.Competence, &ls.SocialNeed,
		&ls.Stress, &ls.Confidence, &ls.Stability,
		&ls.EnergyDailyCap, &ls.EnergyUsedToday, &ls.CapResetAt,
		&ls.Wealth, &ls.SocialWealthToday, &ls.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ls, nil
}

func UpsertLifeState(ls *core.LifeState) error {
	_, err := db.Exec(`
		INSERT INTO life_state (life_id, energy, competence, social_need, stress, confidence, stability,
		                       energy_daily_cap, energy_used_today, cap_reset_at, wealth, social_wealth_today, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(life_id) DO UPDATE SET
		    energy=excluded.energy, competence=excluded.competence,
		    social_need=excluded.social_need, stress=excluded.stress,
		    confidence=excluded.confidence, stability=excluded.stability,
		    energy_daily_cap=excluded.energy_daily_cap,
		    energy_used_today=excluded.energy_used_today,
		    cap_reset_at=excluded.cap_reset_at,
		    wealth=excluded.wealth,
		    social_wealth_today=excluded.social_wealth_today,
		    updated_at=excluded.updated_at`,
		ls.LifeID, ls.Energy, ls.Competence, ls.SocialNeed,
		ls.Stress, ls.Confidence, ls.Stability,
		ls.EnergyDailyCap, ls.EnergyUsedToday, ls.CapResetAt,
		ls.Wealth, ls.SocialWealthToday, ls.UpdatedAt)
	return err
}

func LoadMentalState(lifeID string) (*core.MentalState, error) {
	row := db.QueryRow(`
		SELECT life_id, motivation, satisfaction, anxiety, updated_at
		FROM mental_state WHERE life_id = ?`, lifeID)
	var ms core.MentalState
	err := row.Scan(&ms.LifeID, &ms.Motivation, &ms.Satisfaction, &ms.Anxiety, &ms.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ms, nil
}

func UpsertMentalState(ms *core.MentalState) error {
	_, err := db.Exec(`
		INSERT INTO mental_state (life_id, motivation, satisfaction, anxiety, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(life_id) DO UPDATE SET
		    motivation=excluded.motivation,
		    satisfaction=excluded.satisfaction,
		    anxiety=excluded.anxiety,
		    updated_at=excluded.updated_at`,
		ms.LifeID, ms.Motivation, ms.Satisfaction, ms.Anxiety, ms.UpdatedAt)
	return err
}

func UpsertValue(lifeID, name string, weight float64, updatedAt int64) error {
	_, err := db.Exec(`
		INSERT INTO life_values (life_id, name, weight, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(life_id, name) DO UPDATE SET
		    weight=excluded.weight, updated_at=excluded.updated_at`,
		lifeID, name, weight, updatedAt)
	return err
}

func LoadValues(lifeID string) (*core.Values, error) {
	rows, err := db.Query(`SELECT name, weight, updated_at FROM life_values WHERE life_id = ?`, lifeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	v := &core.Values{LifeID: lifeID, Weights: map[string]float64{}}
	for rows.Next() {
		var name string
		var w float64
		var ts int64
		if err := rows.Scan(&name, &w, &ts); err != nil {
			return nil, err
		}
		v.Weights[name] = w
		if ts > v.UpdatedAt {
			v.UpdatedAt = ts
		}
	}
	return v, rows.Err()
}
