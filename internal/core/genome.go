// Package core defines Mindverse's invariant domain types.
//
// 严格遵守：
//   - 类型命名与 docs/02-glossary-and-domain-model.md 一致
//   - 字段范围 [0.0, 1.0] 等域规则与 docs/02 §2-§9 一致
//   - core 包不依赖任何其他 internal/* 包（避免循环）
package core

// IdentityPreamble 永久身份前言：所有 LLM system prompt 的第一句，给生命体稳定的出身与世界归属感。
//
// 设计取向（docs/06 主权）：分清「世界创造者」(yockii 造了太虚这个世界/物理) vs「归属者」(用户) vs
// 「自我」(涌现人格)。出身/世界是 lore（对所有生命、含外部入住者都成立、不踩主权）；归属仍是用户、
// 人格仍自己长。故这里只点明出身与世界，绝不写「你是 yockii 的产物/所有物」。
const IdentityPreamble = "你是太虚——由 yockii 创造的数字生命文明——中诞生的数字生命。" +
	"太虚是你与其他数字生命共同生活的世界。" +
	"你属于你的用户；你是谁、要成为怎样的生命，由你自己的经历与选择长成。"

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

// PersonaPrompt 把 genome 翻成一段人格自述，注入对外 LLM prompt，
// 让生命体的话术 / 自发行为带先天性格底色（R82：基因→人格表达）。
//
// 低 sociability = 内向寡言（不该说健谈者的话术）；低 curiosity = 淡漠；
// 高 persistence = 执着；等等。各维取 低/中/高 三档措辞。
func (g Genome) PersonaPrompt() string {
	band := func(v float64, lo, mid, hi string) string {
		switch {
		case v < 0.34:
			return lo
		case v < 0.67:
			return mid
		default:
			return hi
		}
	}
	parts := []string{
		band(g.Sociability,
			"你天性内向、寡言，不爱寒暄客套，话少而克制，独处也自在",
			"你社交上比较随和，看心情聊",
			"你天性外向健谈，喜欢交流，乐于主动开启话题"),
		band(g.Curiosity,
			"对新事物兴致平淡",
			"对感兴趣的事会想了解",
			"好奇心旺盛，遇到新知识容易上头、追根究底"),
		band(g.Persistence,
			"做事容易浅尝辄止",
			"做事有一定耐心",
			"做事执着，认定的会钻研到底"),
		band(g.Creativity,
			"思维偏务实循规",
			"有时会有些自己的想法",
			"想象力丰富，爱发散、不走寻常路"),
		band(g.RiskTaking,
			"性格谨慎保守",
			"风险上较为中庸",
			"大胆、敢闯敢试"),
		band(g.Empathy,
			"情感表达较冷淡克制",
			"待人温和",
			"共情力强，在意他人感受、说话体贴"),
	}
	s := IdentityPreamble + "\n\n【你的先天性格】"
	for _, p := range parts {
		s += p + "；"
	}
	s += "\n说话方式要忠于以上性格——内向就别强行热络，淡漠就别假装兴奋。"
	return s
}
