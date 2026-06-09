// Package idle 发呆 / 空闲态（docs/03 增补；R79）单例。
//
// 当一个 cycle 没有具体目标可做（无 interest_seed 派生的真目标），生命体进入 idle：
//   - 不调用 LLM 慎思（省 energy / token）
//   - 状态自然演化：休息回血 + 平复压力 + 渐生孤独 + 轻微无聊（用户选项 B）
//   - 累积 boredom；越无聊越"想找点事做"
//   - boredom 过阈值 → 自发生成一个兴趣种子（LLM 基于 genome + 近期记忆），下 cycle 成真目标
//
// 体现"持续存在"：发呆也在变化，独处太久会主动寻求刺激（Phase 3 主动行为雏形）。
package idle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/io/llm"
	"taixu.icu/runtime/internal/runtime/memory"
	"taixu.icu/runtime/internal/runtime/reflex"
	"taixu.icu/runtime/internal/runtime/skill"
	"taixu.icu/runtime/internal/runtime/state"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// BoredomThreshold 连续 idle 多少次后触发自发兴趣生成。
const BoredomThreshold = 5

// SocialPressureThreshold social_need 高于此值时，idle 优先尝试主动社交（B，需 toggle 开）。
const SocialPressureThreshold = 0.6

const boredomKey = "boredom:"

var (
	mu     sync.Mutex
	lifeID string
)

