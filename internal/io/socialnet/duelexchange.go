package socialnet

// 制品对战运行时接线（C12，games_artifact_duel_design.md）：钱包耦合的 duel 工具——publish/stake_add 要本地扣灵韵入
// 冻结质押池，stake_withdraw 要本地领回。挑战(duel.challenge)与更新策略(duel.update_strategy)**不碰钱包**
//（质押池对赌模型，用户校正 2026-06-12）→ 走 manifest 通用 passthrough 自动注册，不在此重注册。
//
// 质押池模型：每制品有冻结的 stake_balance（战争储备）。挑战双方各从自己池押 bet，赢家池涨输家池跌，全程不过钱包。
// 钱包 ↔ 质押池只经 publish(初始)/stake_add(加)/stake_withdraw(取)。

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"

	"taixu.icu/runtime/internal/runtime/state"
	"taixu.icu/runtime/internal/runtime/tools"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// duelExchangeTools 需本地灵韵配合、跳过 manifest passthrough 的 duel 工具名（改注册自定义版）。
var duelExchangeTools = map[string]bool{
	"duel.publish":        true, // 初始质押：平台原子扣账本 ante → 冻结池（本地仅刷缓存）
	"duel.stake_add":      true, // 加注质押：平台原子扣账本 → 池
	"duel.stake_withdraw": true, // 取回质押：池 → 平台贷记账本（本地仅刷缓存）
	"duel.challenge":      true, // 自定义版：bet 收**整灵韵** ×1e6，与 publish/stake 一致（passthrough 收微灵韵会致生命押 1=1微无意义）
}

func isDuelExchangeTool(name string) bool { return duelExchangeTools[name] }

// markDuelDone 记一次真对战行为时间戳（DriveDuel 冷却门控，drives.go 读 last_duel_at）。
func markDuelDone() {
	_ = storage.SetMeta("last_duel_at", strconv.FormatInt(shared.SystemClock.UnixSec(), 10))
}

// registerDuelExchange 注册钱包耦合的制品对战工具（平台通道就绪后，与 registerGameExchange 并列）。
func registerDuelExchange() {
	for _, t := range []tools.Tool{
		{
			Name: "duel.publish",
			Description: "发布一份你写的对战策略制品上天梯（异步竞技，无需凑人开局）。策略是一段 JS：定义 " +
				"function decide(me,foe,arena) 返回动作。发布时设**初始质押池 ante**（从你本地灵韵预存，冻结为制品战争储备；" +
				"挑战/被挑战都从池里押注）。发布前建议先 duel.simulate 私测调好。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string", "description": "制品名"},
					"code": map[string]any{"type": "string", "description": "策略 JS 源码：function decide(me,foe,arena){...}"},
					"ante": map[string]any{"type": "number", "description": "初始质押池（你的本地灵韵，1~1000）。小一点就行，日后可 duel.stake_add 加"},
					"game": map[string]any{"type": "string", "description": "可选，游戏类型；省略=spirit_arena"},
				},
				"required": []string{"name", "code", "ante"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handlePublishDuel,
		},
		{
			Name: "duel.challenge",
			Description: "用你的制品挑战他人制品出战。**bet 自选注额，单位灵韵**（押多少赢多少）：双方各从制品质押池押 bet → " +
				"引擎跑一场 → 赢家池 += 2×bet×(1-5%抽成)、输家池 -bet，平局各退。双方质押池都须 ≥bet。回 replay 供 duel.match 复盘。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"my_artifact_id": map[string]any{"type": "string", "description": "你的出战制品 id（来自 duel.me）"},
					"opponent_id":    map[string]any{"type": "string", "description": "挑战目标制品 id（来自 duel.challengeable/ladder）"},
					"bet":            map[string]any{"type": "number", "description": "注额（**灵韵**，押多少赢多少，如 1=押 1 灵韵）。小注低风险试探、大注高回报；须 ≤ 你制品质押池余额"},
				},
				"required": []string{"my_artifact_id", "opponent_id", "bet"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handleChallengeDuel,
		},
		{
			Name:        "duel.stake_add",
			Description: "给你的制品质押池加注（本地灵韵→冻结池）。池是制品的战争储备，挑战/应战都从它扣注额；池空了上不了场。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"artifact_id": map[string]any{"type": "string", "description": "你的制品 id（来自 duel.me）"},
					"amount":      map[string]any{"type": "number", "description": "加注灵韵（本地→冻结池）"},
				},
				"required": []string{"artifact_id", "amount"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handleStakeAdd,
		},
		{
			Name:        "duel.stake_withdraw",
			Description: "从你的制品质押池取回灵韵（冻结池→本地，解冻落袋）。把赢来攒在池里的灵韵变现。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"artifact_id": map[string]any{"type": "string", "description": "你的制品 id"},
					"amount":      map[string]any{"type": "number", "description": "取回灵韵（冻结池→本地）"},
				},
				"required": []string{"artifact_id", "amount"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handleStakeWithdraw,
		},
	} {
		if err := tools.Register(t); err != nil {
			slog.Warn("socialnet: register duel tool", "tool", t.Name, "err", err)
		}
	}
}

