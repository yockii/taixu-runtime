// Package goal 三源候选 + Values 仲裁（docs/03 §2.6）单例。
//
// Phase 0.5+：
//   - 两源派生：IntrinsicDrive + ExternalRequest（对话已移至 reflex，外源此处不再用）
//   - Backlog 控制（R75）：入队前检查 active+pending 数，超过 MaxOpenGoals 不入；
//   - 去重（R74 / R75 兄弟问题）：同 interest_seed#N 已在飞时跳过，避免反复派同任务
package goal

import (
	"errors"
	"fmt"
	"regexp"
	"sync"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/runtime/perception"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
)

// MaxOpenGoals 队列内可同时存在的 active+pending 总数上限。
// Phase 0.5 设为 1：生命体一次只专注一件具体的事，做完再产生下一个
// （配合 R79：只有 interest_seed 具体目标入队，不再有通用空目标堆叠）。
const MaxOpenGoals = 1

// GenericGoalCooldownSec 通用（非兴趣种子）目标完成后的再生冷却（R79）。
// 同一空泛 payload（如 "curiosity=.. competence_gap=.."）在此窗口内完成过则不重派，
// 避免 competence 尚未补足时每 cycle 反复生成同一无主题目标刷屏队列。
const GenericGoalCooldownSec = 3600

// interestSeedRe 用于从 payload 抽 "interest_seed#N" 前缀作 dedup key。
var interestSeedRe = regexp.MustCompile(`interest_seed#\d+`)

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

// Arbitrate 打分并入队。
//
// 入队规则（按生效顺序）：
//  1. 计算 headroom = MaxOpenGoals - 当前 active+pending 数；headroom ≤ 0 → 全跳过
//  2. score < 0.6 → 跳过该候选
//  3. payload 含 interest_seed#N 且该 seed 已有 open 目标 → 跳过（dedup）
//  4. payload 与已 open 目标的 payload 完全相同（intent 相同）→ 跳过
//  5. 否则入队，headroom--
//
// maxEnqueue 是本轮单次入队的额外上限（与 headroom 取 min）。
func Arbitrate(cands []Candidate, values *core.Values, maxEnqueue int) ([]int64, error) {
	open, err := storage.CountActiveOrPendingGoals(lifeID)
	if err != nil {
		return nil, fmt.Errorf("count open goals: %w", err)
	}
	headroom := MaxOpenGoals - open
	if headroom <= 0 {
		return nil, nil
	}
	if maxEnqueue > headroom {
		maxEnqueue = headroom
	}

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
	enqueued := 0
	for _, s := range ranked {
		if enqueued >= maxEnqueue {
			break
		}
		if s.score < 0.6 {
			break
		}
		if skip, err := shouldSkipDup(s.c); err != nil {
			return ids, err
		} else if skip {
			continue
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
		enqueued++
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

// shouldSkipDup 判断该候选是否应因去重跳过。
//
//   - interest_seed#N（具体目标）：仅查 open。再探索由 strength/mastery 衰减门控，
//     不加完成冷却（学完想重温是合理的，且 R77 已自然平息）。
//   - 通用目标（competence_gap 等无主题）：查 open + 完成冷却（R79），
//     防 competence 补足前同一空泛目标每 cycle 重生刷屏。
func shouldSkipDup(c Candidate) (bool, error) {
	if seedKey := interestSeedRe.FindString(c.Payload); seedKey != "" {
		dup, err := storage.HasOpenGoalWithPayloadSubstring(lifeID, seedKey)
		if err != nil {
			return false, fmt.Errorf("dedup seed: %w", err)
		}
		return dup, nil
	}
	since := shared.SystemClock.UnixSec() - GenericGoalCooldownSec
	dup, err := storage.HasRecentGoalWithPayloadSubstring(lifeID, c.Payload, since)
	if err != nil {
		return false, fmt.Errorf("dedup payload: %w", err)
	}
	return dup, nil
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
