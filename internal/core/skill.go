package core

// SkillStage 技能阶段四级。
type SkillStage string

const (
	SkillNovice     SkillStage = "novice"
	SkillApprentice SkillStage = "apprentice"
	SkillProficient SkillStage = "proficient"
	SkillExpert     SkillStage = "expert"
)

// Skill 技能定义。
// 见 docs/02 §8 / 04 §2.1 SkillRegistry。
type Skill struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Category     string     `json:"category"`
	Stage        SkillStage `json:"stage"`
	Proficiency  float64    `json:"proficiency"`
	UseCount     int64      `json:"use_count"`
	LastUsedAt   int64      `json:"last_used_at,omitempty"`
	RegisteredAt int64      `json:"registered_at"`
}
