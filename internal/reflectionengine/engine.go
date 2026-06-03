// Package reflectionengine 反思引擎（docs/03 §2.4）。
//
// Phase 0.2 仅 ShallowReflect：
//   - 不修改 Values（DeepReflect Phase 2 启用）
//   - 可固化 SemanticCandidate → Confirmed
//   - 触发由生命体自身决定（用户多次纠正：与基因相关）
//
// 触发概率函数（v1，待 0.5 标定）：
//
//   P(reflect) = 0.10 + 0.35 * Curiosity + 0.25 * Persistence - 0.20 * Anxiety
//   clamp [0.02, 0.85]
//
// 即：高好奇 / 高坚持 ⇒ 更愿反思；高焦虑 ⇒ 反思被压制。
package reflectionengine

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand/v2"

	"mindverse/internal/core"
	"mindverse/internal/memoryengine"
	"mindverse/internal/shared"
)

// Engine 反思引擎。
type Engine struct {
	store  *memoryengine.Store
	mem    *memoryengine.Engine
	lifeID string
	rng    *mrand.Rand
}

// New 构造。
func New(store *memoryengine.Store, mem *memoryengine.Engine, lifeID string) *Engine {
	return &Engine{store: store, mem: mem, lifeID: lifeID, rng: seededRNG()}
}

// ShouldReflect 由生命体自身决定本轮是否反思。与基因相关。
func (e *Engine) ShouldReflect(g core.Genome, ls core.LifeState, ms core.MentalState) bool {
	p := 0.10 + 0.35*g.Curiosity + 0.25*g.Persistence - 0.20*ms.Anxiety
	if p < 0.02 {
		p = 0.02
	}
	if p > 0.85 {
		p = 0.85
	}
	return e.rng.Float64() < p
}

// Reflect 执行一次 ShallowReflect：
//   1. 浅审 SemanticCandidate（高置信 → Confirmed）
//   2. 写入一条 ReflectionMemory 记录本轮反思的事
//
// 返回固化候选数 + 反思记录 ID。
func (e *Engine) Reflect(triggeredBy string) (promoted int, reflectionID int64, err error) {
	candidates, err := e.store.ListCandidatesAboveConfidence(e.lifeID, 0.75, 10)
	if err != nil {
		return 0, 0, fmt.Errorf("list candidates: %w", err)
	}
	now := shared.SystemClock.UnixSec()
	for _, c := range candidates {
		if perr := e.store.PromoteToConfirmed(e.lifeID, c.ID, c.Content, c.Confidence, now); perr == nil {
			promoted++
		}
	}

	summary := fmt.Sprintf("shallow reflect: promoted %d/%d candidates", promoted, len(candidates))
	insight := ""
	if promoted > 0 {
		insight = "consolidated repeated experiences into long-term knowledge"
	}

	id, err := e.store.InsertReflection(e.lifeID, &core.ReflectionMemory{
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
