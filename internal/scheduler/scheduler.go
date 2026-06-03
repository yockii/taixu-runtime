// Package scheduler 自适应循环节拍（docs/03 §1.1 / 05 §1）。
//
// Phase 0.2 节拍函数（v1，待 0.5 长跑标定）：
//
//   base interval = 60s
//   factor =
//     × 0.3  if Energy > 0.7 and (UserPresent or PendingExternal)
//     × 0.5  if Energy > 0.7
//     × 1.0  default
//     × 2.0  if Energy < 0.3
//     × 4.0  if Energy < 0.1 or LowPower
//
// 边界：[1s, 30min]。
package scheduler

import (
	"context"
	"time"

	"mindverse/internal/core"
	"mindverse/internal/eventbus"
	"mindverse/internal/memoryengine"
	"mindverse/internal/statemanager"
)

const (
	MinInterval = 1 * time.Second
	MaxInterval = 30 * time.Minute
	BaseInterval = 60 * time.Second
)

// PendingProvider 返回当前外部请求 / 用户在场标志。
type PendingProvider interface {
	UserPresent() bool
	PendingExternal() int
}

// nullPending 默认无外部输入。
type nullPending struct{}

func (nullPending) UserPresent() bool   { return false }
func (nullPending) PendingExternal() int { return 0 }

// Scheduler 循环驱动器。
type Scheduler struct {
	bus     *eventbus.Bus
	store   *memoryengine.Store
	state   *statemanager.Manager
	lifeID  string
	lcState func() core.LifecycleState
	pending PendingProvider
	cycleID int64
}

// New 构造。lcStateFn 用于读当前宏观状态（决定 Dormant/LowPower 时停跳）。
func New(bus *eventbus.Bus, store *memoryengine.Store, state *statemanager.Manager, lifeID string,
	lcStateFn func() core.LifecycleState, pending PendingProvider) *Scheduler {
	if pending == nil {
		pending = nullPending{}
	}
	return &Scheduler{
		bus:     bus,
		store:   store,
		state:   state,
		lifeID:  lifeID,
		lcState: lcStateFn,
		pending: pending,
	}
}

// Resume 从 raw_trail 续接 cycle_id。
func (s *Scheduler) Resume() error {
	last, err := s.store.MaxCycleID(s.lifeID)
	if err != nil {
		return err
	}
	s.cycleID = last
	return nil
}

// Run 启动循环。每次 tick 前重新计算间隔，发 TickStarted / TickFinished 事件。
// onTick 由 runtime 主程注入：9 步循环编排。
func (s *Scheduler) Run(ctx context.Context, onTick func(cycleID int64)) error {
	for {
		life, _ := s.state.Snapshot()
		lc := s.lcState()
		iv := nextInterval(life, lc, s.pending)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(iv):
		}

		if lc == core.StateDormant || lc == core.StateArchived || lc == core.StateDetached || lc == core.StateMemorial {
			continue
		}

		s.cycleID++
		s.bus.Publish(eventbus.TickStarted{LifeID: s.lifeID, CycleID: s.cycleID})
		onTick(s.cycleID)
		s.bus.Publish(eventbus.TickFinished{LifeID: s.lifeID, CycleID: s.cycleID})
	}
}

func nextInterval(life core.LifeState, lc core.LifecycleState, pending PendingProvider) time.Duration {
	factor := 1.0
	switch {
	case lc == core.StateLowPower || life.Energy < 0.1:
		factor = 4.0
	case life.Energy < 0.3:
		factor = 2.0
	case life.Energy > 0.7 && (pending.UserPresent() || pending.PendingExternal() > 0):
		factor = 0.3
	case life.Energy > 0.7:
		factor = 0.5
	}
	iv := time.Duration(float64(BaseInterval) * factor)
	if iv < MinInterval {
		iv = MinInterval
	}
	if iv > MaxInterval {
		iv = MaxInterval
	}
	return iv
}
