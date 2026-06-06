package embed

import (
	"context"
	"log/slog"
	"time"
)

// DocBlobBestEffort 对一段文本求 doc 端嵌入并编码为 []byte 供 storage embedding 列直接写入。
//
// best-effort：未配置 / server 不可达 / 出错 → 记 warn 并返回 nil（调用方写入 NULL，向量留空，
// 检索回退关键词召回）。生命体写入路径绝不因嵌入失败而阻塞或崩溃——这是首要原则。
//
// 自带短超时上下文，避免拖慢写入节拍（episode seal 等在主循环内联）。
func DocBlobBestEffort(text string) []byte {
	if text == "" || !Configured() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	v, err := EmbedOne(ctx, text, false)
	if err != nil {
		slog.Warn("embed: doc embedding skipped (degrade to no-vector)", "err", err)
		return nil
	}
	return Encode(v)
}
