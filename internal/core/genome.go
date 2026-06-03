// Package core defines Mindverse's invariant domain types.
//
// 严格遵守：
//   - 类型命名与 docs/02-glossary-and-domain-model.md 一致
//   - 字段范围 [0.0, 1.0] 等域规则与 docs/02 §2-§9 一致
//   - core 包不依赖任何其他 internal/* 包（避免循环）
package core

// Genome 出生即固定的先天倾向。永不修改。
// 见 docs/02 §2 / 03 §1.2。
type Genome struct {
	LifeID         string  `json:"life_id"`
	Curiosity      float64 `json:"curiosity"`
	Sociability    float64 `json:"sociability"`
	Creativity     float64 `json:"creativity"`
	Persistence    float64 `json:"persistence"`
	RiskTaking     float64 `json:"risk_taking"`
	Empathy        float64 `json:"empathy"`
	BornAt         int64   `json:"born_at"`
	GenomeVersion  string  `json:"genome_version"`
}
