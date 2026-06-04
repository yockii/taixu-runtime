// Package reflect ShallowReflect 反思（docs/03 §2.4）单例。
//
// Phase 0.2 仅 Shallow：
//   - 不修改 Values（DeepReflect Phase 2）
//   - 可固化 SemanticCandidate ≥0.75 → Confirmed
//   - 触发由生命体自身决定（与基因相关）
//
// 触发概率（v1）：
//   P(reflect) = 0.10 + 0.35*Curiosity + 0.25*Persistence - 0.20*Anxiety, clamp [0.02, 0.85]
package reflect

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	mrand "math/rand/v2"
	"sync"

	"mindverse/internal/core"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
)

var (
	mu     sync.Mutex
	lifeID string
	rng    *mrand.Rand
)

// Init 绑定生命体 + 初始化随机源。
func Init(id string) error {
	if id == "" {
		return errors.New("reflect: empty life id")
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	rng = seededRNG()
	return nil
}

// ShouldReflect 由生命体自身决定本轮是否反思。
func ShouldReflect(g core.Genome, ls core.LifeState, ms core.MentalState) bool {
	p := 0.10 + 0.35*g.Curiosity + 0.25*g.Persistence - 0.20*ms.Anxiety
	if p < 0.02 {
		p = 0.02
	}
	if p > 0.85 {
		p = 0.85
	}
	mu.Lock()
	defer mu.Unlock()
	return rng.Float64() < p
}

// Run 执行一次 ShallowReflect：固化高置信候选 + 写反思记录。
func Run(triggeredBy string) (promoted int, reflectionID int64, err error) {
	candidates, err := storage.ListCandidatesAboveConfidence(lifeID, 0.75, 10)
	if err != nil {
		return 0, 0, fmt.Errorf("list candidates: %w", err)
	}
	now := shared.SystemClock.UnixSec()
	for _, c := range candidates {
		if perr := storage.PromoteToConfirmed(lifeID, c.ID, c.Content, c.Confidence, now); perr != nil {
			slog.Warn("reflect: promote failed", "candidate_id", c.ID, "err", perr)
		} else {
			promoted++
		}
	}

	summary := fmt.Sprintf("shallow reflect: promoted %d/%d candidates", promoted, len(candidates))
	insight := ""
	if promoted > 0 {
		insight = "consolidated repeated experiences into long-term knowledge"
	}

	id, err := storage.InsertReflection(lifeID, &core.ReflectionMemory{
		Kind:        core.ReflectShallow,
		Summary:     summary,
		Insight:     insight,
		TriggeredBy: triggeredBy,
		CreatedAt:   now,
	})
	if err != nil {
		return promoted, 0, fmt.Errorf("insert reflection: %w", err)
	}
	return promoted, id, nil
}

func seededRNG() *mrand.Rand {
	var seed [16]byte
	if _, err := crand.Read(seed[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	s1 := binary.LittleEndian.Uint64(seed[0:8])
	s2 := binary.LittleEndian.Uint64(seed[8:16])
	return mrand.New(mrand.NewPCG(s1, s2))
}
