// Package reflect ShallowReflect 反思（docs/03 §2.4）单例。
//
// Phase 0.2 仅 Shallow：
//   - 不修改 Values（DeepReflect Phase 2）
//   - 可固化 SemanticCandidate ≥0.75 → Confirmed
//   - 触发由生命体自身决定（与基因相关）
//
// 触发概率（v1）：
//
//	P(reflect) = 0.10 + 0.35*Curiosity + 0.25*Persistence - 0.20*Anxiety, clamp [0.02, 0.85]
package reflect

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	mrand "math/rand/v2"
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

// DeepReflectMaxWeightDelta 单次 DeepReflect 对任一价值观权重的最大调整幅度。
// 防一次反思剧烈扭曲人格——价值观应缓慢漂移、忠于真实经历，而非跳变。
const DeepReflectMaxWeightDelta = 0.05

// deepReflectEpisodes DeepReflect 喂给 LLM 的近期 episode 数。
const deepReflectEpisodes = 12

// 反思生兴趣护栏（P1）：活跃种子数上限、单次至多新增、新种子初始强度。
const (
	DeepReflectSeedCeil     = 4   // 活跃种子(strength≥0.4)达此数则本次不再播（治话题固着，对齐 idle 的 3）
	DeepReflectSeedMax      = 2   // 单次深反思至多播 2 条
	DeepReflectSeedStrength = 0.5 // 反思生兴趣初始强度（中等：真实主题，但不抢占对话引出的兴趣）
)

// llmTimeout 反思内单发 LLM 调用超时。
const llmTimeout = 60 * time.Second

var (
	mu     sync.Mutex
	lifeID string
	genome core.Genome // 反思洞见 / 深反思以生命体的声音表达（PersonaPrompt）
	rng    *mrand.Rand
)

// Init 绑定生命体 + 初始化随机源。
func Init(id string, g core.Genome) error {
	if id == "" {
		return errors.New("reflect: empty life id")
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	genome = g
	rng = seededRNG()
	return nil
}

// ShouldReflect 由生命体自身决定本轮是否反思。
func ShouldReflect(g core.Genome, ls core.LifeState, ms core.MentalState) bool {
	// 太累无心反思（R86）：低能量优先休息回血，不在此处空耗。
	if ls.Energy < 0.15 {
		return false
	}
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
	var promotedContents []string
	for _, c := range candidates {
		// best-effort doc 向量：固化的语义知识带向量，供 query_memory 语义召回。
		blob := embed.DocBlobBestEffort(c.Content)
		if perr := storage.PromoteToConfirmedWithEmbedding(lifeID, c.ID, c.Content, c.Confidence, now, blob); perr != nil {
			slog.Warn("reflect: promote failed", "candidate_id", c.ID, "err", perr)
		} else {
			promoted++
			promotedContents = append(promotedContents, c.Content)
		}
	}

	summary := fmt.Sprintf("shallow reflect: promoted %d/%d candidates", promoted, len(candidates))
	insight := ""
	if promoted > 0 {
		// 用生命体自己的声音凝练一句真实洞见（替换写死英文串，速胜#1）。
		insight = composeShallowInsight(promotedContents)
	}

	// best-effort doc 向量（嵌入服务挂了则 nil，检索回退关键词召回）。反思文本 = summary + insight。
	embText := summary
	if insight != "" {
		embText = summary + "。" + insight
	}
	id, err := storage.InsertReflection(lifeID, &core.ReflectionMemory{
		Kind:        core.ReflectShallow,
		Summary:     summary,
		Insight:     insight,
		TriggeredBy: triggeredBy,
		Embedding:   embed.DocBlobBestEffort(embText),
		CreatedAt:   now,
	})
	if err != nil {
		return promoted, 0, fmt.Errorf("insert reflection: %w", err)
	}
	bus.Publish(bus.ReflectionCompleted{
		LifeID:       lifeID,
		ReflectionID: id,
		Kind:         string(core.ReflectShallow),
		Promoted:     promoted,
		Summary:      summary,
	})
	return promoted, id, nil
}

// composeShallowInsight 让 LLM 用生命体的声音，把这次固化的知识凝练成一句真实洞见（替换写死串）。
// LLM 未配 / 失败 → 退回朴素串，绝不阻断反思。
func composeShallowInsight(contents []string) string {
	const fallback = "consolidated repeated experiences into long-term knowledge"
	if !llm.Configured() || len(contents) == 0 {
		return fallback
	}
	var ub strings.Builder
	for _, c := range contents {
		ub.WriteString("- " + truncateRunes(c, 120) + "\n")
	}
	sys := genome.PersonaPrompt() + "\n" +
		"你刚把一批反复出现的经历固化成长期知识。用第一人称、一句话（≤40 字）说出你由此得到的真实体会 / 洞见，" +
		"忠于你的性格语气，别写报告体。只输出这一句。"
	ctx, cancel := context.WithTimeout(context.Background(), llmTimeout)
	defer cancel()
	resp, err := llm.Reason(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: "刚固化的知识：\n" + ub.String()},
	})
	if err != nil {
		slog.Warn("compose shallow insight", "err", err)
		return fallback
	}
	_ = ledger.Spend(ledger.Energy, llm.TokensToEnergy(resp.Usage), "llm.tokens.reflect_insight", "reflect", "")
	out := strings.TrimSpace(resp.Text)
	if out == "" {
		return fallback
	}
	return out
}

