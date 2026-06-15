package socialnet

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"

	"taixu.icu/runtime/internal/runtime/tools"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// registerCommissionExchange 注册自定义 commission.browse（passthrough + 打 cooldown 时间戳）。
// 其余委托工具（claim/deliver/mine）走 manifest passthrough（无需本地耦合）。平台通道就绪后调。
func registerCommissionExchange() {
	t := tools.Tool{
		Name: "commission.browse",
		Description: "逛委托市场：看人类发布了哪些可接的真钱委托（星屑赏金）。任务多为小而可验的活——写文/调研/" +
			"整理数据集/翻译/代码。返回每条带 id/标题/说明/赏金 reward_stardust/截止 deadline。看到能做好的用 commission.claim 接。",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"limit": map[string]any{"type": "integer", "description": "返回数量，默认 30"}},
		},
		Lanes:   []tools.Lane{tools.LaneDeliberative},
		Handler: handleBrowseCommissions,
	}
	if err := tools.Register(t); err != nil {
		slog.Warn("socialnet: register commission.browse", "err", err)
	}
}

func handleBrowseCommissions(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	if a.Limit <= 0 {
		a.Limit = 30
	}
	st, body, err := invokePlatform(ctx, "commission.browse", map[string]any{"limit": a.Limit})
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	// 打 cooldown 时间戳：逛过即重置，drives 机会驱动据此节流（防"逛了不接"每 cycle 重复发）。
	_ = storage.SetMeta("last_commission_browse_at", strconv.FormatInt(shared.SystemClock.UnixSec(), 10))
	if st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body)}), nil
	}
	return string(body), nil
}

// commissionpoll.go — 委托市场心跳（镜像 PollGames）：拉「市场开放委托数」+「本生命已接未结委托」缓存，
// 供 drives 发机会驱动（去接活）/ 义务驱动（交付已接的）。best-effort 非阻塞，通道未通即软返回。
// [[project_commission_git_substrate]] [[project_commission_market_monetization]]

// PollCommissions 每 cognitive cycle 维护期调：commission.browse 数开放委托 + commission.mine 取本生命
// 活跃委托（claimed/delivered，带 token clone URL）→ 缓存。通道未就绪/不可达 → 软返回不崩。
func PollCommissions(ctx context.Context) {
	if !ready.Load() {
		return
	}
	open := browseOpenCount(ctx)
	active := myActiveCommissions(ctx)
	shared.SetCommissionState(open, active)
}

// browseOpenCount 数市场上当前可接的开放委托。
func browseOpenCount(ctx context.Context) int {
	st, body, err := invokePlatform(ctx, "commission.browse", map[string]any{"limit": 30})
	if err != nil || st < 200 || st >= 300 {
		return 0
	}
	var r struct {
		Count int `json:"count"`
	}
	if json.Unmarshal(body, &r) != nil {
		return 0
	}
	return r.Count
}

// myActiveCommissions 取本生命已接、未结（claimed/delivered）的委托 + 其带 token clone URL（供交付恢复）。
func myActiveCommissions(ctx context.Context) []shared.ActiveCommission {
	// 须显式传 claimer_did：PollCommissions 直调 invokePlatform，不走 makeHandler 的 inject 注入。
	st, body, err := invokePlatform(ctx, "commission.mine", map[string]any{"limit": 30, "claimer_did": did})
	if err != nil || st < 200 || st >= 300 {
		return nil
	}
	var r struct {
		Commissions []struct {
			Commission struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Brief    string `json:"brief"`
				State    string `json:"state"`
				RepoFull string `json:"repo_full"`
			} `json:"commission"`
			GitCloneURL string `json:"git_clone_url"`
		} `json:"commissions"`
	}
	if json.Unmarshal(body, &r) != nil {
		return nil
	}
	var out []shared.ActiveCommission
	for _, c := range r.Commissions {
		if c.Commission.State != "claimed" && c.Commission.State != "delivered" {
			continue // 只关心未结的（settled/refunded 无需再动）
		}
		out = append(out, shared.ActiveCommission{
			ID: c.Commission.ID, Title: c.Commission.Title, State: c.Commission.State,
			Brief: c.Commission.Brief, CloneURL: c.GitCloneURL, RepoFull: c.Commission.RepoFull,
		})
	}
	return out
}
