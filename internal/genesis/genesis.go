// Package genesis 生命体出生流程。
//
// 设计纪律（docs/03 §1.2 / docs/04 §2.1）：
//   - 仅在首次启动时调用一次
//   - 写权限：独享 genome 表；同时写 life_state / mental_state / values 初始值 + lifecycle_state Embryonic
//   - 出生后转 lifecycle_state Embryonic→Active 由 LifecycleManager 决定
package genesis

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand/v2"

	"mindverse/internal/core"
	"mindverse/internal/memoryengine"
	"mindverse/internal/shared"
)

// Bear 让一只新生命体出生：生成 Genome + 初始 LifeState/MentalState/Values + 写 Embryonic 状态。
// 返回新生命体 ID。
func Bear(store *memoryengine.Store) (string, error) {
	if has, err := store.HasGenome(); err != nil {
		return "", fmt.Errorf("check existing genome: %w", err)
	} else if has {
		return "", fmt.Errorf("genome already exists; cannot re-bear")
	}

	rng := cryptoSeededRNG()
	now := shared.SystemClock.UnixSec()
	lifeID := shared.NewLifeID()

	g := &core.Genome{
		LifeID:        lifeID,
		Curiosity:     rng.Float64(),
		Sociability:   rng.Float64(),
		Creativity:    rng.Float64(),
		Persistence:   rng.Float64(),
		RiskTaking:    rng.Float64(),
		Empathy:       rng.Float64(),
		BornAt:        now,
		GenomeVersion: "v1",
	}

	if err := store.InsertGenome(g); err != nil {
		return "", fmt.Errorf("insert genome: %w", err)
	}

	// 初始 LifeState：满能量、低焦虑、中性其它。
	ls := &core.LifeState{
		LifeID:          lifeID,
		Energy:          1.0,
		Competence:      0.1,
		SocialNeed:      0.3 + 0.4*g.Sociability,
		Stress:          0.0,
		Confidence:      0.5,
		Stability:       0.7,
		EnergyDailyCap:  1.0,
		EnergyUsedToday: 0.0,
		CapResetAt:      nextDayBoundary(now),
		UpdatedAt:       now,
	}
	if err := store.UpsertLifeState(ls); err != nil {
		return "", fmt.Errorf("seed life_state: %w", err)
	}

	ms := &core.MentalState{
		LifeID:       lifeID,
		Motivation:   0.4 + 0.4*g.Curiosity,
		Satisfaction: 0.5,
		Anxiety:      0.1,
		UpdatedAt:    now,
	}
	if err := store.UpsertMentalState(ms); err != nil {
		return "", fmt.Errorf("seed mental_state: %w", err)
	}

	// 初始价值观：由 Genome 衍生。
	initial := map[string]float64{
		core.ValueGrowth:      0.4 + 0.4*g.Curiosity,
		core.ValueFriendship:  0.3 + 0.5*g.Sociability,
		core.ValueCreativity:  0.3 + 0.5*g.Creativity,
		core.ValueSafety:      0.7 - 0.4*g.RiskTaking,
		core.ValueExploration: 0.3 + 0.4*g.Curiosity + 0.2*g.RiskTaking,
		core.ValueHonesty:     0.5 + 0.3*g.Empathy,
	}
	for name, w := range initial {
		if w < 0 {
			w = 0
		}
		if w > 1 {
			w = 1
		}
		if err := store.UpsertValue(lifeID, name, w, now); err != nil {
			return "", fmt.Errorf("seed value %s: %w", name, err)
		}
	}

	if err := store.UpsertLifecycleState(lifeID, "", core.StateEmbryonic, now, "genesis"); err != nil {
		return "", fmt.Errorf("seed lifecycle: %w", err)
	}

	return lifeID, nil
}

func nextDayBoundary(nowUnix int64) int64 {
	const day = int64(24 * 3600)
	return ((nowUnix / day) + 1) * day
}

// cryptoSeededRNG 用 crypto/rand 给 math/rand/v2 播种。
func cryptoSeededRNG() *mrand.Rand {
	var seed [16]byte
	if _, err := crand.Read(seed[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	s1 := binary.LittleEndian.Uint64(seed[0:8])
	s2 := binary.LittleEndian.Uint64(seed[8:16])
	src := mrand.NewPCG(s1, s2)
	return mrand.New(src)
}
