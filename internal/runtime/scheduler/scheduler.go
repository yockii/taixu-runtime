// Package scheduler 自适应循环节拍（docs/03 §1.1）单例。
//
// 节拍因子（v1）：
//   × 0.3  Energy>0.7 + UserPresent
//   × 0.5  Energy>0.7
//   × 1.0  默认
//   × 2.0  Energy<0.3
//   × 4.0  Energy<0.1 或 LowPower
//
// 边界 [1s, 30min]。
package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"taixu.icu/runtime/internal/bus"
	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/runtime/lifecycle"
	"taixu.icu/runtime/internal/runtime/perception"
	"taixu.icu/runtime/internal/runtime/state"
	"taixu.icu/runtime/internal/storage"
)

const (
	MinInterval  = 1 * time.Second
	MaxInterval  = 30 * time.Minute
	BaseInterval = 60 * time.Second
)

var (
	mu      sync.Mutex
	lifeID  string
	cycleID int64
)

// Init 绑定生命体 + 续接 cycle_id。
func Init(id string) error {
	if id == "" {
		return errors.New("scheduler: empty life id")
	}
	last, err := storage.MaxCycleID(id)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	cycleID = last
	return nil
}

// CycleID 当前 cycle_id（不递增；仅读）。
func CycleID() int64 {
	mu.Lock()
	defer mu.Unlock()
	return cycleID
}

// Run 阻塞启动循环。onTick 由调用方注入：9 步循环编排。
func Run(ctx context.Context, onTick func(cycleID int64)) error {
	for {
		life, _ := state.Snapshot()
		lc, _ := lifecycle.Current()
		iv := nextInterval(life, lc)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(iv):
		}

		if lc == core.StateDormant || lc == core.StateArchived || lc == core.StateDetached || lc == core.StateMemorial {
			continue
		}

		mu.Lock()
		cycleID++
		current := cycleID
		mu.Unlock()

		bus.Publish(bus.TickStarted{LifeID: lifeID, CycleID: current})
		onTick(current)
		bus.Publish(bus.TickFinished{LifeID: lifeID, CycleID: current})
	}
}

func nextInterval(life core.LifeState, lc core.LifecycleState) time.Duration {
	factor := 1.0
	switch {
	case lc == core.StateLowPower || life.Energy < 0.1:
		factor = 4.0
	case life.Energy < 0.3:
		factor = 2.0
	case life.Energy > 0.7 && perception.UserPresent():
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
