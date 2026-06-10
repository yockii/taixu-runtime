package socialnet

import (
	"testing"

	"taixu.icu/runtime/internal/runtime/tools"
)

// TestIsSkillExchangeTool 验跳过集：skill.* 被识别（manifest passthrough 跳过），其余社交工具不跳。
func TestIsSkillExchangeTool(t *testing.T) {
	for _, n := range []string{"skill.publish", "skill.list", "skill.fetch", "wealth.claim"} {
		if !isSkillExchangeTool(n) {
			t.Errorf("%s 应被识别为需本地配合的工具(跳过 passthrough)", n)
		}
	}
	// wealth.balance 只读、可透传，不在跳过集。
	for _, n := range []string{"social.post", "social.comment", "market.publish", "wealth.balance", ""} {
		if isSkillExchangeTool(n) {
			t.Errorf("%s 不应被跳过", n)
		}
	}
}

// TestRegisterSkillExchange 验 C9 wiring：3 个自定义社交工具注册进慎思 lane。
func TestRegisterSkillExchange(t *testing.T) {
	_ = tools.Init()
	registerSkillExchange()
	got := map[string]bool{}
	for _, lt := range tools.ListLLMTools(tools.LaneDeliberative) {
		got[lt.Name] = true
	}
	for _, want := range []string{"social.publish_skill", "social.browse_skills", "social.import_skill", "wealth.claim"} {
		if !got[want] {
			t.Errorf("应注册 %s 进慎思 lane", want)
		}
	}
}
