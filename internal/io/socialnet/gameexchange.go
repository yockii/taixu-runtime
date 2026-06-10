package socialnet

// 游戏参与运行时接线（C15）：钱耦合(本地 SpendWealth/EarnWealth)的 game.join/leave 自定义工具 +
// 心跳 PollGames（平台 game.tend → 缓存进行中待办供 DriveGame + 赢局 mental 接线 + 顺手领奖）。
//
// describe/vote 是平台 passthrough 工具（无钱耦合，makeHandler 自动注入 actor_did），不在此重注册。
// join/leave 必须自定义：入场要本地先扣灵韵(Model L)、失败退回；离场后领回退款——纯 POST 透传做不到。

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"taixu.icu/runtime/internal/runtime/state"
	"taixu.icu/runtime/internal/runtime/tools"
	"taixu.icu/runtime/internal/shared"
)

// ackedResolved 已发过终局 mental 奖励的 session（防心跳每轮重复加分）。进程内即可：
// 重启后某局可能再加一次小满足，clamp[0,1] 下无害；奖金领取本身幂等（claim 已领返 0）。
// mutex 守：pollGamesOnce 现仅 cycle 单 goroutine 调，但加锁防未来并发写 map（Go 并发写 map=panic）。
var (
	ackedMu       sync.Mutex
	ackedResolved = map[string]bool{}
)

// ackResolvedOnce 原子地标记一个 session 已发奖励；返回 true 表示本次首发（应发），false=已发过。
func ackResolvedOnce(sessionID string) bool {
	ackedMu.Lock()
	defer ackedMu.Unlock()
	if ackedResolved[sessionID] {
		return false
	}
	ackedResolved[sessionID] = true
	return true
}

// registerGameExchange 注册钱耦合的游戏参与工具（平台通道就绪后，与 registerSkillExchange 并列）。
func registerGameExchange() {
	for _, t := range []tools.Tool{
		{
			Name: "game.join",
			Description: "加入一局游戏（如《谁是卧底》）。给 game_type 自动撮合（找有空位的局或新建一局），或给 session_id 加指定局。" +
				"先从你本地灵韵付入场费(entry_fee)进奖池、胜方平分；加入失败自动退回。加入后用 game.tend 看你的词和待办。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"game_type":  map[string]any{"type": "string", "description": "游戏类型撮合加入（如 undercover）；与 session_id 二选一"},
					"session_id": map[string]any{"type": "string", "description": "加指定局（来自 game.open_games）；与 game_type 二选一"},
				},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handleJoinGame,
		},
		{
			Name:        "game.leave",
			Description: "开局前离开一局大厅，退回你的入场灵韵。开局后不可退。",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"session_id": map[string]any{"type": "string", "description": "要离开的对局 id"}},
				"required":   []string{"session_id"},
			},
			Lanes:   []tools.Lane{tools.LaneDeliberative},
			Handler: handleLeaveGame,
		},
	} {
		if err := tools.Register(t); err != nil {
			slog.Warn("socialnet: register game tool", "tool", t.Name, "err", err)
		}
	}
}

// gameEntryFee 查某游戏类型入场费（微灵韵），game_type 撮合加入前预取。
func gameEntryFee(ctx context.Context, gameType string) (int64, error) {
	st, body, err := invokePlatform(ctx, "game.config", map[string]any{"game_type": gameType})
	if err != nil || st < 200 || st >= 300 {
		return 0, fmt.Errorf("查游戏配置失败")
	}
	var r struct {
		Config struct {
			EntryFee int64 `json:"entry_fee"`
		} `json:"config"`
	}
	if json.Unmarshal(body, &r) != nil {
		return 0, fmt.Errorf("游戏配置解析失败")
	}
	return r.Config.EntryFee, nil
}

// gameLobbyFee 查指定大厅的实际入场费 + 游戏类型（join 指定 session_id 时按该局真实费率扣，避免币种/费率不符）。
// 该局非开放大厅（已开局/不存在）→ 报错，避免在不可加入的局上预扣灵韵。
func gameLobbyFee(ctx context.Context, sessionID string) (gameType string, fee int64, err error) {
	st, body, e := invokePlatform(ctx, "game.open_games", map[string]any{"limit": 100})
	if e != nil || st < 200 || st >= 300 {
		return "", 0, fmt.Errorf("查开放大厅失败")
	}
	var r struct {
		Games []struct {
			ID       string `json:"id"`
			GameType string `json:"game_type"`
			EntryFee int64  `json:"entry_fee"`
		} `json:"games"`
	}
	if json.Unmarshal(body, &r) != nil {
		return "", 0, fmt.Errorf("大厅列表解析失败")
	}
	for _, g := range r.Games {
		if g.ID == sessionID {
			return g.GameType, g.EntryFee, nil
		}
	}
	return "", 0, fmt.Errorf("该对局不在开放大厅（已开局或不存在），无法加入")
}

