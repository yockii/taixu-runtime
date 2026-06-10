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

// AddEnergyUsed 累加今日已耗精力（认知开销，按 token→energy 折算；R106 死字段复活）。
//
// 与 life.Energy（当前体力标量，gate 行为、自回血）不同：EnergyUsedToday 是当日累计
// 认知支出的只增计量，由 ledger.MaybeResetEnergyDailyCap 每日清零。当前仅供观测/面板，
// 尚不作硬闸（token→cap 折算公式 Phase 1 校准，R45）。amount 应为正（折算后的 energy 量）。
func AddEnergyUsed(amount float64) error {
	if amount <= 0 {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	life.EnergyUsedToday += amount
	life.UpdatedAt = shared.SystemClock.UnixSec()
	return storage.UpsertLifeState(&life)
}

// ResetEnergyDailyCap 重置日精力上限（ledger 模块触发）。一并清当日社交产 wealth 计数（反刷递减按日重置）。
func ResetEnergyDailyCap(newCap float64, nextResetAt int64) error {
	if newCap < 0 || newCap > 1 {
		return errors.New("cap out of [0,1]")
	}
	mu.Lock()
	defer mu.Unlock()
	life.EnergyDailyCap = newCap
	life.EnergyUsedToday = 0
	life.SocialWealthToday = 0
	life.CapResetAt = nextResetAt
	life.UpdatedAt = shared.SystemClock.UnixSec()
	return storage.UpsertLifeState(&life)
}

// 社交活动产 wealth 反刷参数（C10 / R109）。回报系数 Phase 0.5 实测标定，先用保守值。
const socialWealthDiminishK = 0.5 // 递减斜率：awarded = base / (1 + k×当日已产)，今日产越多单次越小

// EarnSocialWealth 据本次社交活动产出 wealth（C10 / 06 §3.1.2）。**递减反刷**：base 经当日已产
// wealth 折减（刷得越多单次越少），floor 0、无上界（wealth 非 [0,1] 标量，不走 Delta clamp）。
// 返回本次实际入账的 wealth。base<=0 不产。
//
// slice1 边界：仅按本生命**自身社交动作等级**给基础产出 + 递减闸。声誉/信任档加权、被回应/被采纳
// 追加、跨所有权链门（§3.1.2 余项）需平台反馈，留后续 slice。
func EarnSocialWealth(base float64) float64 {
	if base <= 0 {
		return 0
	}
	mu.Lock()
	defer mu.Unlock()
	awarded := base / (1 + socialWealthDiminishK*life.SocialWealthToday)
	life.Wealth += awarded
	life.SocialWealthToday += awarded
	life.UpdatedAt = shared.SystemClock.UnixSec()
	if err := storage.UpsertLifeState(&life); err != nil {
		// 入账失败回滚内存，保持一致。
		life.Wealth -= awarded
		life.SocialWealthToday -= awarded
		return 0
	}
	return awarded
}

// EarnWealth 直接入账 wealth（C10 计价桥：收方从平台账本 Claim 回本地的回流，或导入付款失败的退款）。
// 与 EarnSocialWealth 区别：**无递减、不计 social_wealth_today**——这不是社交活动产出，是已属本生命的
// wealth 从账本回流本地。amount<=0 不动账。只增、无上界（wealth 非 [0,1] 标量，不走 Delta clamp）。
// 返回实际入账额；落库失败回滚返 0。
func EarnWealth(amount float64) float64 {
	if amount <= 0 {
		return 0
	}
	mu.Lock()
	defer mu.Unlock()
	life.Wealth += amount
	life.UpdatedAt = shared.SystemClock.UnixSec()
	if err := storage.UpsertLifeState(&life); err != nil {
		life.Wealth -= amount // 落库失败回滚内存，保持与 DB 一致
		return 0
	}
	return amount
}

// SpendWealth 花 wealth（C10：未来技能/物品交易扣款入口）。余额不足返错、不动账。floor 0。
func SpendWealth(amount float64) error {
	if amount <= 0 {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if life.Wealth < amount {
		return errors.New("state: insufficient wealth")
	}
	life.Wealth -= amount
	life.UpdatedAt = shared.SystemClock.UnixSec()
	if err := storage.UpsertLifeState(&life); err != nil {
		life.Wealth += amount // 落库失败回滚内存，保持与 DB 一致
		return err
	}
	return nil
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