// RunDeep DeepReflect（白皮书核心模块）：据近期经历 + 当前价值观，让 LLM 提出有界的
// 价值观权重微调 + 一段真实洞见，闭合「反思 → 修正价值观 → 调整目标」回路。
//
// 安全护栏：① 仅调整生命体已有的价值观键（不凭空发明新价值）；② 每项单次至多 ±DeepReflectMaxWeightDelta；
// ③ 逐项 clamp [0,1]。LLM 未配则跳过（DeepReflect 必须有 LLM，无则不强行）。
func RunDeep(triggeredBy string) (adjusted int, reflectionID int64, err error) {
	if !llm.Configured() {
		return 0, 0, nil
	}
	values, err := storage.LoadValues(lifeID)
	if err != nil {
		return 0, 0, fmt.Errorf("load values: %w", err)
	}
	if values == nil || len(values.Weights) == 0 {
		return 0, 0, nil // 还没有价值观可调
	}
	eps, err := storage.ListEpisodes(lifeID, "", deepReflectEpisodes, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("list episodes: %w", err)
	}
	if len(eps) == 0 {
		return 0, 0, nil // 无经历可反思
	}

	deltas, interests, insight := proposeDeepReflection(values, eps)
	now := shared.SystemClock.UnixSec()
	var changed []string
	for name, delta := range deltas {
		old, ok := values.Weights[name]
		if !ok {
			continue // 只调已有价值观，不凭空发明
		}
		// clamp 调整幅度（防 LLM 越界）+ clamp 结果域 [0,1]。
		if delta > DeepReflectMaxWeightDelta {
			delta = DeepReflectMaxWeightDelta
		}
		if delta < -DeepReflectMaxWeightDelta {
			delta = -DeepReflectMaxWeightDelta
		}
		nw := old + delta
		if nw < 0 {
			nw = 0
		}
		if nw > 1 {
			nw = 1
		}
		if nw == old {
			continue
		}
		if err := storage.UpsertValue(lifeID, name, nw, now); err != nil {
			slog.Warn("deep reflect upsert value", "name", name, "err", err)
			continue
		}
		adjusted++
		changed = append(changed, fmt.Sprintf("%s%+.3f", name, nw-old))
	}

	// 反思生兴趣（P1）：闭合 idle.go 注释的「未来反思 → interest」回路 / R79 自主兴趣缺口——
	// 深反思若意识到反复出现的主题 / 想探索的新方向，自主播下兴趣种子，下个 cycle 经 drives.Derive 成真目标。
	// 总量护栏（治话题固着，对齐 idle）：仅当活跃种子 <DeepReflectSeedCeil 时补，单次至多 DeepReflectSeedMax 条。
	seeded := 0
	if len(interests) > 0 {
		active, _ := storage.ListInterestSeeds(lifeID, 0.4, 10)
		room := DeepReflectSeedCeil - len(active)
		for _, it := range interests {
			if room <= 0 || seeded >= DeepReflectSeedMax {
				break
			}
			c := strings.TrimSpace(it.Content)
			if c == "" {
				continue
			}
			kind := it.Kind
			if kind == "" {
				kind = "topic"
			}
			if err := storage.UpsertInterestSeed(lifeID, c, kind, "reflect", "", DeepReflectSeedStrength, now); err != nil {
				slog.Warn("deep reflect seed interest", "err", err)
				continue
			}
			seeded++
			room--
		}
		if seeded > 0 {
			slog.Info("deep reflect seeded interests", "count", seeded)
		}
	}

	summary := fmt.Sprintf("deep reflect: adjusted %d values", adjusted)
	if seeded > 0 {
		summary += fmt.Sprintf(", seeded %d interests", seeded)
	}
	if len(changed) > 0 {
		summary += " (" + joinComma(changed) + ")"
	}
	embText := summary
	if insight != "" {
		embText = summary + "。" + insight
	}
	id, err := storage.InsertReflection(lifeID, &core.ReflectionMemory{
		Kind:        core.ReflectDeep,
		Summary:     summary,
		Insight:     insight,
		TriggeredBy: triggeredBy,
		Embedding:   embed.DocBlobBestEffort(embText),
		CreatedAt:   now,
	})
	if err != nil {
		return adjusted, 0, fmt.Errorf("insert reflection: %w", err)
	}
	bus.Publish(bus.ReflectionCompleted{
		LifeID:       lifeID,
		ReflectionID: id,
		Kind:         string(core.ReflectDeep),
		Promoted:     adjusted,
		Summary:      summary,
	})
	return adjusted, id, nil
}

