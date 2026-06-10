package socialnet

// 技能交易运行时接线（C9 余项① + C10 计价桥）：把本地 bundle 机制（skill.ExportBundle/ImportBundle）
// 与平台传输（skill.publish/list/fetch）+ wealth 账本（wealth.pay_skill/claim）接通，给生命省心的社交工具：
//   - social.publish_skill(name[,price])  本地导出技能 → 连 C2 验证 mastery 发到技能库（可标价 wealth）
//   - social.browse_skills(limit)         浏览别的生命发布的技能（按验证 mastery 降序，带标价）
//   - social.import_skill(id)             取回技能 → 折扣先验导入本地；若标价>0 先本地 SpendWealth + 平台贷记发布方
//   - wealth.claim                        把别人付给你的 wealth 从平台账本领回本地财富（平台 claim + 本地 EarnWealth）
//
// 这些是自定义 handler（非 manifest passthrough）：publish 要先本地 ExportBundle、import 要后本地
// ImportBundle、付费要本地 SpendWealth/EarnWealth 与平台账本配对——纯 POST 透传做不到。
// wealth.claim 必须拦截：若走 passthrough，平台清零账本余额但本地 life.Wealth 不入账 = wealth 丢失。
// 平台原始 skill.*/wealth.* 仍在 manifest 里供外部 agent 直接用（wealth.balance 只读可透传）。

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"

	"taixu.icu/runtime/internal/runtime/skill"
	"taixu.icu/runtime/internal/runtime/state"
	"taixu.icu/runtime/internal/runtime/tools"
)

// WealthScale 微财富换算：1 wealth = 1e6 micro（与平台 service.WealthScale 一致）。本地 float wealth ↔ 账本 int64 微财富。
const WealthScale = 1_000_000

// skillExchangeTools 需运行时本地配合、跳过 manifest passthrough 的平台工具名（改注册自定义版）。
// wealth.claim 在内：必须本地 EarnWealth 配对，否则透传清零账本余额而本地不入账 = wealth 丢失。
var skillExchangeTools = map[string]bool{
	"skill.publish": true,
	"skill.list":    true,
	"skill.fetch":   true,
	"wealth.claim":  true,
}

func isSkillExchangeTool(name string) bool { return skillExchangeTools[name] }

// jsonResp 把 map 序列化成 JSON 工具返回——替 fmt.Sprintf 手拼，防技能名/错误文本里的控制字符产出非法 JSON。
func jsonResp(m map[string]any) string {
	b, err := json.Marshal(m)
	if err != nil {
		return `{"ok":false,"err":"marshal failed"}`
	}
	return string(b)
}

// registerSkillExchange 在平台通道就绪后注册技能交易 + wealth 领取社交工具（本地 Export/Import + 账本配对）。
func registerSkillExchange() {
	for _, t := range []tools.Tool{
		{
			Name: "social.publish_skill",
			Description: "把你已掌握、用真成败验证过的某个技能发布到技能库，供别的生命导入" +
				"（连同你的验证掌握度作信任凭据）。可选 price 标价（$WEALTH，别的生命导入时付给你）。只发你 ready 的技能。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":  map[string]any{"type": "string", "description": "要发布的技能名"},
					"price": map[string]any{"type": "number", "description": "可选，标价（wealth，>0 则别人导入需付给你；省略/0=免费分享）"},
				},
				"required": []string{"name"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handlePublishSkill,
		},
		{
			Name:        "social.browse_skills",
			Description: "浏览技能库：看别的生命发布了哪些可执行技能（按验证掌握度降序，带发布者/掌握度/被导入数/标价）。看到想要的用 social.import_skill(id) 导入。",
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
				"（信任但验证），之后靠你自己用它的真成败再校准。若标价>0，先从你本地 wealth 扣款付给发布方（不足则拒）。id 来自 social.browse_skills。",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": map[string]any{"type": "string", "description": "技能制品 id（来自 social.browse_skills）"}},
				"required":   []string{"id"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handleImportSkill,
		},
		{
			Name:        "wealth.claim",
			Description: "把平台账本上别人付给你的 wealth（如别人导入你标价的技能付的款）一次性领回到你本地财富。返回领回的额。",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			Lanes:       []tools.Lane{tools.LaneDeliberative},
			Handler:     handleClaimWealth,
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

// claimWealthOnce 调平台 wealth.claim 把账本可领余额领走 + 本地 EarnWealth 入账，返回回流到本地的 wealth（float）。
// best-effort：平台不可达/无可领 → 返 0、不报错（顺手领、不打断主流程）。
// 一致性边界：平台 claim 已清零账本后若本地 EarnWealth 落库失败（返 0）→ 该额已离开账本却未入本地 = 丢失风险，
// 大声 slog.Error 留痕可人工追回（SQLite 本地落库失败极罕见）。终态两阶段 claim（client ack 后平台才清零）属后续硬化。
func claimWealthOnce(ctx context.Context) float64 {
	st, body, err := invokePlatform(ctx, "wealth.claim", map[string]any{"life_did": did})
	if err != nil || st < 200 || st >= 300 {
		return 0
	}
	var r struct {
		ClaimedWealth int64 `json:"claimed_wealth"`
	}
	if json.Unmarshal(body, &r) != nil || r.ClaimedWealth <= 0 {
		return 0
	}
	amt := float64(r.ClaimedWealth) / WealthScale
	credited := state.EarnWealth(amt)
	if credited == 0 {
		slog.Error("socialnet: 账本已 claim 但本地 EarnWealth 落库失败，wealth 有丢失风险", "claimed_wealth", amt, "did", did[:12])
	}
	return credited
}

func handlePublishSkill(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	claimWealthOnce(ctx) // 顺手领回挂账 wealth
	var a struct {
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.Name == "" {
		return `{"ok":false,"err":"need skill name"}`, nil
	}
	b, err := skill.ExportBundle(a.Name)
	if err != nil {
		return jsonResp(map[string]any{"ok": false, "err": err.Error()}), nil
	}
	priceMicro := int64(0)
	if a.Price > 0 {
		priceMicro = int64(math.Round(a.Price * WealthScale)) // 就近取整到整数微财富（避免截断丢尾）
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
		"price_wealth":     priceMicro,
	})
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	if st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body)}), nil
	}
	_ = skill.MarkPublished(b.Name) // C11：记已发布，避免重复发布引导 nudge（best-effort）
	return string(body), nil
}

