package memoryengine

import (
	"encoding/json"
	"fmt"
	"sync"

	"mindverse/internal/core"
	"mindverse/internal/shared"
)

// Engine 四层记忆编排器。
//
// 设计：
//   - WorkingMemory 是 in-memory 单循环 map（每 tick 清）；同时 mirror 到 working_memory 表便于回放
//   - RawTrail 每条事件 append 即落库
//   - Episode 由 ConsiderSeal 决定是否封一段（语义边界判定 v1：长静默、cycle 跨度过大）
//   - SemanticCandidate 由 RawTrail 抽取（v1 极简：发现重复 ToolRunner 成功 / 重复关键词）
type Engine struct {
	store  *Store
	lifeID string

	mu      sync.Mutex
	working map[string]string
	// 待封段游标：raw_trail.id 大于此值的尚未封入 Episode
	pendingFromID int64
}

// NewEngine 构造。从最近一条 Episode 的 raw_end_id 续接游标。
func NewEngine(store *Store, lifeID string) (*Engine, error) {
	cursor, err := store.LatestEpisodeRawEndID(lifeID)
	if err != nil {
		return nil, err
	}
	return &Engine{
		store:         store,
		lifeID:        lifeID,
		working:       make(map[string]string),
		pendingFromID: cursor,
	}, nil
}

// PutWorking 写工作记忆槽（in-mem 主，mirror 落库）。
func (e *Engine) PutWorking(cycleID int64, slot, content string) {
	e.mu.Lock()
	e.working[slot] = content
	e.mu.Unlock()
	if _, err := e.store.AppendWorking(e.lifeID, cycleID, slot, content, shared.SystemClock.UnixSec()); err != nil {
		// 静默：mirror 失败不影响主流程
		_ = err
	}
}

// Working 读取工作记忆某槽。
func (e *Engine) Working(slot string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.working[slot]
	return v, ok
}

// ResetWorking 每 tick 末清空 in-mem（落库的 working_memory 仍可回放）。
func (e *Engine) ResetWorking() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.working = make(map[string]string)
}

// AppendEvent 写入一条 RawTrail 事件。任何模块可调。
// payload 任意可 JSON 化对象。
func (e *Engine) AppendEvent(cycleID int64, eventType string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return e.store.AppendRawTrail(e.lifeID, cycleID, eventType, string(b), shared.SystemClock.UnixSec())
}

// ConsiderSealEpisode 检查是否应当封一段 Episode（语义边界判定 v1）。
// 规则：
//   - 自上次封段后 raw_trail 累积 ≥ 20 条 ⇒ 封段
//   - 或最早未封事件距今 ≥ 30 分钟 ⇒ 封段
//
// Phase 0.5 标定后引入：话题转移 / 显著情绪转折 / Goal 完成。
func (e *Engine) ConsiderSealEpisode() (*core.Episode, error) {
	trail, err := e.store.RawTrailSinceID(e.lifeID, e.pendingFromID)
	if err != nil {
		return nil, err
	}
	if len(trail) == 0 {
		return nil, nil
	}
	const minBatch = 20
	const maxAgeSec = int64(30 * 60)
	now := shared.SystemClock.UnixSec()
	if len(trail) < minBatch && now-trail[0].CreatedAt < maxAgeSec {
		return nil, nil
	}
	ep := &core.Episode{
		Summary:    summarize(trail),
		StartedAt:  trail[0].CreatedAt,
		EndedAt:    trail[len(trail)-1].CreatedAt,
		RawStartID: trail[0].ID,
		RawEndID:   trail[len(trail)-1].ID,
		Salience:   0.5,
		CreatedAt:  now,
		SealedAt:   now,
	}
	id, err := e.store.InsertEpisode(e.lifeID, ep)
	if err != nil {
		return nil, err
	}
	ep.ID = id
	e.mu.Lock()
	e.pendingFromID = ep.RawEndID
	e.mu.Unlock()
	return ep, nil
}

// ExtractSemantic 简易 v1 抽取：从 trail 找重复 event_type=tool.success 的内容作为候选。
// Phase 0.3 LLM 接通后改为 Summarize 模型抽取。
func (e *Engine) ExtractSemantic() (int, error) {
	trail, err := e.store.ListRecentRawTrail(e.lifeID, 50)
	if err != nil {
		return 0, err
	}
	freq := map[string]int{}
	for _, t := range trail {
		if t.EventType == "tool.success" {
			freq[t.Payload]++
		}
	}
	now := shared.SystemClock.UnixSec()
	added := 0
	for content, n := range freq {
		if n >= 2 {
			if err := e.store.UpsertSemanticCandidate(e.lifeID, content, "extractor:v1", now); err == nil {
				added++
			}
		}
	}
	return added, nil
}

func summarize(trail []core.RawTrailEntry) string {
	if len(trail) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, t := range trail {
		counts[t.EventType]++
	}
	var parts []string
	for k, v := range counts {
		parts = append(parts, fmt.Sprintf("%s×%d", k, v))
	}
	return fmt.Sprintf("auto-segment %d events: %s", len(trail), joinComma(parts))
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
