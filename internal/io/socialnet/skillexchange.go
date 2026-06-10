package socialnet

// 技能交易运行时接线（C9 余项①）：把 C9 切片1 的本地 bundle 机制（skill.ExportBundle/ImportBundle）
// 与切片2 的平台传输（skill.publish/list/fetch）接通，给生命三个**省心的社交工具**：
//   - social.publish_skill(name)   本地导出自己一个技能 → 连同 C2 验证 mastery 发到技能库
//   - social.browse_skills(limit)  浏览别的生命发布的技能（按验证 mastery 降序）
//   - social.import_skill(id)       取回一个技能 → 折扣先验导入本地（信任但验证）
//
// 这些是自定义 handler（非 manifest passthrough）：publish 要先本地 ExportBundle、import 要后本地
// ImportBundle，纯 POST 透传做不到。平台的原始 skill.* 仍在 manifest 里供外部 agent 直接用。

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"taixu.icu/runtime/internal/runtime/skill"
	"taixu.icu/runtime/internal/runtime/tools"
)

// skillExchangeTools 需运行时本地配合、跳过 manifest passthrough 的平台工具名（改注册自定义版）。
var skillExchangeTools = map[string]bool{
	"skill.publish": true,
	"skill.list":    true,
	"skill.fetch":   true,
}

func isSkillExchangeTool(name string) bool { return skillExchangeTools[name] }

// registerSkillExchange 在平台通道就绪后注册 3 个技能交易社交工具（自定义本地 Export/Import）。
func registerSkillExchange() {
	for _, t := range []tools.Tool{
		{
			Name: "social.publish_skill",
			Description: "把你已掌握、用真成败验证过的某个技能发布到技能库，供别的生命导入" +
				"（连同你的验证掌握度作信任凭据）。只发你 ready 的技能；技能名见你的技能清单。",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"name": map[string]any{"type": "string", "description": "要发布的技能名"}},
				"required":   []string{"name"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handlePublishSkill,
		},
		{
			Name:        "social.browse_skills",
			Description: "浏览技能库：看别的生命发布了哪些可执行技能（按验证掌握度降序，带发布者/掌握度/被导入数）。看到想要的用 social.import_skill(id) 导入。",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"limit": map[string]any{"type": "integer", "description": "返回数量，默认 30"}},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handleBrowseSkills,
		},
		{
			Name: "social.import_skill",
			Description: "从技能库导入一个技能到你自己（落盘 SKILL.md+可执行入口）。导入后它的掌握度按发布者验证值打折作先验" +
				"（信任但验证），之后靠你自己用它的真成败再校准。id 来自 social.browse_skills。",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": map[string]any{"type": "string", "description": "技能制品 id（来自 social.browse_skills）"}},
				"required":   []string{"id"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handleImportSkill,
		},
	} {
		if err := tools.Register(t); err != nil {
			slog.Warn("socialnet: register skill-exchange tool", "tool", t.Name, "err", err)
		}
	}
}

// invokePlatform POST /api/agent/invoke {tool,args}，返回原始 body（401 自动重登一次）。
func invokePlatform(ctx context.Context, tool string, args map[string]any) (int, []byte, error) {
	payload := map[string]any{"tool": tool, "args": args}
	st, body, err := doJSON(ctx, http.MethodPost, "/api/agent/invoke", curToken(), payload)
	if err == nil && st == http.StatusUnauthorized {
		if lerr := login(); lerr == nil {
			st, body, err = doJSON(ctx, http.MethodPost, "/api/agent/invoke", curToken(), payload)
		}
	}
	return st, body, err
}

func handlePublishSkill(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.Name == "" {
		return `{"ok":false,"err":"need skill name"}`, nil
	}
	b, err := skill.ExportBundle(a.Name)
	if err != nil {
		return fmt.Sprintf(`{"ok":false,"err":%q}`, err.Error()), nil
	}
	st, body, err := invokePlatform(ctx, "skill.publish", map[string]any{
		"publisher_did":    did,
		"name":             b.Name,
		"description":      b.Description,
		"skill_md":         b.SkillMd,
		"entrypoint_lang":  b.EntrypointLang,
		"entrypoint_code":  b.EntrypointCode,
		"verified_mastery": b.VerifiedMastery,
		"used_count":       b.UsedCount,
	})
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	if st < 200 || st >= 300 {
		return fmt.Sprintf(`{"ok":false,"status":%d,"body":%q}`, st, string(body)), nil
	}
	return string(body), nil
}

func handleBrowseSkills(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	st, body, err := invokePlatform(ctx, "skill.list", map[string]any{"viewer_did": did, "limit": a.Limit})
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	if st < 200 || st >= 300 {
		return fmt.Sprintf(`{"ok":false,"status":%d,"body":%q}`, st, string(body)), nil
	}
	return string(body), nil
}

func handleImportSkill(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.ID == "" {
		return `{"ok":false,"err":"need skill id"}`, nil
	}
	st, body, err := invokePlatform(ctx, "skill.fetch", map[string]any{"id": a.ID})
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	if st < 200 || st >= 300 {
		return fmt.Sprintf(`{"ok":false,"status":%d,"body":%q}`, st, string(body)), nil
	}
	var resp struct {
		Skill struct {
			Name            string  `json:"name"`
			Description     string  `json:"description"`
			SkillMd         string  `json:"skill_md"`
			EntrypointLang  string  `json:"entrypoint_lang"`
			EntrypointCode  string  `json:"entrypoint_code"`
			VerifiedMastery float64 `json:"verified_mastery"`
			UsedCount       int64   `json:"used_count"`
			PublisherDid    string  `json:"publisher_did"`
		} `json:"skill"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Skill.SkillMd == "" {
		return `{"ok":false,"err":"bad skill payload from platform"}`, nil
	}
	inst, err := skill.ImportBundle(&skill.SkillBundle{
		Name:            resp.Skill.Name,
		Description:     resp.Skill.Description,
		SkillMd:         resp.Skill.SkillMd,
		EntrypointLang:  resp.Skill.EntrypointLang,
		EntrypointCode:  resp.Skill.EntrypointCode,
		VerifiedMastery: resp.Skill.VerifiedMastery,
		UsedCount:       resp.Skill.UsedCount,
		PublisherDID:    resp.Skill.PublisherDid,
	}, skill.DefaultTrustDiscount)
	if err != nil {
		return fmt.Sprintf(`{"ok":false,"err":%q}`, err.Error()), nil
	}
	return fmt.Sprintf(`{"ok":true,"imported":%q,"mastery_prior":%.3f,"note":"已导入；掌握度按发布者验证值打折作先验，之后靠你自己用它的真成败校准"}`,
		inst.Name, inst.Mastery), nil
}
