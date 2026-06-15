package shared

import "sync"

// CommissionDeliverMarker 嵌在「交付已接委托」义务目标 payload 里的稳定标记串：
//   - 主循环据它去重（避免每 cycle 重复入队）；
//   - drives 据它发交付义务驱动。
// 放 shared（叶子包）供 drives/main 共用。
const CommissionDeliverMarker = "交付你接下的委托"

// ActiveCommission 本生命已接（claimed/delivered）的委托摘要（心跳 socialnet.PollCommissions 从平台
// commission.mine 拉来缓存）。CloneURL 含 token（仅本进程内存，drives 拼进交付义务目标供 git.clone）。
type ActiveCommission struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	State    string `json:"state"` // claimed / delivered
	Brief    string `json:"brief"`
	CloneURL string `json:"git_clone_url"` // 带 token，git.clone 用
	RepoFull string `json:"repo_full"`
}

var (
	commMu        sync.RWMutex
	commOpenCount int                // 市场上当前开放（可接）委托数（commission.browse）
	commActive    []ActiveCommission // 本生命已接、未结的委托（commission.mine）
)

// SetCommissionState 心跳刷新委托缓存（开放数 + 本生命活跃委托）。
func SetCommissionState(openCount int, active []ActiveCommission) {
	commMu.Lock()
	defer commMu.Unlock()
	commOpenCount = openCount
	commActive = active
}

// CommissionOpenCount 读当前市场开放委托数（drives 机会驱动用）。
func CommissionOpenCount() int {
	commMu.RLock()
	defer commMu.RUnlock()
	return commOpenCount
}

// GetActiveCommissions 读本生命活跃委托（拷贝，drives 义务驱动用）。
func GetActiveCommissions() []ActiveCommission {
	commMu.RLock()
	defer commMu.RUnlock()
	out := make([]ActiveCommission, len(commActive))
	copy(out, commActive)
	return out
}
