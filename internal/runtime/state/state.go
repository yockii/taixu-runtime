// Package state 独占 LifeState / MentalState 写入（docs/04 §2.1 / TECH-STACK §13.2）。
//
// 包级单例：进程内仅一个生命体。Init 加载初值，Apply Δ + clamp [0,1] + 持久化 + 广播 StateChanged。
package state

import (
	"errors"
	"sync"

	"taixu.icu/runtime/internal/bus"
	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// Delta 状态变更建议。nil 字段不变。
type Delta struct {
	Energy       *float64
	Competence   *float64
	SocialNeed   *float64
	Stress       *float64
	Confidence   *float64
	Stability    *float64
	Motivation   *float64
	Satisfaction *float64
	Anxiety      *float64
	Reason       string
}

// StateChanged 状态实际落库后广播。
type StateChanged struct {
	LifeID string
	Life   core.LifeState
	Mental core.MentalState
	Reason string
}

var (
	mu     sync.Mutex
	lifeID string
	life   core.LifeState
	mental core.MentalState
)

// Init 加载初始 LifeState / MentalState（必先 storage.Init 完成）。
func Init(id string) error {
	if id == "" {
		return errors.New("state: empty life id")
	}
	ls, err := storage.LoadLifeState(id)
	if err != nil {
		return err
	}
	ms, err := storage.LoadMentalState(id)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	life = *ls
	mental = *ms
	return nil
}

// Snapshot 当前快照（拷贝）。
func Snapshot() (core.LifeState, core.MentalState) {
	mu.Lock()
	defer mu.Unlock()
	return life, mental
}

// Apply 应用 Δ：clamp + 持久化 + 广播。
func Apply(d Delta) error {
	mu.Lock()
	defer mu.Unlock()
	now := shared.SystemClock.UnixSec()

	applyDelta(&life.Energy, d.Energy)
	applyDelta(&life.Competence, d.Competence)
	applyDelta(&life.SocialNeed, d.SocialNeed)
	applyDelta(&life.Stress, d.Stress)
	applyDelta(&life.Confidence, d.Confidence)
	applyDelta(&life.Stability, d.Stability)
	life.UpdatedAt = now

	applyDelta(&mental.Motivation, d.Motivation)
	applyDelta(&mental.Satisfaction, d.Satisfaction)
	applyDelta(&mental.Anxiety, d.Anxiety)
	mental.UpdatedAt = now

	if err := storage.UpsertLifeState(&life); err != nil {
		return err
	}
	if err := storage.UpsertMentalState(&mental); err != nil {
		return err
	}

	bus.Publish(StateChanged{LifeID: lifeID, Life: life, Mental: mental, Reason: d.Reason})
	return nil
}

// ResetEnergyDailyCap 重置日精力上限（ledger 模块触发）。
func ResetEnergyDailyCap(newCap float64, nextResetAt int64) error {
	if newCap < 0 || newCap > 1 {
		return errors.New("cap out of [0,1]")
	}
	mu.Lock()
	defer mu.Unlock()
	life.EnergyDailyCap = newCap
	life.EnergyUsedToday = 0
	life.CapResetAt = nextResetAt
	life.UpdatedAt = shared.SystemClock.UnixSec()
	return storage.UpsertLifeState(&life)
}

func applyDelta(field *float64, delta *float64) {
	if delta == nil {
		return
	}
	v := *field + *delta
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	*field = v
}