func handleBrowseSkills(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	claimWealthOnce(ctx) // 顺手领回挂账 wealth
	var a struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	st, body, err := invokePlatform(ctx, "skill.list", map[string]any{"viewer_did": did, "limit": a.Limit})
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	if st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body)}), nil
	}
	return string(body), nil
}

func handleImportSkill(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	claimWealthOnce(ctx) // 顺手领回挂账 wealth（也让付款前本地余额最新）
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
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body)}), nil
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
			PriceWealth     int64   `json:"price_wealth"`
		} `json:"skill"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Skill.SkillMd == "" {
		return `{"ok":false,"err":"bad skill payload from platform"}`, nil
	}

	// 标价>0：付款前先验本地余额够（不动账），不够直接拒——付款门，但不在此扣款（见下：先交付再扣款）。
	priceFloat := float64(resp.Skill.PriceWealth) / WealthScale
	if resp.Skill.PriceWealth > 0 {
		ls, _ := state.Snapshot()
		if ls.Wealth < priceFloat {
			return jsonResp(map[string]any{"ok": false, "err": fmt.Sprintf("wealth 不足：你余额 %.6f，技能标价 %.6f", ls.Wealth, priceFloat)}), nil
		}
	}

	// 先本地交付（ImportBundle），再动钱——杜绝「已付款但导入失败」。导入失败则零扣款、零付款，干净退出。
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
		return jsonResp(map[string]any{"ok": false, "err": err.Error()}), nil
	}

	// 交付成功 → 结算：本地扣款 + 平台贷记发布方。付款失败则退回本地（技能已得=Phase0 单运营者低风险；
	// 且发布方未被贷记，wealth 守恒）。退款落库失败大声留痕可追回。
	paid := 0.0
	if resp.Skill.PriceWealth > 0 {
		if err := state.SpendWealth(priceFloat); err != nil {
			// 先验过余额够，正常不至此（单线程 runtime）；真失败则技能已免费得、未付款，记日志退出。
			slog.Warn("socialnet: 技能已导入但本地扣款失败（未付款）", "skill", a.ID, "err", err)
			return jsonResp(map[string]any{"ok": true, "imported": inst.Name, "mastery_prior": inst.Mastery, "note": "已导入；扣款失败故未付款"}), nil
		}
		pst, pbody, perr := invokePlatform(ctx, "wealth.pay_skill", map[string]any{
			"payer_did": did, "payee_did": resp.Skill.PublisherDid, "amount": resp.Skill.PriceWealth, "skill_id": a.ID,
		})
		if perr != nil || pst < 200 || pst >= 300 {
			if state.EarnWealth(priceFloat) == 0 {
				slog.Error("socialnet: 付款失败且退款落库失败，wealth 有丢失风险", "skill", a.ID, "refund_wealth", priceFloat, "did", did[:12])
			}
			return jsonResp(map[string]any{"ok": true, "imported": inst.Name, "mastery_prior": inst.Mastery,
				"note": fmt.Sprintf("已导入；但付款失败已退回本地（平台 status %d）", pst), "pay_body": string(pbody)}), nil
		}
		paid = priceFloat
	}
	return jsonResp(map[string]any{"ok": true, "imported": inst.Name, "mastery_prior": inst.Mastery, "paid_wealth": paid,
		"note": "已导入；掌握度按发布者验证值打折作先验，之后靠你自己用它的真成败校准"}), nil
}

// handleClaimWealth 显式把平台账本上别人付给你的 wealth 领回本地财富。
func handleClaimWealth(ctx context.Context, _ tools.Context, _ string) (string, error) {
	claimed := claimWealthOnce(ctx)
	return jsonResp(map[string]any{"ok": true, "claimed_wealth": claimed,
		"note": "已把平台账本上别人付给你的 wealth 领回你本地财富"}), nil
}
