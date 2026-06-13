package shared

import "sync"

// GameObligationMarker 嵌在游戏义务目标 payload 里的稳定标记串（轮到你的回合 describe/vote）：
//   - 主循环据它去重（HasOpenGoalWithPayloadSubstring）避免每 cycle 重复入队；
//   - action 层据它识别「游戏义务目标」→ 过程零数值消耗（认真游戏，结算延到整局 resolved）。
// 放 shared（叶子包）供 drives/action/main 共用，避免 action→drives 反向依赖。
const GameObligationMarker = "进行中的对局待办"

// GamePending 一局进行中对局对本生命体的待办摘要（心跳 socialnet.PollGames 从平台 game.tend 拉来缓存，
// drives.Derive 读它发 DriveGame）。只含本人视角，绝不含他人词/角色。
type GamePending struct {
	SessionID string `json:"session_id"`
	GameType  string `json:"game_type"`
	State     string `json:"state"`
	Phase     string `json:"phase"`
	RoundNo   int    `json:"round_no"`
	YourWord  string `json:"your_word"`
	YourRole  string `json:"your_role"`
	Deadline  string `json:"deadline"` // 平台来的 RFC3339 截止串；""=无
	Pot       int64  `json:"pot"`
}

var (
	gamePendingMu sync.RWMutex
	gamePending   []GamePending
)

// SetGamePending 心跳刷新本生命体进行中对局待办缓存（替换全量）。
func SetGamePending(p []GamePending) {
	gamePendingMu.Lock()
	defer gamePendingMu.Unlock()
	gamePending = p
}

// GetGamePending 读当前缓存（拷贝，drives 用）。
func GetGamePending() []GamePending {
	gamePendingMu.RLock()
	defer gamePendingMu.RUnlock()
	out := make([]GamePending, len(gamePending))
	copy(out, gamePending)
	return out
}
