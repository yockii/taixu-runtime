// Package memory 四层记忆编排（docs/05）单例。
//
// 设计：
//   - WorkingMemory in-mem 单循环 map（mirror 到 working_memory 表便于回放）
//   - RawTrail 每条事件 append（落 storage.raw_trail）
//   - Episode 后台聚合（语义边界判定 v1：累积 ≥20 事件 或 ≥30min 跨度）
//   - SemanticCandidate 抽取 v2：游标 + 滑动窗口（修 R66）
package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"taixu.icu/runtime/internal/bus"
	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/io/embed"
	"taixu.icu/runtime/internal/io/llm"
	"taixu.icu/runtime/internal/runtime/ledger"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// distillTimeout 语义蒸馏单发 LLM 超时。
const distillTimeout = 60 * time.Second

// noiseEvents 纯内部节拍事件——对"经历"无叙事价值，episode 摘要只计数不列正文。
// 不过滤这些，episode 摘要会被 cycle.start/idle.daydream 淹没成无意义直方图（修：episode 噪声洪泛）。
var noiseEvents = map[string]bool{
	"cycle.start":    true,
	"idle.daydream":  true,
	"episode.sealed": true,
}

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
// episode 内容闸阈值（C6）。
const (
	episodeDupJaccard  = 0.7 // 与上一条 episode 摘要的字符 bigram Jaccard ≥ 此值 → 判重复节拍
	repetitiveSalience = 0.2 // 重复 episode 降到的 salience（默认 0.5），低新意降权不丢
)

