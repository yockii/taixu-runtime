// Package drives IntrinsicDrive 派生（docs/03 §2.5）。
//
// 从 Genome / LifeState / MentalState 推出本轮内驱力。纯函数；无状态。
package drives

import (
	"fmt"

	"mindverse/internal/core"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
)

// Derive 派生本轮内驱力。
//
// Phase 0.5 关键设计（R79）：**只有具体兴趣（interest_seed）派生慎思目标**。
//
// 原通用驱动（competence_gap 知识 / social / creativity / stability / achievement）
// 全部移除——它们的 payload 只是 "social_need=.. sociability=.." 之类的 Reason 字符串，
// 无具体主题，到了 deliberative agent loop LLM 无从下手，且每 cycle 重生刷屏队列。
//
// 这些 genome/state 压力改由其它通道体现，不烧 LLM 跑空目标：
//   - 社交压力（social_need 高）→ idle 主动社交（reflex.TryProactiveReach）
//   - 好奇/无聊（boredom）→ idle 自发生成具体兴趣 → 下轮成具体目标
//   - 压力/创造/成就 → 影响 state / mood（Phase 2 反思、Phase 3 自主项目再细化）
//
// 即：自主行动只来自"具体的想做的事"，不来自空泛的情绪标签。
func Derive(g core.Genome, ls core.LifeState, ms core.MentalState, lifeID string) []core.Drive {
	now := shared.SystemClock.UnixSec()
	var ds []core.Drive

	// 兴趣种子派生 DriveKnowledge（最强 3 条；strength≥0.4）。
	// 来源：对话识别（reflex add_interest）/ idle 自发 / 未来反思。
	seeds, _ := storage.ListInterestSeeds(lifeID, 0.4, 3)
	for _, s := range seeds {
		// 掌握度衰减（R77）：掌握越深，再探索的内驱越弱（知识感知，非盲衰减）。
		masteryFactor := 1.0 - s.Mastery
		// 探索次数衰减（防止单一兴趣短时间被反复消费）。
		exploreFactor := 1.0
		if s.ExploredCount > 0 {
			exploreFactor = 1.0 / (1.0 + 0.3*float64(s.ExploredCount))
		}
		strength := (s.Strength*0.7 + 0.3*g.Curiosity) * exploreFactor * masteryFactor
		ds = append(ds, core.Drive{
			Kind:     core.DriveKnowledge,
			Strength: clamp01(strength),
			Reason:   fmt.Sprintf("interest_seed#%d %s (%s)", s.ID, s.Content, s.Kind),
			BornAt:   now,
		})
	}

	_ = ls // 通用 state/genome 压力现经 idle 通道体现，不在此派生空泛目标
	_ = ms
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