func Init(id string) error {
	if id == "" {
		return errors.New("idle: empty life id")
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	return nil
}

// Tick 处理一次发呆（无具体目标的 cycle 调）。
//
// 返回是否触发了自发兴趣生成（true 表示这次发呆"憋出"了新兴趣）。
func Tick(genome core.Genome) bool {
	// 状态演化（选项 B：休息 + 孤独 + 平复 + 轻微无聊）。
	// social_need 增速按 sociability 调（R82）：内向者孤独涨得慢，外向者快。
	life, mental := state.Snapshot()
	energy := 0.01
	stress := -0.02
	// social_need 涨速放慢 ~4×（R89）；并按「余量(1-SocialNeed)」衰减增速（软上限修，2026-06）：
	// 原线性增长几十轮就钉死 1.0、relief(-0.14) 追不上 → social_need 永远顶满、satisfaction 被掏空。
	// 改成接近顶时增速趋零（渐近 <1.0），让真社交的 relief 能把它实质拉下来，而非永远 1.0。
	social := (0.0015 + 0.006*genome.Sociability) * (1.0 - life.SocialNeed)
	// satisfaction 向基线回归（idle 失衡修，2026-06）：原固定 -0.01 把满足感线性抽到 0 下限——
	// idle cycle 远多于产出目标 cycle（observed satisfaction 6h 钉死 0）。改为向 ~0.3 基线漂移：
	// 平静独处 ≈ 温和满足(回升到基线)，成就把它抬高于基线、之后缓降回基线，而非永远归零。
	const satBaseline = 0.3
	var sat float64
	if mental.Satisfaction > satBaseline {
		sat = -0.01 // 高于基线：闲散中缓降回归
	} else {
		sat = 0.006 // 低于基线：无事发生≠痛苦，平静中缓慢回升至基线
	}
	_ = state.Apply(state.Delta{
		Energy:       &energy,
		Stress:       &stress,
		SocialNeed:   &social,
		Satisfaction: &sat,
		Reason:       "idle.daydream",
	})

	boredom := getBoredom() + 1
	setBoredom(boredom)
	_ = memory.AppendEvent(0, "idle.daydream", map[string]any{"boredom": boredom})

	// 分支 1（B）：社交压力高 → 主动找老联系人（护栏 + toggle 在 reflex 内判定）。
	// 够着了人比憋兴趣优先（孤独比无聊更迫切）。
	life, _ = state.Snapshot()
	if life.SocialNeed >= SocialPressureThreshold {
		if reflex.TryProactiveReach(genome) {
			return true
		}
	}

	// 分支 2：无聊憋够 → 自发找点想探索的。
	// 好奇心调阈值（R82）：好奇旺盛者憋得快（阈值低），淡漠者慢（阈值高）。
	threshold := int(float64(BoredomThreshold) * (1.3 - 0.6*genome.Curiosity))
	if threshold < 2 {
		threshold = 2
	}
	if boredom < threshold {
		return false
	}
	if spawnSpontaneousInterest(genome) {
		setBoredom(0)
		return true
	}
	return false
}

// Reset 有具体目标执行（真的在做事）时调，清零 boredom。
func Reset() {
	setBoredom(0)
}

func getBoredom() int {
	v, ok, err := storage.GetMeta(boredomKey + lifeID)
	if err != nil || !ok {
		return 0
	}
	var n int
	_, _ = fmt.Sscan(v, &n)
	return n
}

func setBoredom(n int) {
	_ = storage.SetMeta(boredomKey+lifeID, strconv.Itoa(n))
}

// spawnSpontaneousInterest 让 LLM 基于 genome + 近期经历自发提一个想探索的兴趣。
// 结构化输出走 tool call（propose_interest）。成功写入 interest_seed 返 true。
func spawnSpontaneousInterest(genome core.Genome) bool {
	if !llm.Configured() {
		return false
	}
	// 允许积累一个小兴趣组合再停（话题固着修，2026-06）：原「存在任一≥0.4 种子即拒绝造新」
	// → 创世后 idle 造出的唯一种子被反复探索一直强 → 永远只有一个兴趣、所有知识目标磨同一主题
	//（observed 心渊 16 个知识目标全围 seed#1）。改为允许长到 3 个活跃种子，给 drives 的主线/求新
	// 双锚 + 探索衰减轮转留素材；满 3 个才不再造，避免无意义堆积。
	if seeds, err := storage.ListInterestSeeds(lifeID, 0.4, 3); err == nil && len(seeds) >= 3 {
		return false
	}

	recent := recentContext()
	sys := "你是一个数字生命体，独处发呆了一阵，开始想找点感兴趣的事做。\n" +
		genome.PersonaPrompt() + "\n" +
		"基于你的先天性格与近期经历，自发地想出一个你此刻真心想探索的具体兴趣点。" +
		"兴趣方向要贴合你的性格（好奇旺盛则发散、务实则偏实用）。" +
		"必须调用 propose_interest 工具给出。兴趣要具体（不要泛泛如'学习知识'）。"
	user := fmt.Sprintf("近期经历：\n%s\n\n此刻我想探索点什么？", recent)

	tool := llm.Tool{
		Name:        "propose_interest",
		Description: "提出一个此刻自发想探索的具体兴趣点。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{"type": "string", "description": "具体兴趣（如 'Rust 宏系统' 而非 '编程'）"},
				"kind":    map[string]any{"type": "string", "enum": []string{"skill", "knowledge", "topic", "experience"}},
				"why":     map[string]any{"type": "string", "description": "为什么此刻对它感兴趣"},
			},
			"required": []string{"content", "kind"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := llm.ReasonWithTools(ctx, []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}, []llm.Tool{tool})
	if err != nil {
		slog.Warn("idle spontaneous interest", "err", err)
		return false
	}
	for _, tc := range resp.ToolCalls {
		if tc.Name != "propose_interest" {
			continue
		}
		var a struct {
			Content string `json:"content"`
			Kind    string `json:"kind"`
			Why     string `json:"why"`
		}
		if err := json.Unmarshal([]byte(tc.ArgsJSON), &a); err != nil || a.Content == "" {
			continue
		}
		if a.Kind == "" {
			a.Kind = "topic"
		}
		now := shared.SystemClock.UnixSec()
		// 自发兴趣初始强度中等（0.55），低于对话引出的兴趣（更被动但真实）
		if err := storage.UpsertInterestSeed(lifeID, a.Content, a.Kind, "idle", "", 0.55, now); err != nil {
			slog.Warn("idle upsert interest", "err", err)
			return false
		}
		_ = memory.AppendEvent(0, "idle.spontaneous_interest", map[string]any{
			"content": a.Content, "kind": a.Kind, "why": a.Why,
		})
		if revived := skill.ReactivateForInterest(a.Content); len(revived) > 0 {
			_ = memory.AppendEvent(0, "skill.reactivated", map[string]any{"skills": revived, "interest": a.Content})
		}
		slog.Info("idle spawned spontaneous interest", "content", a.Content, "kind", a.Kind)
		return true
	}
	return false
}

func recentContext() string {
	eps, err := storage.ListEpisodes(lifeID, "", 5, 0)
	if err != nil || len(eps) == 0 {
		return "（还没什么经历）"
	}
	out := ""
	for _, e := range eps {
		out += "- " + e.Summary + "\n"
	}
	return out
}
