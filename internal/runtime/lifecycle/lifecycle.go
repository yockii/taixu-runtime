// Package lifecycle 宏观状态机（docs/03 §4）单例。
//
// Phase 0 七态：Embryonic / Active / LowPower / Dormant / Archived / Detached / Memorial
//
// 合法迁移：
//   Embryonic -> Active
//   Active <-> LowPower
//   Active / LowPower -> Dormant
//   Dormant -> Active
//   Active / LowPower / Dormant -> Archived
//   Active / LowPower / Dormant -> Detached
//   Archived -> Memorial
//   Detached -> Memorial
//
// 写权限：独占 lifecycle_state / lifecycle_history（通过 storage 包）。
package lifecycle

import (
	"errors"
	"fmt"

	"taixu.icu/runtime/internal/bus"
	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

var allowed = map[core.LifecycleState]map[core.LifecycleState]bool{
	core.StateEmbryonic: {core.StateActive: true},
	core.StateActive:    {core.StateLowPower: true, core.StateDormant: true, core.StateArchived: true, core.StateDetached: true},
	core.StateLowPower:  {core.StateActive: true, core.StateDormant: true, core.StateArchived: true, core.StateDetached: true},
	core.StateDormant:   {core.StateActive: true, core.StateArchived: true, core.StateDetached: true},
	core.StateArchived:  {core.StateMemorial: true},
	core.StateDetached:  {core.StateMemorial: true},
	core.StateMemorial:  {},
}

var (
	lifeID string
)

// Init 绑定本生命体 ID。
func Init(id string) error {
	if id == "" {
		return errors.New("lifecycle: empty life id")
	}
	lifeID = id
	return nil
}

// LifeID 当前绑定的生命体 ID。
func LifeID() string { return lifeID }

// Current 读取当前宏观状态。
func Current() (core.LifecycleState, error) {
	s, _, err := storage.LoadLifecycleState(lifeID)
	return s, err
}

// Transition 尝试迁移；非法返回错误。
func Transition(to core.LifecycleState, reason string) error {
	from, _, err := storage.LoadLifecycleState(lifeID)
	if err != nil {
		return fmt.Errorf("load current: %w", err)
	}
	if !IsAllowed(from, to) {
		return fmt.Errorf("illegal transition %s -> %s", from, to)
	}
	now := shared.SystemClock.UnixSec()
	if err := storage.UpsertLifecycleState(lifeID, from, to, now, reason); err != nil {
		return fmt.Errorf("persist transition: %w", err)
	}
	bus.Publish(bus.LifecycleTransitioned{
		LifeID:    lifeID,
		FromState: string(from),
		ToState:   string(to),
		Reason:    reason,
	})
	return nil
}

// IsAllowed 是否允许 from -> to。
func IsAllowed(from, to core.LifecycleState) bool {
	if from == to {
		return false
	}
	tos, ok := allowed[from]
	if !ok {
		return false
	}
	return tos[to]
}
