package core

// MentalState 情绪层。仅 StateManager 可写。
// 见 docs/02 §4。
type MentalState struct {
	LifeID       string  `json:"life_id"`
	Motivation   float64 `json:"motivation"`
	Satisfaction float64 `json:"satisfaction"`
	Anxiety      float64 `json:"anxiety"`
	UpdatedAt    int64   `json:"updated_at"`
}
