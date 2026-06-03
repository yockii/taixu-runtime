// Package perception 感知层（docs/03 §2.1）。
//
// 聚合三类感知源：
//   1. ExternalRequest 队列（CLI / 飞书 IM 注入；Phase 0.3 接通）
//   2. 系统事件（如 cap 重置、节拍变化）
//   3. 自身内部状态变化（取自 StateManager 快照）
//
// 输出 PerceptionFrame 给 9 步循环下游模块。
package perception

import (
	"fmt"
	"sync"
	"time"

	"mindverse/internal/core"
	"mindverse/internal/statemanager"
)

// ExternalRequest 外部请求（用户消息 / 系统调度）。
type ExternalRequest struct {
	ID         string    // 请求 ID（IM 消息 ID / CLI 序号）
	Channel    string    // "feishu" / "cli" / "web" 等
	From       string    // 来源标识（用户 ID）
	Content    string    // 文本
	ReceivedAt time.Time // 入队时间
}

// PerceptionFrame 单次循环的感知聚合。
type PerceptionFrame struct {
	CycleID    int64
	Externals  []ExternalRequest
	Life       core.LifeState
	Mental     core.MentalState
	PerceiveAt int64 // unix sec
}

// SummaryLine 简短描述：写工作记忆 "perceive.summary" 槽用。
func (f PerceptionFrame) SummaryLine() string {
	return fmt.Sprintf("cycle=%d ext=%d energy=%.2f stress=%.2f anxiety=%.2f",
		f.CycleID, len(f.Externals), f.Life.Energy, f.Life.Stress, f.Mental.Anxiety)
}

// Perceiver 感知聚合器。线程安全的 Inbox。
type Perceiver struct {
	state *statemanager.Manager

	mu    sync.Mutex
	queue []ExternalRequest
}

// New 构造。
func New(state *statemanager.Manager) *Perceiver {
	return &Perceiver{state: state}
}

// Inject 由 IMAdapter / CLI / Web 调用，入队一条外部请求。
func (p *Perceiver) Inject(r ExternalRequest) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queue = append(p.queue, r)
}

// Pending 当前队列长度（Scheduler PendingProvider 实现）。
func (p *Perceiver) Pending() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.queue)
}

// UserPresent 简化判定：队列非空 ⇒ 用户在场。Phase 0.3+ 可改为基于最近活跃时间。
func (p *Perceiver) UserPresent() bool { return p.Pending() > 0 }

// PendingExternal 同 Pending，为 Scheduler 接口命名一致。
func (p *Perceiver) PendingExternal() int { return p.Pending() }

// Perceive 收集本轮感知。drain 队列。
func (p *Perceiver) Perceive(cycleID int64) PerceptionFrame {
	p.mu.Lock()
	externals := p.queue
	p.queue = nil
	p.mu.Unlock()

	life, mental := p.state.Snapshot()
	return PerceptionFrame{
		CycleID:    cycleID,
		Externals:  externals,
		Life:       life,
		Mental:     mental,
		PerceiveAt: time.Now().Unix(),
	}
}
