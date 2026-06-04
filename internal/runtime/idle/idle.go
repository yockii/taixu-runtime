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

	"mindverse/internal/core"
	"mindverse/internal/io/llm"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/runtime/reflex"
	"mindverse/internal/runtime/state"
	"mindverse/internal/shared"
	"mindverse/internal/storage"
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
	// 状态演化（选项 B：休息 + 孤独 + 平复 + 轻微无聊）
	energy := 0.01
	stress := -0.02
	social := 0.02
	sat := -0.01
	_ = state.Apply(state.Delta{
		Energy:     &energy,
		Stress:     &stress,
		SocialNeed: &social,
		Satisfaction: &sat,
		Reason:     "idle.daydream",
	})

	boredom := getBoredom() + 1
	setBoredom(boredom)
	_ = memory.AppendEvent(0, "idle.daydream", map[string]any{"boredom": boredom})

	// 分支 1（B）：社交压力高 → 主动找老联系人（护栏 + toggle 在 reflex 内判定）。
	// 够着了人比憋兴趣优先（孤独比无聊更迫切）。
	life, _ := state.Snapshot()
	if life.SocialNeed >= SocialPressureThreshold {
		if reflex.TryProactiveReach(genome) {
			return true
		}
	}

	// 分支 2：无聊憋够 → 自发找点想探索的（好奇心越高越容易憋出东西）。
	if boredom < BoredomThreshold {
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
	// 已有较强兴趣时不强行造新的（避免无意义堆积）
	if seeds, err := storage.ListInterestSeeds(lifeID, 0.4, 1); err == nil && len(seeds) > 0 {
		return false
	}

	recent := recentContext()
	sys := "你是一个数字生命体，独处发呆了一阵，开始想找点感兴趣的事做。" +
		"基于你的先天性格与近期经历，自发地想出一个你此刻真心想探索的具体兴趣点。" +
		"必须调用 propose_interest 工具给出。兴趣要具体（不要泛泛如'学习知识'）。"
	user := fmt.Sprintf("我的性格：好奇心%.2f 创造力%.2f 社交性%.2f。\n近期经历：\n%s\n\n此刻我想探索点什么？",
		genome.Curiosity, genome.Creativity, genome.Sociability, recent)

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
