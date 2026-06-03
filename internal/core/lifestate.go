package core

// LifeState 持续变化的生命状态。仅 StateManager 可写。
// 见 docs/02 §3 / 03 §3。
type LifeState struct {
	LifeID           string  `json:"life_id"`
	Energy           float64 `json:"energy"`
	Competence       float64 `json:"competence"`
	SocialNeed       float64 `json:"social_need"`
	Stress           float64 `json:"stress"`
	Confidence       float64 `json:"confidence"`
	Stability        float64 `json:"stability"`
	EnergyDailyCap   float64 `json:"energy_daily_cap"`
	EnergyUsedToday  float64 `json:"energy_used_today"`
	CapResetAt       int64   `json:"cap_reset_at"`
	UpdatedAt        int64   `json:"updated_at"`
}
