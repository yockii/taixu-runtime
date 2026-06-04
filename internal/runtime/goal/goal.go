// Package goal 三源候选 + Values 仲裁（docs/03 §2.6）单例。
//
// Phase 0.2 启用两源：IntrinsicDrive + ExternalRequest。
package goal

import (
	"errors"
	"fmt"
	"sync"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/runtime/perception"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
)

var (
	mu     sync.Mutex
	lifeID string
)

func Init(id string) error {
	if id == "" {
		return errors.New("goal: empty life id")
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	return nil
}

// Candidate 仲裁候选。
type Candidate struct {
	Source        core.GoalSource
	Intent        string
	Payload       string
	MatchedValues []string
}

// CollectCandidates 收集本轮所有候选。
//
// Phase 0.5：对话已移至 reflex 通道；此处不再为 ExternalRequest 派生 respond_to_user。
// 慎思层仅响应 IntrinsicDrive（DriveDerive 输出）。Reflection Phase 2+ 加入。
func CollectCandidates(frame perception.Frame, drives []core.Drive) []Candidate {
	var out []Candidate
	_ = frame // externals 仅留作未来"用户在场"语义；不入候选池
	for _, d := range drives {
		c := Candidate{Source: core.GoalIntrinsic, Intent: string(d.Kind), Payload: d.Reason}
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

// Arbitrate 打分并入队（score ≥ 0.6；上限 maxEnqueue）。返回入队的 Goal ID 列表。
func Arbitrate(cands []Candidate, values *core.Values, maxEnqueue int) ([]int64, error) {
	type scored struct {
		c     Candidate
		score float64
	}
	var ranked []scored
	for _, c := range cands {
		ranked = append(ranked, scored{c: c, score: score(c, values)})
	}
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
		id, err := storage.EnqueueGoal(lifeID, g)
		if err != nil {
			return ids, err
		}
		ids = append(ids, id)
		bus.Publish(bus.GoalEnqueued{
			LifeID:   lifeID,
			GoalID:   id,
			Source:   string(g.Source),
			Intent:   g.Intent,
			Priority: g.Priority,
			Payload:  g.Payload,
		})
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
