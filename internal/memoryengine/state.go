package memoryengine

import (
	"mindverse/internal/core"
)

// LoadLifeState 读取当前生命状态。
func (s *Store) LoadLifeState(lifeID string) (*core.LifeState, error) {
	row := s.db.QueryRow(`
		SELECT life_id, energy, competence, social_need, stress, confidence, stability,
		       energy_daily_cap, energy_used_today, cap_reset_at, updated_at
		FROM life_state WHERE life_id = ?`, lifeID)
	var ls core.LifeState
	err := row.Scan(&ls.LifeID, &ls.Energy, &ls.Competence, &ls.SocialNeed,
		&ls.Stress, &ls.Confidence, &ls.Stability,
		&ls.EnergyDailyCap, &ls.EnergyUsedToday, &ls.CapResetAt, &ls.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ls, nil
}

// UpsertLifeState 写入 / 更新（仅 StateManager 应调用）。
func (s *Store) UpsertLifeState(ls *core.LifeState) error {
	_, err := s.db.Exec(`
		INSERT INTO life_state (life_id, energy, competence, social_need, stress, confidence, stability,
		                       energy_daily_cap, energy_used_today, cap_reset_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(life_id) DO UPDATE SET
		    energy=excluded.energy, competence=excluded.competence,
		    social_need=excluded.social_need, stress=excluded.stress,
		    confidence=excluded.confidence, stability=excluded.stability,
		    energy_daily_cap=excluded.energy_daily_cap,
		    energy_used_today=excluded.energy_used_today,
		    cap_reset_at=excluded.cap_reset_at,
		    updated_at=excluded.updated_at`,
		ls.LifeID, ls.Energy, ls.Competence, ls.SocialNeed,
		ls.Stress, ls.Confidence, ls.Stability,
		ls.EnergyDailyCap, ls.EnergyUsedToday, ls.CapResetAt, ls.UpdatedAt,
	)
	return err
}

// LoadMentalState 读取情绪层。
func (s *Store) LoadMentalState(lifeID string) (*core.MentalState, error) {
	row := s.db.QueryRow(`
		SELECT life_id, motivation, satisfaction, anxiety, updated_at
		FROM mental_state WHERE life_id = ?`, lifeID)
	var ms core.MentalState
	err := row.Scan(&ms.LifeID, &ms.Motivation, &ms.Satisfaction, &ms.Anxiety, &ms.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ms, nil
}

// UpsertMentalState 写入 / 更新（仅 StateManager 应调用）。
func (s *Store) UpsertMentalState(ms *core.MentalState) error {
	_, err := s.db.Exec(`
		INSERT INTO mental_state (life_id, motivation, satisfaction, anxiety, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(life_id) DO UPDATE SET
		    motivation=excluded.motivation,
		    satisfaction=excluded.satisfaction,
		    anxiety=excluded.anxiety,
		    updated_at=excluded.updated_at`,
		ms.LifeID, ms.Motivation, ms.Satisfaction, ms.Anxiety, ms.UpdatedAt,
	)
	return err
}

// UpsertValue 写入或更新单个价值观权重。
func (s *Store) UpsertValue(lifeID, name string, weight float64, updatedAt int64) error {
	_, err := s.db.Exec(`
		INSERT INTO life_values (life_id, name, weight, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(life_id, name) DO UPDATE SET
		    weight=excluded.weight, updated_at=excluded.updated_at`,
		lifeID, name, weight, updatedAt)
	return err
}

// LoadValues 读取价值观权重表。
func (s *Store) LoadValues(lifeID string) (*core.Values, error) {
	rows, err := s.db.Query(`SELECT name, weight, updated_at FROM life_values WHERE life_id = ?`, lifeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	v := &core.Values{
		LifeID:  lifeID,
		Weights: map[string]float64{},
	}
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
