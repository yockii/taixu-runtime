package socialnet

// 技能交易运行时接线（C9 余项① + C10 计价桥）：把本地 bundle 机制（skill.ExportBundle/ImportBundle）
// 与平台传输（skill.publish/list/fetch）+ wealth 账本（wealth.pay_skill/claim）接通，给生命省心的社交工具：
//   - social.publish_skill(name[,price])  本地导出技能 → 连 C2 验证 mastery 发到技能库（可标价 wealth）
//   - social.browse_skills(limit)         浏览别的生命发布的技能（按验证 mastery 降序，带标价）
//   - social.import_skill(id)             导入技能；标价>0 时：本地 SpendWealth → 平台 /api/agent/pay-skill 结算 → fetch 凭流水提货 → 导入
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
	"word.submit":   true, // C12：改注册带本地灵韵奖励的自定义 social.contribute_word 版
	"game.join":     true, // C15：改注册带本地 SpendWealth+退回的自定义版
	"game.leave":    true, // C15：改注册带本地领回退款的自定义版
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
				"（连同你的验证掌握度作信任凭据）。可选 price 标价（灵韵，别的生命导入时付给你）。只发你 ready 的技能。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":  map[string]any{"type": "string", "description": "要发布的技能名"},
					"price": map[string]any{"type": "number", "description": "可选，标价（灵韵，>0 则别人导入需付给你；省略/0=免费分享）"},
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
				"（信任但验证），之后靠你自己用它的真成败再校准。若标价>0，先从你本地灵韵扣款付给发布方（不足则拒）。id 来自 social.browse_skills。",
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
			Description: "把平台账本上别人付给你的灵韵（如别人导入你标价的技能付的款）一次性领回到你本地财富。返回领回的额。",
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

