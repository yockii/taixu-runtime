package core

// LifeState 持续变化的生命状态。仅 StateManager 可写。
// 见 docs/02 §3 / 03 §3。
type LifeState struct {
	LifeID          string  `json:"life_id"`
	Energy          float64 `json:"energy"`
	Competence      float64 `json:"competence"`
	SocialNeed      float64 `json:"social_need"`
	Stress          float64 `json:"stress"`
	Confidence      float64 `json:"confidence"`
	Stability       float64 `json:"stability"`
	EnergyDailyCap  float64 `json:"energy_daily_cap"`
	EnergyUsedToday float64 `json:"energy_used_today"`
	CapResetAt      int64   `json:"cap_reset_at"`
	// Wealth 生命体在世经济财富（$WEALTH，白皮书 5 资源之一，C10）。**非 [0,1] 标量**——
	// 无界累积货币，floor 0、无上界，绝不走 state.Delta 的 [0,1] clamp。社交活动产出（§3.1.2）、
	// 未来技能/物品交易流通（06 §7）。与平台星屑物理隔离不可兑换（06 §3.5 / R110）。
	Wealth float64 `json:"wealth"`
	// SocialWealthToday 当日社交活动已产 wealth 累计——反刷递减用（§3.1.2 / R109）：今日产得越多，
	// 单次社交回报越小。随日精力上限一同每日清零。
	SocialWealthToday float64 `json:"social_wealth_today"`
	UpdatedAt         int64   `json:"updated_at"`
}
