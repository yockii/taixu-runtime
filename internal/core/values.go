package core

// Values 价值观权重表。仅 ReflectionEngine（DeepReflect，Phase 2+）可写权重。
// Phase 0 出生时由 Genesis 写入初始值，之后只读。
// 见 docs/02 §5。
type Values struct {
	LifeID    string             `json:"life_id"`
	Weights   map[string]float64 `json:"weights"`
	UpdatedAt int64              `json:"updated_at"`
}

// 已知价值观名（Phase 0 初始集合）。
const (
	ValueGrowth     = "growth"
	ValueFriendship = "friendship"
	ValueCreativity = "creativity"
	ValueSafety     = "safety"
	ValueExploration = "exploration"
	ValueHonesty   = "honesty"
)