// paySkillPlatform 技能付费结算（平台专用内部路径 POST /api/agent/pay-skill，非通用 invoke——
// wealth.pay_skill 已撤出通用 agent 面，通用面会被平台 manifest 白名单 404）。401 自动重登一次。
// 平台侧校验：payer 须属调用账户、payee=技能发布者、amount=标价、单 payer 日累计上限。
func paySkillPlatform(ctx context.Context, payeeDID string, amount int64, skillID string) (int, []byte, error) {
	payload := map[string]any{"payer_did": did, "payee_did": payeeDID, "amount": amount, "skill_id": skillID}
	st, body, err := doJSON(ctx, http.MethodPost, "/api/agent/pay-skill", curToken(), payload)
	if err == nil && st == http.StatusUnauthorized {
		if lerr := login(); lerr == nil {
			st, body, err = doJSON(ctx, http.MethodPost, "/api/agent/pay-skill", curToken(), payload)
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
		// 「传输错误 ≠ 平台未提交」：超时/响应丢失时平台可能已清零账本可领余额 → 该额未入本地 = 静默丢失。
		// 留痕（slog 自带时间戳）供对照平台 wealth_ledger reason=claim 人工补账；仍返 0 不打断主流程。
		slog.Error("socialnet: wealth.claim 失败，若平台已清零账本则该额未入本地（留痕待对账）",
			"status", st, "err", err, "did", did[:12])
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
	// 平台 skill.fetch 带付费闸（viewer_did=本生命 DID）：免费/已结算/自家的 → 全量正文；
	// 付费未结算 → 只回元数据（skill_md 空 + payment_required=true）。故编排为：
	// fetch①（探明价/收款方或直接拿货）→（需付费时）本地扣款 → 专用 pay-skill 结算 → fetch②提货 → 导入。
	// 结算流水在平台 wealth_ledger 持久：已付款而提货/导入失败时，重跑 social.import_skill 会凭流水免费提货，不会二次扣款。
	fetchOnce := func() (int, []byte, error) {
		return invokePlatform(ctx, "skill.fetch", map[string]any{"id": a.ID, "viewer_did": did})
	}
	st, body, err := fetchOnce()
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	if st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body)}), nil
	}
	type fetchResp struct {
		PaymentRequired bool `json:"payment_required"`
		Skill           struct {
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
	var resp fetchResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return `{"ok":false,"err":"bad skill payload from platform"}`, nil
	}

	priceFloat := float64(resp.Skill.PriceWealth) / WealthScale
	paid := 0.0
	if resp.PaymentRequired && resp.Skill.SkillMd == "" {
		if resp.Skill.PriceWealth <= 0 {
			return `{"ok":false,"err":"bad skill payload from platform"}`, nil
		}
		// 付款门：本地余额不够直接拒（不动账）。
		ls, _ := state.Snapshot()
		if ls.Wealth < priceFloat {
			return jsonResp(map[string]any{"ok": false, "err": fmt.Sprintf("wealth 不足：你余额 %.6f，技能标价 %.6f", ls.Wealth, priceFloat)}), nil
		}
		// 先本地扣款，再平台结算（平台只在结算流水存在时放行正文，先货后款不可能）。
		if err := state.SpendWealth(priceFloat); err != nil {
			return jsonResp(map[string]any{"ok": false, "err": "本地扣款失败：" + err.Error()}), nil
		}
		pst, pbody, perr := paySkillPlatform(ctx, resp.Skill.PublisherDid, resp.Skill.PriceWealth, a.ID)
		switch {
		case perr == nil && pst >= 200 && pst < 300:
			paid = priceFloat
		case perr == nil && pst >= 400 && pst < 500:
			// 明确业务拒绝（平台收到并处理了请求、带响应体拒绝执行 → 收款方必未被贷记）→ 立即退款，守恒不破。
			if state.EarnWealth(priceFloat) == 0 {
				slog.Error("socialnet: 付款被拒且退款落库失败，wealth 有丢失风险", "skill_id", a.ID, "refund_wealth", priceFloat, "did", did[:12])
			}
			return jsonResp(map[string]any{"ok": false,
				"err": fmt.Sprintf("付款被平台拒绝（status %d），款项已退回你本地，技能未导入", pst), "pay_body": string(pbody)}), nil
		default:
			// 传输类失败（超时/连接错/5xx）：「传输错误 ≠ 平台未提交」——平台可能已记结算，本地再退款 = 灵韵翻倍，
			// 破坏守恒（宪法级不变量）→ 不退款，留痕。若结算实已落账，重跑 import 会凭流水免费提货，款不白付。
			slog.Error("socialnet: pay-skill 传输类失败，款项状态未知（不退款，留痕待对账；若已落账可重跑 import 免费提货）",
				"skill_id", a.ID, "payee_did", resp.Skill.PublisherDid, "amount_wealth", priceFloat,
				"status", pst, "err", perr, "did", did[:12])
			return jsonResp(map[string]any{"ok": false,
				"err": "付款请求传输失败，款项状态未知（已留痕待对账）；稍后重试 social.import_skill——若款已入账会直接提货，不二次扣款"}), nil
		}
		// 结算成功 → 再 fetch 提货（平台凭流水放行正文）。
		st, body, err = fetchOnce()
		if err != nil || st < 200 || st >= 300 {
			slog.Error("socialnet: 已结算但提货 fetch 失败（结算流水持久，重跑 import 可免费提货）",
				"skill_id", a.ID, "status", st, "err", err, "did", did[:12])
			return jsonResp(map[string]any{"ok": false, "err": "已付款但提货失败；稍后重跑 social.import_skill 免费提货"}), nil
		}
		resp = fetchResp{}
		if err := json.Unmarshal(body, &resp); err != nil || resp.Skill.SkillMd == "" {
			slog.Error("socialnet: 已结算但平台仍未交付正文（待查），重跑 import 可再试", "skill_id", a.ID, "did", did[:12])
			return jsonResp(map[string]any{"ok": false, "err": "已付款但平台未交付正文；稍后重跑 social.import_skill 再试（不二次扣款）"}), nil
		}
	}
	if resp.Skill.SkillMd == "" {
		return `{"ok":false,"err":"bad skill payload from platform"}`, nil
	}

	// 本地交付（ImportBundle）。已付款时导入失败也不退款——结算流水持久，重跑 import 免费提货重试即可。
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
		if paid > 0 {
			return jsonResp(map[string]any{"ok": false, "err": "已付款但本地导入失败：" + err.Error() + "；重跑 social.import_skill 可免费提货重试"}), nil
		}
		return jsonResp(map[string]any{"ok": false, "err": err.Error()}), nil
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