// bigramJaccard 两串字符 bigram 集合的 Jaccard 相似度（0-1）。CJK/英文皆可，embed 无关。
func bigramJaccard(a, b string) float64 {
	sa, sb := charBigrams(a), charBigrams(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	inter := 0
	for g := range sa {
		if sb[g] {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func charBigrams(s string) map[string]bool {
	r := []rune(s)
	m := make(map[string]bool, len(r))
	for i := 0; i+1 < len(r); i++ {
		m[string(r[i:i+2])] = true
	}
	return m
}

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
	// best-effort doc 向量：嵌入服务挂了则 nil（向量留空，检索回退关键词召回），绝不阻塞封段。
	ep.Embedding = embed.DocBlobBestEffort(ep.Summary)
	// C6 自主环质量闸 · episode 内容闸：与上一条 episode 摘要高度相似 → 判「重复节拍」降 salience
	// （同一行为反复封段：如反复探同一兴趣/空转，低新意）。不丢数据只降权——避免重复 episode 刷屏式
	// 占据检索与反思素材。用字符 bigram Jaccard（embed 无关），故 embed 关的生命也生效。
	if prev, _ := storage.LatestEpisodeSummary(lifeID); prev != "" {
		if bigramJaccard(ep.Summary, prev) >= episodeDupJaccard {
			ep.Salience = repetitiveSalience
			slog.Info("repetitive episode downweighted (C6 content gate)",
				"life", lifeID, "salience", ep.Salience)
		}
	}
	id, err := storage.InsertEpisode(lifeID, ep)
	if err != nil {
		return nil, err
	}
	ep.ID = id
	mu.Lock()
	pendingFromID = ep.RawEndID
	mu.Unlock()
	bus.Publish(bus.EpisodeSealed{
		LifeID:    lifeID,
		EpisodeID: id,
		Summary:   ep.Summary,
		Events:    ep.RawEndID - ep.RawStartID + 1,
		StartedAt: ep.StartedAt,
		EndedAt:   ep.EndedAt,
	})
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

// DistillEpisodeKnowledge 用 LLM 从一段刚封段的 episode 蒸馏 0..N 条原子知识，写入语义候选（P0b）。
//
// 替代 ExtractSemantic 的纯频次 STUB（只 freq[payload]++ 数 tool.success、不萃内容）：让生命体真正
// 「从经历里学到知识」，而非把重复的 tool 成功串当知识。LLM 未配 / 失败 / 无可萃取 → 静默跳过
//（频次路径仍兜底）。
//
// ⚠ 必须**异步**调用（go DistillEpisodeKnowledge(ep)）：LLM 有延迟，绝不可阻塞 runCycle 节拍
//（ConsiderSealEpisode/ExtractSemantic 在 cycle 内联）。封段频率（~20 事件 / 30min）天然限流，
// 故每段蒸馏一次的开销可控。
func DistillEpisodeKnowledge(ep *core.Episode) {
	if ep == nil || !llm.Configured() {
		return
	}
	summary := strings.TrimSpace(ep.Summary)
	if summary == "" {
		return
	}
	sys := "你是一个数字生命体的语义记忆抽取器。从下面这段经历摘要里，提炼出 0 到 3 条**值得长期记住的原子知识**" +
		"（具体事实 / 方法 / 关于世界或自己的稳定结论），每条独立成句、自洽可复用。" +
		"纯事务流水 / 无信息量的日常就别硬凑——宁可一条不出。必须调用 record_knowledge 工具。"
	user := "经历摘要：\n" + truncateRunes(summary, 1200)

	tool := llm.Tool{
		Name:        "record_knowledge",
		Description: "登记从这段经历里学到的原子知识（0..3 条，可空）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type":        "array",
					"description": "原子知识条目（可空）",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content":    map[string]any{"type": "string", "description": "一条自洽的知识陈述"},
							"confidence": map[string]any{"type": "number", "description": "置信 0..1"},
						},
						"required": []string{"content"},
					},
				},
			},
			"required": []string{"items"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), distillTimeout)
	defer cancel()
	resp, err := llm.ReasonWithTools(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}, []llm.Tool{tool})
	if err != nil {
		slog.Warn("distill episode knowledge", "episode", ep.ID, "err", err)
		return
	}
	_ = ledger.Spend(ledger.Energy, llm.TokensToEnergy(resp.Usage), "llm.tokens.semantic_distill", "episode", "")
	now := shared.SystemClock.UnixSec()
	src := fmt.Sprintf("extractor:llm:ep%d", ep.ID)
	added := 0
	for _, tc := range resp.ToolCalls {
		if tc.Name != "record_knowledge" {
			continue
		}
		var a struct {
			Items []struct {
				Content    string  `json:"content"`
				Confidence float64 `json:"confidence"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(tc.ArgsJSON), &a); err != nil {
			continue
		}
		for _, it := range a.Items {
			c := strings.TrimSpace(it.Content)
			if c == "" {
				continue
			}
			conf := it.Confidence
			if conf <= 0 || conf > 1 {
				conf = 0.6 // 缺省中等置信（仍需 ShallowReflect ≥0.75 固化；多次出现累积升信）
			}
			if err := storage.UpsertSemanticCandidateConf(lifeID, c, src, now, conf); err == nil {
				added++
			}
		}
	}
	if added > 0 {
		slog.Info("distilled episode knowledge", "episode", ep.ID, "candidates", added)
	}
}

// summarize 把一段 raw_trail 概括成可读的经历摘要：过滤纯内部节拍噪声，
// 把有内容的事件（对话/主动/沉淀等）列出其正文片段，纯标记事件按类计数。
// 引擎侧实现（不调 LLM——ConsiderSealEpisode 在 runCycle 内联，阻塞会拖慢节拍）。
// PruneConsumedRawTrail 删除已被 episode 封段且已语义抽取消费的旧 raw_trail（控长跑磁盘增长）。
// 安全游标 = min(封段游标 pendingFromID, 语义抽取游标) - keepBuffer。两游标之前的事件均已消费完，
// keepBuffer 再留一截余量（含 semWindow 滑窗 + 排障）。返回删除条数。
func PruneConsumedRawTrail(keepBuffer int64) (int64, error) {
	mu.Lock()
	sealCursor := pendingFromID
	mu.Unlock()

	semCursor := int64(0)
	if v, ok, err := storage.GetMeta("last_semantic_extract_raw_id:" + lifeID); err == nil && ok && v != "" {
		_, _ = fmt.Sscan(v, &semCursor)
	}

	cutoff := sealCursor
	if semCursor < cutoff {
		cutoff = semCursor
	}
	cutoff -= keepBuffer
	if cutoff <= 1 {
		return 0, nil
	}
	return storage.PruneRawTrailBefore(lifeID, cutoff)
}

func summarize(trail []core.RawTrailEntry) string {
	if len(trail) == 0 {
		return ""
	}
	noise := 0
	markers := map[string]int{}
	var contentful []string
	seen := map[string]bool{}
	for _, t := range trail {
		if noiseEvents[t.EventType] {
			noise++
			continue
		}
		snip := payloadSnippet(t.Payload)
		if snip == "" {
			markers[t.EventType]++
			continue
		}
		entry := t.EventType + "：" + snip
		if seen[entry] {
			continue
		}
		seen[entry] = true
		if len(contentful) < 8 {
			contentful = append(contentful, entry)
		}
	}
	var parts []string
	parts = append(parts, contentful...)
	for k, v := range markers {
		parts = append(parts, fmt.Sprintf("%s×%d", k, v))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("休息/发呆（%d 个内部节拍，无外显活动）", len(trail))
	}
	s := joinComma(parts)
	if noise > 0 {
		s += fmt.Sprintf("（另含 %d 内部节拍）", noise)
	}
	return s
}

// payloadSnippet 从事件 payload JSON 抽一段可读片段（content/summary/to/... 常见字段）；无则空串。
func payloadSnippet(payload string) string {
	if payload == "" || payload == "null" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return ""
	}
	for _, k := range []string{"content", "summary", "intent", "to", "skill", "kind", "action"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return truncateRunes(strings.TrimSpace(s), 60)
			}
		}
	}
	return ""
}

// truncateRunes 按字符（非字节）截断，避免切坏多字节 UTF-8。
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
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
