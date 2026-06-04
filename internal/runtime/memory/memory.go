// Package memory 四层记忆编排（docs/05）单例。
//
// 设计：
//   - WorkingMemory in-mem 单循环 map（mirror 到 working_memory 表便于回放）
//   - RawTrail 每条事件 append（落 storage.raw_trail）
//   - Episode 后台聚合（语义边界判定 v1：累积 ≥20 事件 或 ≥30min 跨度）
//   - SemanticCandidate 抽取 v2：游标 + 滑动窗口（修 R66）
package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"mindverse/internal/core"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
)

var (
	mu            sync.Mutex
	lifeID        string
	working       = make(map[string]string)
	pendingFromID int64
	semWindow     []core.RawTrailEntry
)

// Init 加载游标（从最近一条 Episode 的 raw_end_id 续接）。
func Init(id string) error {
	if id == "" {
		return errors.New("memory: empty life id")
	}
	cursor, err := storage.LatestEpisodeRawEndID(id)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	pendingFromID = cursor
	working = make(map[string]string)
	semWindow = nil
	return nil
}

// PutWorking 写工作记忆槽（in-mem 主，mirror 落库）。
func PutWorking(cycleID int64, slot, content string) {
	mu.Lock()
	working[slot] = content
	mu.Unlock()
	_, _ = storage.AppendWorking(lifeID, cycleID, slot, content, shared.SystemClock.UnixSec())
}

// Working 读取某槽。
func Working(slot string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	v, ok := working[slot]
	return v, ok
}

// ResetWorking 每 tick 末清空 in-mem。
func ResetWorking() {
	mu.Lock()
	working = make(map[string]string)
	mu.Unlock()
}

// AppendEvent 写一条 RawTrail 事件。
func AppendEvent(cycleID int64, eventType string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return storage.AppendRawTrail(lifeID, cycleID, eventType, string(b), shared.SystemClock.UnixSec())
}

// ConsiderSealEpisode 检查是否封段（≥20 事件 或 ≥30min 跨度）。
func ConsiderSealEpisode() (*core.Episode, error) {
	trail, err := storage.RawTrailSinceID(lifeID, pendingFromID)
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
	id, err := storage.InsertEpisode(lifeID, ep)
	if err != nil {
		return nil, err
	}
	ep.ID = id
	mu.Lock()
	pendingFromID = ep.RawEndID
	mu.Unlock()
	return ep, nil
}

// ExtractSemantic 抽取 v2：游标 + 滑动窗口避免重复扫描（修 R66）。
func ExtractSemantic() (int, error) {
	cursorKey := "last_semantic_extract_raw_id:" + lifeID
	cursorStr, _, err := storage.GetMeta(cursorKey)
	if err != nil {
		return 0, err
	}
	var cursor int64
	if cursorStr != "" {
		_, _ = fmt.Sscan(cursorStr, &cursor)
	}

	newEvents, err := storage.RawTrailSinceID(lifeID, cursor)
	if err != nil {
		return 0, err
	}
	if len(newEvents) == 0 {
		return 0, nil
	}

	mu.Lock()
	semWindow = append(semWindow, newEvents...)
	const windowMax = 200
	if len(semWindow) > windowMax {
		semWindow = semWindow[len(semWindow)-windowMax:]
	}
	freq := map[string]int{}
	for _, t := range semWindow {
		if t.EventType == "tool.success" {
			freq[t.Payload]++
		}
	}
	mu.Unlock()

	now := shared.SystemClock.UnixSec()
	added := 0
	for content, n := range freq {
		if n >= 2 {
			if err := storage.UpsertSemanticCandidate(lifeID, content, "extractor:v2", now); err == nil {
				added++
			}
		}
	}

	maxID := newEvents[len(newEvents)-1].ID
	if err := storage.SetMeta(cursorKey, fmt.Sprintf("%d", maxID)); err != nil {
		return added, err
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