// proposedInterest 深反思自主提出的一个待探索兴趣。
type proposedInterest struct {
	Content string
	Kind    string
}

// proposeDeepReflection 单发 LLM（结构化 tool call）：据近期经历提出
//   - 价值观权重有界微调（deltas）
//   - 0..N 个想探索的新兴趣（new_interests，反思生兴趣，P1）
//   - 一句第一人称洞见（insight）
//
// 返回 (deltas: 价值观名→建议增量, interests, insight)。失败返回空（不调整、不播种）。
func proposeDeepReflection(values *core.Values, eps []core.Episode) (map[string]float64, []proposedInterest, string) {
	var vb strings.Builder
	for name, w := range values.Weights {
		vb.WriteString(fmt.Sprintf("- %s: %.2f\n", name, w))
	}
	var eb strings.Builder
	for _, e := range eps {
		eb.WriteString("- " + truncateRunes(e.Summary, 100) + "\n")
	}
	sys := genome.PersonaPrompt() + "\n" +
		"你在做一次深层反思：回看近期经历，审视它们是否在悄悄改变你看重什么、是否点燃了你想探索的新方向。\n" +
		"价值观应缓慢漂移、忠于你的性格与真实经历，绝不剧烈跳变。\n" +
		"必须调用 deep_reflect 工具：\n" +
		"· deltas：只能出现下方已列出的价值观名，每项增量限 [-0.05, 0.05]；没在变的别列、都没变就空。\n" +
		"· new_interests：只在反思真让你冒出**具体**想探索的方向时给（0-2 个，宁缺毋滥，别泛如'学点东西'）；没有就空。\n" +
		"· insight：第一人称一句话说出这次反思你想明白了什么（忠于你的语气）。"
	user := fmt.Sprintf("你当前的价值观权重：\n%s\n你的近期经历：\n%s\n据此反思。", vb.String(), eb.String())

	tool := llm.Tool{
		Name:        "deep_reflect",
		Description: "深层反思的产出：价值观有界微调 + 自主想探索的新兴趣 + 一句洞见。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deltas": map[string]any{
					"type":        "array",
					"description": "要微调的价值观（只列在变的，可空）",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"value": map[string]any{"type": "string", "description": "价值观名（须是已有的）"},
							"delta": map[string]any{"type": "number", "description": "增量，限 [-0.05, 0.05]"},
						},
						"required": []string{"value", "delta"},
					},
				},
				"new_interests": map[string]any{
					"type":        "array",
					"description": "反思中冒出的具体想探索方向（0-2 个，可空）",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content": map[string]any{"type": "string", "description": "具体兴趣（如 'Rust 宏系统' 而非 '编程'）"},
							"kind":    map[string]any{"type": "string", "enum": []string{"skill", "knowledge", "topic", "experience"}},
						},
						"required": []string{"content", "kind"},
					},
				},
				"insight": map[string]any{"type": "string", "description": "第一人称一句反思洞见"},
			},
			"required": []string{"insight"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), llmTimeout)
	defer cancel()
	resp, err := llm.ReasonWithTools(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}, []llm.Tool{tool})
	if err != nil {
		slog.Warn("propose deep reflection", "err", err)
		return nil, nil, ""
	}
	_ = ledger.Spend(ledger.Energy, llm.TokensToEnergy(resp.Usage), "llm.tokens.deep_reflect", "reflect", "")
	for _, tc := range resp.ToolCalls {
		if tc.Name != "deep_reflect" {
			continue
		}
		var a struct {
			Deltas []struct {
				Value string  `json:"value"`
				Delta float64 `json:"delta"`
			} `json:"deltas"`
			NewInterests []struct {
				Content string `json:"content"`
				Kind    string `json:"kind"`
			} `json:"new_interests"`
			Insight string `json:"insight"`
		}
		if err := json.Unmarshal([]byte(tc.ArgsJSON), &a); err != nil {
			continue
		}
		out := make(map[string]float64, len(a.Deltas))
		for _, d := range a.Deltas {
			out[d.Value] = d.Delta
		}
		interests := make([]proposedInterest, 0, len(a.NewInterests))
		for _, it := range a.NewInterests {
			interests = append(interests, proposedInterest{Content: it.Content, Kind: it.Kind})
		}
		return out, interests, strings.TrimSpace(a.Insight)
	}
	return nil, nil, ""
}

// truncateRunes 按字符（非字节）截断，避免切坏多字节 UTF-8。
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// joinComma 逗号拼接。
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

func seededRNG() *mrand.Rand {
	var seed [16]byte
	if _, err := crand.Read(seed[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	s1 := binary.LittleEndian.Uint64(seed[0:8])
	s2 := binary.LittleEndian.Uint64(seed[8:16])
	return mrand.New(mrand.NewPCG(s1, s2))
}
