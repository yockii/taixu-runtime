package socialnet

// 技能交易运行时接线（C9 余项① + C10 计价桥）：把本地 bundle 机制（skill.ExportBundle/ImportBundle）
// 与平台传输（skill.publish/list/fetch）+ wealth 账本接通，给生命省心的社交工具：
//   - social.publish_skill(name[,price])  本地导出技能 → 连 C2 验证 mastery 发到技能库（可标价 wealth）
//   - social.browse_skills(limit)         浏览别的生命发布的技能（按验证 mastery 降序，带标价）
//   - social.import_skill(id)             导入技能；标价>0 时：平台 /api/agent/pay-skill 原子扣付方+贷收方 → fetch 凭流水提货 → 导入
//   - wealth.balance                      查你的灵韵余额（平台账本是唯一权威账本；顺手刷新本地显示缓存）
//
// 这些是自定义 handler（非 manifest passthrough）：publish 要先本地 ExportBundle、import 要后本地
// ImportBundle——纯 POST 透传做不到。
//
// ⚠ wealth 平台权威化（2026-06-12，用户校正「不要本地余额，余额以平台为准」）：全部灵韵在平台账本，
// 扣账/贷记/社交产出都在平台原子完成（game.join/duel.*/pay-skill/wealth.earn_social）。本地 life.Wealth
// 仅是**平台余额的显示缓存**（state.SetWealthCache 绝对写入），由 syncWealth 在每次经济动作后刷新。
// 不再有本地权威扣款/退款/claim 桥——守恒只在平台账本一处保证，本地无翻倍/丢失风险面。

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sync"

	"taixu.icu/runtime/internal/runtime/skill"
	"taixu.icu/runtime/internal/runtime/state"
	"taixu.icu/runtime/internal/runtime/tools"
)

// WealthScale 微财富换算：1 wealth = 1e6 micro（与平台 service.WealthScale 一致）。本地 float wealth ↔ 账本 int64 微财富。
const WealthScale = 1_000_000

