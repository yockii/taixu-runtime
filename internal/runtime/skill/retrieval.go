package skill

import (
	"context"
	"sort"
	"time"

	"taixu.icu/runtime/internal/io/embed"
	"taixu.icu/runtime/internal/storage"
)

// SkillListThreshold 技能数 ≤ 此值时直接全列（无需检索，省一次 embed 调用）。
// 超过才按目标语义检索 top-k，避免技能多了 prompt token 线性膨胀（多数与当前目标无关）。
const SkillListThreshold = 8

// RelevantTopK 检索时注入的相关技能上限。
const RelevantTopK = 8

// RelevantReady 按当前目标文本返回「最该让 LLM 看到」的 ready 技能（真正按需装载）：
//   - 技能数 ≤ 阈值 / 未配嵌入 / 检索失败 → 全列（与旧行为一致，绝不因检索而漏技能）。
//   - 否则用 goalText 语义检索，取 top-k 相关技能。
//
// 懒嵌入自愈：ready 但还没向量的技能，在此顺手补一次（best-effort），无需在装载/结晶处插钩子。
func RelevantReady(goalText string) ([]storage.SkillInstance, error) {
	all, err := ListReady()
	if err != nil {
		return nil, err
	}
	if len(all) <= SkillListThreshold || goalText == "" || !embed.Configured() {
		return all, nil
	}

	mu.Lock()
	lid := lifeID
	mu.Unlock()

	// 懒补向量：给 ready 但缺向量的技能算一次描述向量（best-effort，失败略过）。
	if missing, _ := storage.ReadySkillsMissingEmbedding(lid); len(missing) > 0 {
		for _, s := range missing {
			if s.Description == "" {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			v, e := embed.EmbedOne(ctx, s.Description, false) // 文档端
			cancel()
			if e == nil {
				_ = storage.UpdateSkillEmbedding(s.ID, embed.Encode(v))
			}
		}
	}

	vecs, err := storage.ReadySkillVectors(lid)
	if err != nil || len(vecs) == 0 {
		return all, nil // 降级全列
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	qv, err := embed.EmbedOne(ctx, goalText, true) // query 端
	cancel()
	if err != nil {
		return all, nil
	}

	type scored struct {
		s     storage.SkillInstance
		score float64
	}
	var ranked []scored
	for _, s := range all {
		blob, ok := vecs[s.ID]
		if !ok {
			continue
		}
		dv, e := embed.Decode(blob)
		if e != nil {
			continue
		}
		ranked = append(ranked, scored{s: s, score: embed.Cosine(qv, dv)})
	}
	if len(ranked) == 0 {
		return all, nil
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	k := RelevantTopK
	if k > len(ranked) {
		k = len(ranked)
	}
	out := make([]storage.SkillInstance, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, ranked[i].s)
	}
	return out, nil
}
