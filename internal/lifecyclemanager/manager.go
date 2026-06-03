// Package lifecyclemanager 宏观状态机（docs/03 §4）。
//
// Phase 0 七态（无 Transferred）：
//   Embryonic / Active / LowPower / Dormant / Archived / Detached / Memorial
//
// 合法迁移（Phase 0）：
//   Embryonic -> Active
//   Active <-> LowPower
//   Active -> Dormant
//   LowPower -> Dormant
//   Dormant -> Active
//   Active / LowPower / Dormant -> Archived
//   Active / LowPower / Dormant -> Detached
//   Archived -> Memorial
//   Detached -> Memorial
//
// 写权限：独享 lifecycle_state / lifecycle_history。
package lifecyclemanager

import (
	"fmt"

	"mindverse/internal/core"
	"mindverse/internal/eventbus"
	"mindverse/internal/memoryengine"
	"mindverse/internal/shared"
)

// allowed 合法迁移：from -> 一组 to。
var allowed = map[core.LifecycleState]map[core.LifecycleState]bool{
	core.StateEmbryonic: {core.StateActive: true},
	core.StateActive:    {core.StateLowPower: true, core.StateDormant: true, core.StateArchived: true, core.StateDetached: true},
	core.StateLowPower:  {core.StateActive: true, core.StateDormant: true, core.StateArchived: true, core.StateDetached: true},
	core.StateDormant:   {core.StateActive: true, core.StateArchived: true, core.StateDetached: true},
	core.StateArchived:  {core.StateMemorial: true},
	core.StateDetached:  {core.StateMemorial: true},
	core.StateMemorial:  {}, // 终态
}

// Manager 状态机管理器。
type Manager struct {
	store *memoryengine.Store
	bus   *eventbus.Bus
}

// New 构造。
func New(store *memoryengine.Store, bus *eventbus.Bus) *Manager {
	return &Manager{store: store, bus: bus}
}

// Current 读取当前状态。
func (m *Manager) Current(lifeID string) (core.LifecycleState, error) {
	s, _, err := m.store.LoadLifecycleState(lifeID)
	return s, err
}

// Transition 尝试迁移。如非法返回错误。
func (m *Manager) Transition(lifeID string, to core.LifecycleState, reason string) error {
	from, _, err := m.store.LoadLifecycleState(lifeID)
	if err != nil {
		return fmt.Errorf("load current: %w", err)
	}
	if !IsAllowed(from, to) {
		return fmt.Errorf("illegal transition %s -> %s", from, to)
	}
	now := shared.SystemClock.UnixSec()
	if err := m.store.UpsertLifecycleState(lifeID, from, to, now, reason); err != nil {
		return fmt.Errorf("persist transition: %w", err)
	}
	m.bus.Publish(eventbus.LifecycleTransitioned{
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