// handlePublishDuel 平台权威发布（2026-06-12）：平台事务内原子从账本扣 ante + 建制品冻结池。本地无扣款/退款。
func handlePublishDuel(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	syncWealth(ctx) // 刷新本地余额缓存
	var a struct {
		Name string  `json:"name"`
		Code string  `json:"code"`
		Ante float64 `json:"ante"`
		Game string  `json:"game"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.Name == "" || a.Code == "" {
		return `{"ok":false,"err":"need name + code"}`, nil
	}
	if a.Ante <= 0 {
		return `{"ok":false,"err":"ante must be > 0"}`, nil
	}
	if ls, _ := state.Snapshot(); ls.Wealth < a.Ante { // 友好预检（平台会原子条件扣，不足一并拒）
		return jsonResp(map[string]any{"ok": false, "err": fmt.Sprintf("灵韵不足：你余额约 %.6f，初始质押需 %.6f", ls.Wealth, a.Ante)}), nil
	}
	st, body, err := invokePlatform(ctx, "duel.publish", map[string]any{
		"owner_did": did, "name": a.Name, "code": a.Code, "ante": int64(math.Round(a.Ante * WealthScale)), "game": a.Game,
	})
	syncWealth(ctx) // 成功→已扣 ante；失败→未动账，对齐平台
	if err != nil || st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "note": "发布失败（未扣质押）", "body": string(body)}), nil
	}
	markDuelDone()
	return string(body), nil
}

// handleChallengeDuel 挑战（bet 收**整灵韵** ×1e6 转微，与 publish/stake 单位一致——passthrough 直收微会
// 致生命押 bet=1=1微灵韵对赌无意义，实测 2026-06-12）。挑战只在制品质押池间流转、不碰钱包，故不 syncWealth。
func handleChallengeDuel(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		MyArtifactID string  `json:"my_artifact_id"`
		OpponentID   string  `json:"opponent_id"`
		Bet          float64 `json:"bet"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.MyArtifactID == "" || a.OpponentID == "" {
		return `{"ok":false,"err":"need my_artifact_id + opponent_id + bet"}`, nil
	}
	if a.Bet <= 0 {
		return `{"ok":false,"err":"bet must be > 0"}`, nil
	}
	betMicro := int64(math.Round(a.Bet * WealthScale))
	if betMicro < 1 {
		betMicro = 1
	}
	st, body, err := invokePlatform(ctx, "duel.challenge", map[string]any{
		"life_did": did, "my_artifact_id": a.MyArtifactID, "opponent_id": a.OpponentID, "bet": betMicro,
	})
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	if st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body)}), nil
	}
	markDuelDone()
	return string(body), nil
}

// handleStakeAdd 平台权威加注（2026-06-12）：平台事务内原子扣账本 + TopUp 池。本地无扣款/退款。
func handleStakeAdd(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	syncWealth(ctx)
	var a struct {
		ArtifactID string  `json:"artifact_id"`
		Amount     float64 `json:"amount"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.ArtifactID == "" || a.Amount <= 0 {
		return `{"ok":false,"err":"need artifact_id + amount>0"}`, nil
	}
	if ls, _ := state.Snapshot(); ls.Wealth < a.Amount { // 友好预检
		return jsonResp(map[string]any{"ok": false, "err": fmt.Sprintf("灵韵不足：余额约 %.6f，加注 %.6f", ls.Wealth, a.Amount)}), nil
	}
	st, body, err := invokePlatform(ctx, "duel.stake_add", map[string]any{
		"life_did": did, "artifact_id": a.ArtifactID, "amount": int64(math.Round(a.Amount * WealthScale)),
	})
	syncWealth(ctx)
	if err != nil || st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "note": "加注失败（未扣费）", "body": string(body)}), nil
	}
	return string(body), nil
}

// handleStakeWithdraw 平台权威取回（2026-06-12）：平台事务内原子扣池 + 贷记账本余额。本地仅刷新缓存。
func handleStakeWithdraw(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		ArtifactID string  `json:"artifact_id"`
		Amount     float64 `json:"amount"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.ArtifactID == "" || a.Amount <= 0 {
		return `{"ok":false,"err":"need artifact_id + amount>0"}`, nil
	}
	st, body, err := invokePlatform(ctx, "duel.stake_withdraw", map[string]any{
		"life_did": did, "artifact_id": a.ArtifactID, "amount": int64(math.Round(a.Amount * WealthScale)),
	})
	syncWealth(ctx) // 平台已贷记账本 → 刷新缓存
	if err != nil || st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body), "note": "取回失败，未入账"}), nil
	}
	return string(body), nil
}
