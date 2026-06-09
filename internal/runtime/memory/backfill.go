package memory

import (
	"context"
	"log/slog"

	"taixu.icu/runtime/internal/io/embed"
	"taixu.icu/runtime/internal/storage"
)

// 历史回填：给 embedding 为空的历史记忆行补 doc 向量（锁定长跑生命的旧记忆需要）。
//
// 全程 best-effort + 有界 + 可重入：
//   - 嵌入服务不可用 → 整体跳过（返回 0，不报错）；
//   - 每层每次限量（maxPerLayer），不阻塞主循环；
//   - 幂等：只取 embedding IS NULL 的行，已嵌过的天然跳过；某行嵌不出（server 半途挂）下次再来。
//
// 未来 scale 可换成游标分页持续回填；Phase 0 单生命启动跑一轮 + 手动端点触发足够。

// backfillLayers 是参与回填的三记忆层。
var backfillLayers = []string{"episodic", "semantic", "reflection"}

// BackfillEmbeddings 对各记忆层各回填至多 maxPerLayer 行的空向量。返回成功回填的总行数。
// ctx 取消 / 嵌入服务不可达时尽早返回已完成数，绝不 panic、绝不阻塞调用方过久。
func BackfillEmbeddings(ctx context.Context, maxPerLayer int) int {
	if lifeID == "" || maxPerLayer <= 0 {
		return 0
	}
	if !embed.Configured() {
		slog.Info("backfill: embed not configured, skip")
		return 0
	}
	total := 0
	for _, layer := range backfillLayers {
		select {
		case <-ctx.Done():
			return total
		default:
		}
		total += backfillLayer(ctx, layer, maxPerLayer)
	}
	if total > 0 {
		slog.Info("backfill: embeddings filled", "rows", total)
	}
	return total
}

// backfillLayer 分批回填单层（每批 batch 行，至多 maxRows）。afterID 游标推进保证不重扫已处理行。
func backfillLayer(ctx context.Context, layer string, maxRows int) int {
	const batch = 16
	filled := 0
	var afterID int64
	for filled < maxRows {
		select {
		case <-ctx.Done():
			return filled
		default:
		}
		n := batch
		if rem := maxRows - filled; rem < n {
			n = rem
		}
		rows, err := storage.ListRowsMissingEmbedding(lifeID, layer, afterID, n)
		if err != nil {
			slog.Warn("backfill: list missing", "layer", layer, "err", err)
			return filled
		}
		if len(rows) == 0 {
			return filled
		}
		texts := make([]string, len(rows))
		for i, r := range rows {
			texts[i] = r.Text
			if r.ID > afterID {
				afterID = r.ID
			}
		}
		vecs, err := embed.Embed(ctx, texts, false)
		if err != nil {
			// 嵌入服务半途挂：停止本层，已填的保留，下次可重入续填。
			slog.Warn("backfill: embed batch failed, will retry next run", "layer", layer, "err", err)
			return filled
		}
		for i, r := range rows {
			if err := storage.UpdateEmbedding(layer, r.ID, embed.Encode(vecs[i])); err != nil {
				slog.Warn("backfill: update embedding", "layer", layer, "id", r.ID, "err", err)
				continue
			}
			filled++
		}
	}
	return filled
}