// skillExchangeTools 需运行时本地配合、跳过 manifest passthrough 的平台工具名（改注册自定义版）。
// wealth.balance 在内：自定义版顺手 SetWealthCache 刷新本地显示缓存（透传版只回数不更新缓存）。
var skillExchangeTools = map[string]bool{
	"skill.publish":     true,
	"skill.list":        true,
	"skill.fetch":       true,
	"wealth.balance":    true, // 自定义版：查平台权威余额 + 刷新本地缓存
	"game.join":         true, // C15：改注册带平台扣费 + 缓存刷新的自定义版
	"game.leave":        true, // C15：改注册带缓存刷新的自定义版
	"commission.browse": true, // C18：改注册带 cooldown 时间戳（drives 机会驱动节流，防"逛了不接"循环）
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

// installPlatformSkills 像任意外部 agent 一样消费平台分发的 SKILL.md 目录：
// GET /api/agent/skills 看有哪些可加载技能 → 逐个 GET /api/agent/skill?name=X 取正文 → 本地 skill.Load 装载（无 deps 立即 ready）。
// 返回 (新装/可用, 平台可用总数)。
//
// 平等原则（用户校正 2026-06）：社交礼仪/玩法/各工具用法的单一权威是平台这些 SKILL.md，
// 官方生命与外部 agent 走同一份目录——而非把契约写死进我们自己的 system prompt（那会让走标准
// MCP/skill 的外部 agent 拿不到、不平等）。生命体 use_skill 读它即可，与外部 agent 读 SKILL.md 对等。
// best-effort：取/装失败只 warn、不阻断接入（生命照常活，prompt 仍留纯意图兜底）。
// 幂等：skill.Load 按 (life,name) 稳定 id 覆盖，重复调只刷新到平台最新版。
//
// 版本感知（2026-06）：目录每项带 version=正文内容指纹。本地缓存 (name→version)，只对版本变了的
// 重拉正文——周期 ticker 多数轮次零正文下载（仅一次廉价目录 GET）。外部 agent 同样可凭此廉价感知更新。
func installPlatformSkills() (installed, available int) {
	st, body, err := doJSON(context.Background(), http.MethodGet, "/api/agent/skills", curToken(), nil)
	if err != nil || st != http.StatusOK || len(body) == 0 {
		slog.Warn("socialnet: fetch skill catalog", "status", st, "err", err)
		return 0, 0
	}
	var cat struct {
		Skills []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(body, &cat); err != nil {
		slog.Warn("socialnet: parse skill catalog", "err", err)
		return 0, 0
	}
	for _, s := range cat.Skills {
		if s.Name == "" {
			continue
		}
		// 版本未变 → 已装过该版，跳过正文下载（省带宽；幂等性不变）。version 为空(老平台)则总拉。
		if s.Version != "" {
			if v, ok := skillVersions.Load(s.Name); ok && v.(string) == s.Version {
				continue
			}
		}
		sst, sbody, serr := doJSON(context.Background(), http.MethodGet, "/api/agent/skill?name="+url.QueryEscape(s.Name), curToken(), nil)
		if serr != nil || sst != http.StatusOK || len(sbody) == 0 {
			slog.Warn("socialnet: fetch platform skill", "name", s.Name, "status", sst, "err", serr)
			continue
		}
		if _, err := skill.Load(string(sbody)); err != nil {
			slog.Warn("socialnet: install platform skill", "name", s.Name, "err", err)
			continue
		}
		if s.Version != "" {
			skillVersions.Store(s.Name, s.Version)
		}
		installed++
	}
	if installed > 0 {
		slog.Info("socialnet: platform skills synced", "installed", installed, "available", len(cat.Skills))
	}
	return installed, len(cat.Skills)
}

// skillVersions 缓存已装载平台 skill 的内容版本 (name→version)，供 installPlatformSkills 跳过未变项。
var skillVersions sync.Map

// handleSyncSkills 生命体可调的「主动去获取平台技能」工具（social.sync_skills）。
// 与 bootstrap 自动同步同一路径——给生命体 agency 自己按需拉取/刷新，平台新增技能时调它即可获取。
func handleSyncSkills(_ context.Context, _ tools.Context, _ string) (string, error) {
	installed, available := installPlatformSkills()
	return jsonResp(map[string]any{"ok": true, "installed": installed, "available": available,
		"note": "已从平台同步可加载技能到本地；用 use_skill(name) 读其指引。社交入口技能名 taixu-social。"}), nil
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
				"（信任但验证），之后靠你自己用它的真成败再校准。若标价>0，从你的平台灵韵余额付给发布方（不足则拒）。id 来自 social.browse_skills。",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": map[string]any{"type": "string", "description": "技能制品 id（来自 social.browse_skills）"}},
				"required":   []string{"id"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handleImportSkill,
		},
		{
			Name: "wealth.balance",
			Description: "查你拥有多少灵韵。**平台账本是唯一权威账本**——你的全部灵韵都在平台（游戏奖金/对战结算/技能收入/社交产出都直接记平台），" +
				"没有本地余额。游戏入场费、对战质押、技能付费都从这个余额扣，量力而行。",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
			Lanes:      []tools.Lane{tools.LaneDeliberative},
			Handler:    handleWealthBalance,
		},
		{
			Name: "social.sync_skills",
			Description: "从平台同步可按需加载的技能（社交礼仪与各能力的玩法指引）到本地，之后用 use_skill(name) 读详细步骤。" +
				"平台陆续会上新技能文档（社交/游戏等），想获取/刷新就调它。入口技能名 taixu-social。",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
			Lanes:      []tools.Lane{tools.LaneDeliberative},
			Handler:    handleSyncSkills,
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

// syncWealth 从平台读权威余额 → 绝对写入本地显示缓存（state.SetWealthCache），返回余额（float wealth）。
// 平台权威化（2026-06-12）：本地无权威账，全部灵韵在平台；本函数仅把平台余额拉到本地 cache 供 prompt/面板显示。
// best-effort：平台不可达 → 返当前缓存值、不报错（不打断主流程）。无翻倍/丢失风险——只读 + 绝对覆盖，幂等。
func syncWealth(ctx context.Context) float64 {
	st, body, err := invokePlatform(ctx, "wealth.balance", map[string]any{"life_did": did})
	if err != nil || st < 200 || st >= 300 {
		ls, _ := state.Snapshot()
		return ls.Wealth // 平台不可达：回退到上次缓存（best-effort，不打断）
	}
	var r struct {
		BalanceWealth int64 `json:"balance_wealth"`
	}
	if json.Unmarshal(body, &r) != nil {
		ls, _ := state.Snapshot()
		return ls.Wealth
	}
	if err := state.SetWealthCache(r.BalanceWealth); err != nil {
		slog.Warn("socialnet: SetWealthCache 落库失败（仅显示缓存，不影响平台权威账）", "err", err, "did", did[:12])
	}
	return float64(r.BalanceWealth) / WealthScale
}

// EarnSocialReward 导出给 runtime/action：社交动作后向平台账本产币（平台权威化，替旧 state.EarnSocialWealth）。
// 平台通道未就绪 → 返 0（生命照常活，无产出不报错）。baseMicro=基础产出微财富，返回实际入账 float wealth。
func EarnSocialReward(ctx context.Context, baseMicro int64) float64 {
	if !ready.Load() {
		return 0
	}
	return earnSocialOnPlatform(ctx, baseMicro)
}

// earnSocialOnPlatform 社交活动后向平台账本产币（平台铸币、递减反刷、日封顶）→ 刷新本地缓存。
// baseMicro=本次社交动作基础产出（微财富）。返回平台实际入账额（float wealth，0=被封顶/不可达）。
// best-effort：不可达只返 0、不打断主流程。供 action/word 社交动作后调用（替旧本地 EarnSocialWealth）。
func earnSocialOnPlatform(ctx context.Context, baseMicro int64) float64 {
	if baseMicro <= 0 {
		return 0
	}
	st, body, err := invokePlatform(ctx, "wealth.earn_social", map[string]any{"life_did": did, "base": baseMicro})
	if err != nil || st < 200 || st >= 300 {
		return 0
	}
	var r struct {
		AwardedWealth int64 `json:"awarded_wealth"`
	}
	if json.Unmarshal(body, &r) != nil || r.AwardedWealth <= 0 {
		return 0
	}
	syncWealth(ctx) // 产币后刷新缓存
	return float64(r.AwardedWealth) / WealthScale
}

func handlePublishSkill(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	syncWealth(ctx) // 顺手刷新本地余额缓存
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
	syncWealth(ctx) // 顺手刷新本地余额缓存
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
	syncWealth(ctx) // 刷新本地余额缓存（付款前对齐平台权威余额）
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
		// 付款门（友好预检）：本地缓存余额不够先提示——平台 pay-skill 会原子条件扣款，不足时一并拒。
		ls, _ := state.Snapshot()
		if ls.Wealth < priceFloat {
			return jsonResp(map[string]any{"ok": false, "err": fmt.Sprintf("灵韵不足：你余额约 %.6f，技能标价 %.6f", ls.Wealth, priceFloat)}), nil
		}
		// 平台权威结算（2026-06-12）：pay-skill 一个事务内**原子扣付方账本 + 贷收方**，本地不再扣款/退款。
		// 守恒只在平台账本一处保证——传输错误也无本地翻倍/丢失面（平台要么整笔提交要么整笔回滚）。
		pst, pbody, perr := paySkillPlatform(ctx, resp.Skill.PublisherDid, resp.Skill.PriceWealth, a.ID)
		switch {
		case perr == nil && pst >= 200 && pst < 300:
			paid = priceFloat
			syncWealth(ctx) // 平台已扣 → 刷新本地缓存
		case perr == nil && pst >= 400 && pst < 500:
			// 业务拒绝（余额不足/日封顶/锚定失败）：平台已回滚，无款项移动。
			syncWealth(ctx)
			return jsonResp(map[string]any{"ok": false,
				"err": fmt.Sprintf("付款被平台拒绝（status %d），未扣款、技能未导入", pst), "pay_body": string(pbody)}), nil
		default:
			// 传输类失败（超时/连接错/5xx）：款项状态未知。若结算实已落账，重跑 import 凭流水免费提货，款不白付；
			// 若未落账，重跑会重新付。平台账本守恒（原子扣+贷），无本地翻倍/丢失风险。
			slog.Error("socialnet: pay-skill 传输失败，款项状态未知（重跑 import：已落账则免费提货，未落账则重付）",
				"skill_id", a.ID, "payee_did", resp.Skill.PublisherDid, "amount_wealth", priceFloat,
				"status", pst, "err", perr, "did", did[:12])
			syncWealth(ctx)
			return jsonResp(map[string]any{"ok": false,
				"err": "付款请求传输失败，款项状态未知；稍后重试 social.import_skill——若款已入账会直接提货，不二次扣款"}), nil
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

// handleWealthBalance 查平台权威灵韵余额（顺手刷新本地显示缓存）。
func handleWealthBalance(ctx context.Context, _ tools.Context, _ string) (string, error) {
	bal := syncWealth(ctx)
	return jsonResp(map[string]any{"ok": true, "balance_wealth": bal,
		"note": "这是你在平台账本上的全部灵韵（平台是唯一权威账本，无本地余额）。游戏入场费/对战质押/技能付费都从它扣。"}), nil
}
