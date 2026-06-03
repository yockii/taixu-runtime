// Package statemanager 独占 life_state / mental_state 写入权（docs/04 §2.1 / TECH-STACK §13.2）。
//
// 其他模块仅通过 EventBus 发"建议变更"事件，由 StateManager 收集并应用。
// 直接调用 store.UpsertLifeState/UpsertMentalState 视为违规。
package statemanager

import (
	"errors"
	"sync"

	"mindverse/internal/core"
	"mindverse/internal/eventbus"
	"mindverse/internal/memoryengine"
	"mindverse/internal/shared"
)

// Delta 表示一次状态变更建议。任意字段为 nil 表示不变。
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

// StateChanged 事件：状态实际落库后广播。
type StateChanged struct {
	LifeID    string
	Life      core.LifeState
	Mental    core.MentalState
	Reason    string
}

// Manager 状态管理器。
type Manager struct {
	store *memoryengine.Store
	bus   *eventbus.Bus

	mu     sync.Mutex
	lifeID string
	life   core.LifeState
	mental core.MentalState
}

// New 加载初始 LifeState/MentalState。
func New(store *memoryengine.Store, bus *eventbus.Bus, lifeID string) (*Manager, error) {
	ls, err := store.LoadLifeState(lifeID)
	if err != nil {
		return nil, err
	}
	ms, err := store.LoadMentalState(lifeID)
	if err != nil {
		return nil, err
	}
	return &Manager{store: store, bus: bus, lifeID: lifeID, life: *ls, mental: *ms}, nil
}

// Snapshot 返回当前快照（拷贝）。
func (m *Manager) Snapshot() (core.LifeState, core.MentalState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.life, m.mental
}

// Apply 应用一次 Delta：clamp [0,1] + 持久化 + 广播 StateChanged。
func (m *Manager) Apply(d Delta) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := shared.SystemClock.UnixSec()

	applyDelta(&m.life.Energy, d.Energy)
	applyDelta(&m.life.Competence, d.Competence)
	applyDelta(&m.life.SocialNeed, d.SocialNeed)
	applyDelta(&m.life.Stress, d.Stress)
	applyDelta(&m.life.Confidence, d.Confidence)
	applyDelta(&m.life.Stability, d.Stability)
	m.life.UpdatedAt = now

	applyDelta(&m.mental.Motivation, d.Motivation)
	applyDelta(&m.mental.Satisfaction, d.Satisfaction)
	applyDelta(&m.mental.Anxiety, d.Anxiety)
	m.mental.UpdatedAt = now

	if err := m.store.UpsertLifeState(&m.life); err != nil {
		return err
	}
	if err := m.store.UpsertMentalState(&m.mental); err != nil {
		return err
	}

	m.bus.Publish(StateChanged{
		LifeID: m.lifeID,
		Life:   m.life,
		Mental: m.mental,
		Reason: d.Reason,
	})
	return nil
}

// ResetEnergyDailyCap 重置日精力上限（ResourceLedger 在 cap 周期触发时调）。
func (m *Manager) ResetEnergyDailyCap(newCap float64, nextResetAt int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if newCap < 0 || newCap > 1 {
		return errors.New("cap out of [0,1]")
	}
	m.life.EnergyDailyCap = newCap
	m.life.EnergyUsedToday = 0
	m.life.CapResetAt = nextResetAt
	m.life.UpdatedAt = shared.SystemClock.UnixSec()
	return m.store.UpsertLifeState(&m.life)
}

// ConsumeEnergy 直接消耗能量（ResourceLedger 桥接）。
func (m *Manager) ConsumeEnergy(amount float64, reason string) error {
	if amount < 0 {
		return errors.New("negative consumption")
	}
	delta := -amount
	d := Delta{Energy: &delta, Reason: reason}
	return m.Apply(d)
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
