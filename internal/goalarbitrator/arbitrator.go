// Package goalarbitrator 三源候选 + Values 仲裁（docs/03 §2.6）。
//
// Phase 0.2 启用两源：IntrinsicDrive + ExternalRequest。
// ReflectionGoal Phase 2 启用。
//
// 仲裁打分（v1）：
//
//   score = base + values_alignment + source_weight
//
//   base = 0.5
//   values_alignment = Σ matched_value.weight × match_strength
//   source_weight:
//     ExternalRequest: +0.20（用户优先）
//     IntrinsicDrive:  +0.00
//     ReflectionGoal:  +0.10
package goalarbitrator

import (
	"fmt"

	"mindverse/internal/core"
	"mindverse/internal/memoryengine"
	"mindverse/internal/perception"
	"mindverse/internal/shared"
)

// Candidate 仲裁候选。
type Candidate struct {
	Source  core.GoalSource
	Intent  string
	Payload string
	// MatchedValues 命中的价值观名（用于 alignment 加权）。
	MatchedValues []string
}

// Arbitrator 仲裁器。
type Arbitrator struct {
	store  *memoryengine.Store
	lifeID string
}

// New 构造。
func New(store *memoryengine.Store, lifeID string) *Arbitrator {
	return &Arbitrator{store: store, lifeID: lifeID}
}

// CollectCandidates 收集本轮所有候选。
//
//   frame  : Perception 输出
//   drives : 当前 IntrinsicDrive 列表（来自需求系统）
func (a *Arbitrator) CollectCandidates(frame perception.PerceptionFrame, drives []core.Drive) []Candidate {
	var out []Candidate
	for _, r := range frame.Externals {
		out = append(out, Candidate{
			Source:        core.GoalExternal,
			Intent:        "respond_to_user",
			Payload:       r.Content,
			MatchedValues: []string{core.ValueFriendship, core.ValueHonesty},
		})
	}
	for _, d := range drives {
		c := Candidate{
			Source:  core.GoalIntrinsic,
			Intent:  string(d.Kind),
			Payload: d.Reason,
		}
		switch d.Kind {
		case core.DriveKnowledge:
			c.MatchedValues = []string{core.ValueGrowth, core.ValueExploration}
		case core.DriveSocial:
			c.MatchedValues = []string{core.ValueFriendship}
		case core.DriveAchievement:
			c.MatchedValues = []string{core.ValueGrowth}
		case core.DriveCreativity:
			c.MatchedValues = []string{core.ValueCreativity}
		case core.DriveStability:
			c.MatchedValues = []string{core.ValueSafety}
		}
		out = append(out, c)
	}
	return out
}

// Arbitrate 对候选打分 + 入队最高分若干。
// 返回入队的 Goal ID 列表。
//
// Phase 0.2 入队策略：取分数 >= 0.6 的全部入队；上限 maxEnqueue。
func (a *Arbitrator) Arbitrate(cands []Candidate, values *core.Values, maxEnqueue int) ([]int64, error) {
	type scored struct {
		c     Candidate
		score float64
	}
	var ranked []scored
	for _, c := range cands {
		s := score(c, values)
		ranked = append(ranked, scored{c: c, score: s})
	}
	// 简易冒泡排（候选不会多）。
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].score > ranked[i].score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	var ids []int64
	now := shared.SystemClock.UnixSec()
	for i, s := range ranked {
		if i >= maxEnqueue || s.score < 0.6 {
			break
		}
		g := &core.Goal{
			Source:          s.c.Source,
			Intent:          s.c.Intent,
			Payload:         s.c.Payload,
			Priority:        s.score,
			Status:          core.GoalPending,
			CreatedAt:       now,
			ArbitrationNote: fmt.Sprintf("score=%.3f matched=%v", s.score, s.c.MatchedValues),
		}
		id, err := a.store.EnqueueGoal(a.lifeID, g)
		if err != nil {
			return ids, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func score(c Candidate, values *core.Values) float64 {
	base := 0.5
	alignment := 0.0
	if values != nil {
		for _, name := range c.MatchedValues {
			if w, ok := values.Weights[name]; ok {
				alignment += w * 0.4
			}
		}
	}
	srcWeight := 0.0
	switch c.Source {
	case core.GoalExternal:
		srcWeight = 0.20
	case core.GoalReflection:
		srcWeight = 0.10
	}
	return base + alignment + srcWeight
}