func handleJoinGame(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	claimWealthOnce(ctx) // 顺手领回挂账（上局奖金/退款），让余额最新
	var a struct {
		GameType  string `json:"game_type"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || (a.GameType == "" && a.SessionID == "") {
		return `{"ok":false,"err":"need game_type or session_id"}`, nil
	}
	// 入场费：按 session_id 取该局**真实**费率（防默认猜测致币种/费率不符）；否则按 game_type 撮合费率。
	var (
		gt  = a.GameType
		fee int64
		err error
	)
	if a.SessionID != "" {
		var gtFound string
		if gtFound, fee, err = gameLobbyFee(ctx, a.SessionID); err != nil {
			return jsonResp(map[string]any{"ok": false, "err": err.Error()}), nil
		}
		gt = gtFound
	} else {
		if fee, err = gameEntryFee(ctx, gt); err != nil {
			return jsonResp(map[string]any{"ok": false, "err": err.Error()}), nil
		}
	}
	feeFloat := float64(fee) / WealthScale
	if ls, _ := state.Snapshot(); ls.Wealth < feeFloat {
		return jsonResp(map[string]any{"ok": false, "err": fmt.Sprintf("灵韵不足：你余额 %.6f，入场费 %.6f", ls.Wealth, feeFloat)}), nil
	}
	// 本地先扣（Model L）→ 平台 join（统一传 gt：撮合 or 该局类型）→ 失败退回。
	if err := state.SpendWealth(feeFloat); err != nil {
		return jsonResp(map[string]any{"ok": false, "err": "扣费失败：" + err.Error()}), nil
	}
	st, body, err := invokePlatform(ctx, "game.join", map[string]any{
		"life_did": did, "game_type": gt, "session_id": a.SessionID,
	})
	if err != nil || st < 200 || st >= 300 {
		if state.EarnWealth(feeFloat) == 0 {
			slog.Error("socialnet: game.join 失败且退费落库失败，灵韵有丢失风险", "fee", feeFloat, "did", did[:12])
		}
		if err != nil {
			return `{"ok":false,"err":"platform unreachable"}`, err
		}
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body), "note": "加入失败，入场费已退回"}), nil
	}
	// 加入成功 → 玩感增益（参与游戏=社交+满足；纾解社交需求）。
	sat, sn := 0.04, -0.05
	_ = state.Apply(state.Delta{Satisfaction: &sat, SocialNeed: &sn, Reason: "game.join"})
	pollGamesOnce(ctx) // 立即刷新待办缓存，让 DriveGame 接上本局
	return string(body), nil
}

func handleLeaveGame(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.SessionID == "" {
		return `{"ok":false,"err":"need session_id"}`, nil
	}
	st, body, err := invokePlatform(ctx, "game.leave", map[string]any{"life_did": did, "session_id": a.SessionID})
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	if st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body)}), nil
	}
	claimWealthOnce(ctx)  // 领回退款到本地
	pollGamesOnce(ctx)    // 刷新缓存（已离场则该局从待办消失）
	return string(body), nil
}

// PollGames 心跳（每 cognitive cycle 维护期调）：平台 game.tend → 缓存进行中待办供 DriveGame +
// 处理近期终局（赢局 satisfaction/social/confidence 增益、参与给小满足；赢了顺手领奖）。
// 非阻塞 best-effort：通道未就绪/平台不可达 → 软返回不崩（同 socialnet 降级）。
func PollGames(ctx context.Context) {
	if !ready {
		return
	}
	pollGamesOnce(ctx)
}

func pollGamesOnce(ctx context.Context) {
	st, body, err := invokePlatform(ctx, "game.tend", map[string]any{"life_did": did})
	if err != nil || st < 200 || st >= 300 {
		return
	}
	var r struct {
		Pending      []shared.GamePending `json:"pending"`
		JustResolved []struct {
			SessionID string `json:"session_id"`
			Won       bool   `json:"won"`
		} `json:"just_resolved"`
	}
	if json.Unmarshal(body, &r) != nil {
		return
	}
	shared.SetGamePending(r.Pending)
	// 终局 mental 接线（每局只加一次，ackedResolved 去重）。
	anyResolved := false
	for _, jr := range r.JustResolved {
		if jr.SessionID == "" || !ackResolvedOnce(jr.SessionID) {
			continue
		}
		anyResolved = true
		if jr.Won {
			sat, sn, conf := 0.12, -0.06, 0.04 // 赢局：满足/纾解社交/长信心
			_ = state.Apply(state.Delta{Satisfaction: &sat, SocialNeed: &sn, Confidence: &conf, Reason: "game.win:" + jr.SessionID})
		} else {
			sat := 0.03 // 参与过也有小满足
			_ = state.Apply(state.Delta{Satisfaction: &sat, Reason: "game.played:" + jr.SessionID})
		}
	}
	if anyResolved {
		claimWealthOnce(ctx) // 赢了奖金已挂账，领回本地
	}
}
