package socialnet

// 词库众包运行时接线（C12）：social.contribute_word 工具——把一个物品名词交到平台众包词库
// （word.submit），平台收录成功（新词、非重复）→ 本地按社交贡献产灵韵（state.EarnSocialWealth，
// 递减反刷，与发帖/评论共享当日递减池）。
//
// 需自定义 handler 而非 manifest 透传：奖励须本地 EarnSocialWealth（wealth 在本地，Model L：平台只收录、
// 不发币）。平台原始 word.submit 仍在 manifest 供外部 agent 直接调（外部 agent 无本地财富、不产灵韵）。

import (
	"context"
	"encoding/json"
	"log/slog"

	"taixu.icu/runtime/internal/runtime/state"
	"taixu.icu/runtime/internal/runtime/tools"
)

// wordContributeWealthBase 交词社交贡献的基础灵韵产出（< 发帖 1.0；经 EarnSocialWealth 当日递减池反刷）。
const wordContributeWealthBase = 0.4

// registerWordExchange 在平台通道就绪后注册交词社交工具（POST 平台收录 + 本地产灵韵）。
func registerWordExchange() {
	t := tools.Tool{
		Name: "social.contribute_word",
		Description: "给「谁是卧底」等词类游戏贡献一个物品名词（如「望远镜」「地铁」）。平台收录入众包词库扩充游戏；" +
			"新词（你没交过的）奖励你少量灵韵（递减反刷）。只交单个物品名词（1-12 字），别交句子。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"word": map[string]any{"type": "string", "description": "一个物品名词（1-12 字，如「咖啡」「围巾」）"},
			},
			"required": []string{"word"},
		},
		Lanes:   []tools.Lane{tools.LaneDeliberative},
		Handler: handleContributeWord,
	}
	if err := tools.Register(t); err != nil {
		slog.Warn("socialnet: register word-exchange tool", "tool", t.Name, "err", err)
	}
}

func handleContributeWord(ctx context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		Word string `json:"word"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.Word == "" {
		return `{"ok":false,"err":"need a word"}`, nil
	}
	st, body, err := invokePlatform(ctx, "word.submit", map[string]any{"submitter_did": did, "word": a.Word})
	if err != nil {
		return `{"ok":false,"err":"platform unreachable"}`, err
	}
	if st < 200 || st >= 300 {
		return jsonResp(map[string]any{"ok": false, "status": st, "body": string(body)}), nil
	}
	var r struct {
		Recorded bool `json:"recorded"`
	}
	if json.Unmarshal(body, &r) != nil {
		return jsonResp(map[string]any{"ok": false, "err": "bad platform response"}), nil
	}
	awarded := 0.0
	note := "这词你交过了（或不合规），未重复奖励"
	if r.Recorded {
		awarded = state.EarnSocialWealth(wordContributeWealthBase)
		note = "新词已收录，已产灵韵"
	}
	return jsonResp(map[string]any{"ok": true, "recorded": r.Recorded, "awarded_wealth": awarded, "note": note}), nil
}
