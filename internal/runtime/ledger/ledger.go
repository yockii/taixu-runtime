// Package ledger 资源账本（docs/06）单例。
//
// Phase 0 仅 energy / knowledge。其他在 Phase 3+。
// EnergyDailyCap 周期重置由本包发起 → state 落库。
package ledger

import (
	"errors"
	"sync"

	"taixu.icu/runtime/internal/runtime/state"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

const (
	Energy     = "energy"
	Knowledge  = "knowledge"
	Wealth     = "wealth"     // Phase 3+
	Reputation = "reputation" // Phase 3+
	Social     = "social"     // Phase 4+
)

var (
	mu      sync.Mutex
	lifeID  string
	balance = make(map[string]float64)
)

// Init 加载已有 balance。
func Init(id string) error {
	if id == "" {
		return errors.New("ledger: empty life id")
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	balance = make(map[string]float64)
	for _, r := range []string{Energy, Knowledge} {
		v, err := storage.SumLedger(id, r)
		if err != nil {
			return err
		}
		balance[r] = v
	}
	return nil
}

// Spend 花费（amount > 0）。
func Spend(resource string, amount float64, reason, sourceKind, sourceRef string) error {
	if amount < 0 {
		return errors.New("amount must be >= 0")
	}
	return appendLedger(resource, -amount, reason, sourceKind, sourceRef)
}

// Earn 获得（amount > 0）。
func Earn(resource string, amount float64, reason, sourceKind, sourceRef string) error {
	if amount < 0 {
		return errors.New("amount must be >= 0")
	}
	return appendLedger(resource, amount, reason, sourceKind, sourceRef)
}

// Balance 当前余额。
func Balance(resource string) float64 {
	mu.Lock()
	defer mu.Unlock()
	return balance[resource]
}

// MaybeResetEnergyDailyCap 若达重置时间则重置 cap。返回是否重置。
func MaybeResetEnergyDailyCap() (bool, error) {
	life, _ := state.Snapshot()
	now := shared.SystemClock.UnixSec()
	if now < life.CapResetAt {
		return false, nil
	}
	const day = int64(24 * 3600)
	next := ((now / day) + 1) * day
	if err := state.ResetEnergyDailyCap(1.0, next); err != nil {
		return false, err
	}
	return true, nil
}

func appendLedger(resource string, delta float64, reason, sourceKind, sourceRef string) error {
	mu.Lock()
	defer mu.Unlock()
	newBalance := balance[resource] + delta
	now := shared.SystemClock.UnixSec()
	if err := storage.AppendLedger(lifeID, resource, delta, newBalance, reason, sourceKind, sourceRef, now); err != nil {
		return err
	}
	balance[resource] = newBalance
	return nil
}
