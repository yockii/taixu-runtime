// Package resourceledger 五资源账本（docs/06）。
//
// Phase 0 仅启用：energy / knowledge
// EnergyDailyCap 周期重置由本模块发起 → StateManager 真正落库。
package resourceledger

import (
	"errors"
	"sync"

	"mindverse/internal/memoryengine"
	"mindverse/internal/shared"
	"mindverse/internal/statemanager"
)

// Resource 类型常量。
const (
	Energy     = "energy"
	Knowledge  = "knowledge"
	Wealth     = "wealth"     // Phase 3+
	Reputation = "reputation" // Phase 3+
	Social     = "social"     // Phase 4+
)

// Ledger 资源账本。
type Ledger struct {
	store *memoryengine.Store
	state *statemanager.Manager

	mu      sync.Mutex
	lifeID  string
	balance map[string]float64
}

// New 构造。从持久化账本恢复 balance。
func New(store *memoryengine.Store, state *statemanager.Manager, lifeID string) (*Ledger, error) {
	l := &Ledger{
		store:   store,
		state:   state,
		lifeID:  lifeID,
		balance: map[string]float64{},
	}
	for _, r := range []string{Energy, Knowledge} {
		v, err := store.SumLedger(lifeID, r)
		if err != nil {
			return nil, err
		}
		l.balance[r] = v
	}
	return l, nil
}

// Spend 花费某资源（amount > 0）。
// 对 energy：同时通过 StateManager.ConsumeEnergy 反映到 LifeState.Energy（dual write 防误用）。
func (l *Ledger) Spend(resource string, amount float64, reason, sourceKind, sourceRef string) error {
	if amount < 0 {
		return errors.New("amount must be >= 0")
	}
	return l.append(resource, -amount, reason, sourceKind, sourceRef)
}

// Earn 获得某资源（amount > 0）。
func (l *Ledger) Earn(resource string, amount float64, reason, sourceKind, sourceRef string) error {
	if amount < 0 {
		return errors.New("amount must be >= 0")
	}
	return l.append(resource, amount, reason, sourceKind, sourceRef)
}

// Balance 返回某资源当前余额（来自累加缓存）。
func (l *Ledger) Balance(resource string) float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.balance[resource]
}

// MaybeResetEnergyDailyCap 若达到重置时间则重置 cap，返回是否重置。
func (l *Ledger) MaybeResetEnergyDailyCap() (bool, error) {
	life, _ := l.state.Snapshot()
	now := shared.SystemClock.UnixSec()
	if now < life.CapResetAt {
		return false, nil
	}
	const day = int64(24 * 3600)
	next := ((now / day) + 1) * day
	// Phase 0 cap 公式：基线 1.0；后续 Phase 0.5 标定按 Stress / Stability 调整。
	if err := l.state.ResetEnergyDailyCap(1.0, next); err != nil {
		return false, err
	}
	return true, nil
}

func (l *Ledger) append(resource string, delta float64, reason, sourceKind, sourceRef string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	newBalance := l.balance[resource] + delta
	now := shared.SystemClock.UnixSec()
	if err := l.store.AppendLedger(l.lifeID, resource, delta, newBalance, reason, sourceKind, sourceRef, now); err != nil {
		return err
	}
	l.balance[resource] = newBalance
	return nil
}
