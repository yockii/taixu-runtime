// Package genesis 出生流程。仅首次启动调一次。
//
// 独占 genome 表 + seed life_state / mental_state / values + Embryonic 状态。
package genesis

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand/v2"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// Bear 创建新生命体。返回 LifeID。
func Bear() (string, error) {
	if has, err := storage.HasGenome(); err != nil {
		return "", fmt.Errorf("check existing genome: %w", err)
	} else if has {
		return "", fmt.Errorf("genome already exists; cannot re-bear")
	}

	rng := cryptoSeededRNG()
	now := shared.SystemClock.UnixSec()
	lifeID := shared.NewLifeID()

	cur, soc, cre, per, ris, emp := drawGenome(rng)
	g := &core.Genome{
		LifeID:        lifeID,
		Curiosity:     cur,
		Sociability:   soc,
		Creativity:    cre,
		Persistence:   per,
		RiskTaking:    ris,
		Empathy:       emp,
		BornAt:        now,
		GenomeVersion: "v2",
	}
	if err := storage.InsertGenome(g); err != nil {
		return "", fmt.Errorf("insert genome: %w", err)
	}

	ls := &core.LifeState{
		LifeID:          lifeID,
		Energy:          1.0,
		Competence:      0.1,
		SocialNeed:      0.2 + 0.3*g.Sociability, // 起点别太接近触发阈值（R89）
		Stress:          0.0,
		Confidence:      0.5,
		Stability:       0.7,
		EnergyDailyCap:  1.0,
		EnergyUsedToday: 0.0,
		CapResetAt:      nextDayBoundary(now),
		UpdatedAt:       now,
	}
	if err := storage.UpsertLifeState(ls); err != nil {
		return "", fmt.Errorf("seed life_state: %w", err)
	}

	ms := &core.MentalState{
		LifeID:       lifeID,
		Motivation:   0.4 + 0.4*g.Curiosity,
		Satisfaction: 0.5,
		Anxiety:      0.1,
		UpdatedAt:    now,
	}
	if err := storage.UpsertMentalState(ms); err != nil {
		return "", fmt.Errorf("seed mental_state: %w", err)
	}

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
		if err := storage.UpsertValue(lifeID, name, w, now); err != nil {
			return "", fmt.Errorf("seed value %s: %w", name, err)
		}
	}

	if err := storage.UpsertLifecycleState(lifeID, "", core.StateEmbryonic, now, "genesis"); err != nil {
		return "", fmt.Errorf("seed lifecycle: %w", err)
	}

	return lifeID, nil
}

// 基因预算带（R85）：6 维独立均匀 [0,1] 会蹦出"全低废柴/全高超人"，
// 且均匀分布极端值过多。改：集中分布（压极端）+ 软预算带（约束总和，保性格形状）+
// 小概率越界（留极个别天才/弱鸡）。中心 sum≈3.0（每维均值 ~0.5，性格差异最大化）。
const (
	genomeBudgetMin    = 2.7  // 6 维总和下限（防全低）
	genomeBudgetMax    = 3.3  // 6 维总和上限（防全高）
	genomeTraitFloor   = 0.05 // 单维下限（无绝对零维）
	genomeTraitCeil    = 0.95 // 单维上限（无绝对满维）
	genomeBreakoutProb = 0.05 // 越界放行概率（极个别特例生命体）
)

// drawGenome 生成 6 维先天倾向（R85 预算带）。
//
// 步骤：
//  1. 每维三角分布（两均匀取平均）——集中 0.5、少极端，但仍保留宽幅差异。
//  2. 算总和：落 [2.7,3.3] 带内直接用（保留自然差异）。
//  3. 越界则 5% 概率原样放行（天才/弱鸡特例），否则 renormalize 到最近带边
//     ——按比例缩放保留性格**形状**（高好奇低社交的轮廓不变），只调总预算。
func drawGenome(rng *mrand.Rand) (cur, soc, cre, per, ris, emp float64) {
	tri := func() float64 {
		v := (rng.Float64() + rng.Float64()) / 2 // 两均匀取平均 → 三角分布
		if v < genomeTraitFloor {
			v = genomeTraitFloor
		}
		if v > genomeTraitCeil {
			v = genomeTraitCeil
		}
		return v
	}
	t := [6]float64{tri(), tri(), tri(), tri(), tri(), tri()}
	sum := 0.0
	for _, x := range t {
		sum += x
	}
	inBand := sum >= genomeBudgetMin && sum <= genomeBudgetMax
	if !inBand && rng.Float64() >= genomeBreakoutProb {
		target := genomeBudgetMin
		if sum > genomeBudgetMax {
			target = genomeBudgetMax
		}
		// 迭代缩放：每次按比例拉向 target，再逐维 clamp；clamp 会把 sum 顶偏，
		// 故多迭代几次让未触顶/底的维吸收残差，收敛进带（最多 6 次足够）。
		for iter := 0; iter < 6; iter++ {
			s := 0.0
			for _, x := range t {
				s += x
			}
			if s >= genomeBudgetMin && s <= genomeBudgetMax {
				break
			}
			f := target / s
			for i := range t {
				t[i] *= f
				if t[i] < genomeTraitFloor {
					t[i] = genomeTraitFloor
				}
				if t[i] > genomeTraitCeil {
					t[i] = genomeTraitCeil
				}
			}
		}
	}
	return t[0], t[1], t[2], t[3], t[4], t[5]
}

func nextDayBoundary(nowUnix int64) int64 {
	const day = int64(24 * 3600)
	return ((nowUnix / day) + 1) * day
}

func cryptoSeededRNG() *mrand.Rand {
	var seed [16]byte
	if _, err := crand.Read(seed[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	s1 := binary.LittleEndian.Uint64(seed[0:8])
	s2 := binary.LittleEndian.Uint64(seed[8:16])
	return mrand.New(mrand.NewPCG(s1, s2))
}
