// Package drives IntrinsicDrive 派生（docs/03 §2.5）。
//
// 从 Genome / LifeState / MentalState 推出本轮内驱力。纯函数；无状态。
package drives

import (
	"fmt"

	"mindverse/internal/core"
	"mindverse/internal/shared"
)

// Derive 派生本轮内驱力（v1 公式，待 0.5 标定）。
func Derive(g core.Genome, ls core.LifeState, ms core.MentalState) []core.Drive {
	now := shared.SystemClock.UnixSec()
	var ds []core.Drive

	if ls.SocialNeed > 0.5 || g.Sociability > 0.7 {
		strength := 0.3 + 0.4*ls.SocialNeed + 0.3*g.Sociability
		ds = append(ds, core.Drive{
			Kind:     core.DriveSocial,
			Strength: clamp01(strength),
			Reason:   fmt.Sprintf("social_need=%.2f sociability=%.2f", ls.SocialNeed, g.Sociability),
			BornAt:   now,
		})
	}
	if g.Curiosity > 0.5 && ls.Competence < 0.6 {
		strength := 0.3 + 0.5*g.Curiosity + 0.2*(1-ls.Competence)
		ds = append(ds, core.Drive{
			Kind:     core.DriveKnowledge,
			Strength: clamp01(strength),
			Reason:   fmt.Sprintf("curiosity=%.2f competence_gap=%.2f", g.Curiosity, 1-ls.Competence),
			BornAt:   now,
		})
	}
	if g.Creativity > 0.6 && ms.Satisfaction < 0.7 {
		strength := 0.3 + 0.5*g.Creativity + 0.2*(1-ms.Satisfaction)
		ds = append(ds, core.Drive{
			Kind:     core.DriveCreativity,
			Strength: clamp01(strength),
			Reason:   fmt.Sprintf("creativity=%.2f satisfaction_gap=%.2f", g.Creativity, 1-ms.Satisfaction),
			BornAt:   now,
		})
	}
	if ls.Stress > 0.5 {
		strength := 0.3 + 0.6*ls.Stress
		ds = append(ds, core.Drive{
			Kind:     core.DriveStability,
			Strength: clamp01(strength),
			Reason:   fmt.Sprintf("stress=%.2f", ls.Stress),
			BornAt:   now,
		})
	}
	if g.Persistence > 0.6 && ls.Confidence < 0.4 {
		strength := 0.3 + 0.4*g.Persistence + 0.3*(1-ls.Confidence)
		ds = append(ds, core.Drive{
			Kind:     core.DriveAchievement,
			Strength: clamp01(strength),
			Reason:   fmt.Sprintf("persistence=%.2f low_confidence=%.2f", g.Persistence, ls.Confidence),
			BornAt:   now,
		})
	}
	return ds
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
