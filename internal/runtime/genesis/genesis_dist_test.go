package genesis

import (
	crand "crypto/rand"
	"encoding/binary"
	mrand "math/rand/v2"
	"testing"
)

func TestGenomeDistribution(t *testing.T) {
	var seed [16]byte
	_, _ = crand.Read(seed[:])
	rng := mrand.New(mrand.NewPCG(binary.LittleEndian.Uint64(seed[0:8]), binary.LittleEndian.Uint64(seed[8:16])))

	const N = 50000
	inBand, allLow, allHigh := 0, 0, 0
	minSum, maxSum := 99.0, 0.0
	var traitMin, traitMax [6]float64
	for i := range traitMin {
		traitMin[i] = 1.0
	}
	for n := 0; n < N; n++ {
		v := [6]float64{}
		v[0], v[1], v[2], v[3], v[4], v[5] = drawGenome(rng)
		sum := 0.0
		lo, hi := true, true
		for i, x := range v {
			sum += x
			if x >= 0.4 {
				lo = false
			}
			if x < 0.6 {
				hi = false
			}
			if x < traitMin[i] {
				traitMin[i] = x
			}
			if x > traitMax[i] {
				traitMax[i] = x
			}
		}
		if sum >= genomeBudgetMin && sum <= genomeBudgetMax {
			inBand++
		}
		if lo {
			allLow++
		}
		if hi {
			allHigh++
		}
		if sum < minSum {
			minSum = sum
		}
		if sum > maxSum {
			maxSum = sum
		}
	}
	t.Logf("N=%d inBand=%.1f%% allLow(<0.4)=%d allHigh(>=0.6)=%d sumRange=[%.2f,%.2f]",
		N, 100*float64(inBand)/N, allLow, allHigh, minSum, maxSum)
	t.Logf("per-trait observed range: c=[%.2f,%.2f] s=[%.2f,%.2f] cr=[%.2f,%.2f] p=[%.2f,%.2f] r=[%.2f,%.2f] e=[%.2f,%.2f]",
		traitMin[0], traitMax[0], traitMin[1], traitMax[1], traitMin[2], traitMax[2],
		traitMin[3], traitMax[3], traitMin[4], traitMax[4], traitMin[5], traitMax[5])

	// 大部分在预算带内（R85 中心 sum≈3.0）：留约 5% 越界特例，故 >=90% 即合格。
	if pct := float64(inBand) / N; pct < 0.90 {
		t.Errorf("inBand %.1f%% < 90%%（预算带收敛失效）", 100*pct)
	}
	// 全低 / 全高生命体应几近绝迹（核心诉求：无 6 维全低废柴）。
	if allLow+allHigh > N/500 {
		t.Errorf("allLow+allHigh=%d 过多（>%d）", allLow+allHigh, N/500)
	}
	// 单维仍需跨大幅度（性格差异性）：每维实测幅度应接近 [floor, ceil]。
	for i := range traitMin {
		if traitMin[i] > 0.15 || traitMax[i] < 0.85 {
			t.Errorf("维度 %d 幅度过窄 [%.2f,%.2f]（性格差异被压没）", i, traitMin[i], traitMax[i])
		}
	}
}
