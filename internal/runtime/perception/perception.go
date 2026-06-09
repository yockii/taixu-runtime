// Package perception 感知层（docs/03 §2.1）单例。
//
// 聚合 ExternalRequest 队列 + 状态快照 → Frame 给下游 9 步循环。
package perception

import (
	"fmt"
	"sync"
	"time"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/runtime/state"
)

// ExternalRequest 外部请求。
type ExternalRequest struct {
	ID         string
	Channel    string // "feishu" / "cli" / "web"
	From       string
	Content    string
	ReceivedAt time.Time
}

// Frame 单次循环的感知聚合。
type Frame struct {
	CycleID    int64
	Externals  []ExternalRequest
	Life       core.LifeState
	Mental     core.MentalState
	PerceiveAt int64
}

// SummaryLine 简短描述（写工作记忆 "perceive.summary" 槽）。
func (f Frame) SummaryLine() string {
	return fmt.Sprintf("cycle=%d ext=%d energy=%.2f stress=%.2f anxiety=%.2f",
		f.CycleID, len(f.Externals), f.Life.Energy, f.Life.Stress, f.Mental.Anxiety)
}

var (
	mu    sync.Mutex
	queue []ExternalRequest
)

// Inject 外部请求入队（IM / CLI / Web 调用）。
func Inject(r ExternalRequest) {
	mu.Lock()
	queue = append(queue, r)
	mu.Unlock()
}

// Pending 当前队列长度。
func Pending() int {
	mu.Lock()
	defer mu.Unlock()
	return len(queue)
}

// UserPresent 简化判定：队列非空 ⇒ 用户在场。
func UserPresent() bool { return Pending() > 0 }

// Perceive 收集本轮感知并 drain 队列。
func Perceive(cycleID int64) Frame {
	mu.Lock()
	externals := queue
	queue = nil
	mu.Unlock()
	life, mental := state.Snapshot()
	return Frame{
		CycleID:    cycleID,
		Externals:  externals,
		Life:       life,
		Mental:     mental,
		PerceiveAt: time.Now().Unix(),
	}
}
