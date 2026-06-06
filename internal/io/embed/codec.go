package embed

import (
	"encoding/binary"
	"errors"
	"math"
)

// Encode 把 []float32 编码为小端 []byte（每元素 4 字节），与 storage 各表 embedding BLOB 列一致。
// nil / 空向量编码为 nil（DB 存 NULL，表示"无向量"）。
func Encode(v []float32) []byte {
	if len(v) == 0 {
		return nil
	}
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

// Decode 把小端 []byte 还原为 []float32。长度非 4 的倍数视为损坏返回 error。
// 空 / nil 返回 nil, nil（无向量，非错误——调用方据此跳过该行）。
func Decode(b []byte) ([]float32, error) {
	if len(b) == 0 {
		return nil, nil
	}
	if len(b)%4 != 0 {
		return nil, errors.New("embed: corrupt vector blob (len not multiple of 4)")
	}
	n := len(b) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v, nil
}

// Cosine 计算两向量余弦相似度，范围 [-1,1]。长度不等或任一为零向量返回 0。
func Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		na += fa * fa
		nb += fb * fb
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Scored 一条带相似度分的候选。
type Scored struct {
	ID    int64
	Score float64
}

// TopK 暴力法：对 query 向量与一组 (id, embeddingBlob) 候选逐一算 cosine，返回相似度降序的前 k 条。
//
// Phase 0 单生命规模（数千条以内）暴力 cosine 足够，无需向量索引。未来 scale（万级以上 / 多生命）
// 再引 sqlite-vec C 扩展——但 modernc 纯 Go driver 无法稳载 C 扩展，届时需评估 driver 或独立向量库。
//
// 跳过空 blob / 损坏 blob / 维度不符的候选（best-effort，绝不因脏数据 panic）。
func TopK(query []float32, candidates []struct {
	ID   int64
	Blob []byte
}, k int) []Scored {
	if len(query) == 0 || k <= 0 {
		return nil
	}
	scored := make([]Scored, 0, len(candidates))
	for _, c := range candidates {
		v, err := Decode(c.Blob)
		if err != nil || len(v) != len(query) {
			continue
		}
		scored = append(scored, Scored{ID: c.ID, Score: Cosine(query, v)})
	}
	// 简单选择排序取 top-k（候选规模小，避免引排序依赖的开销不值；用标准库 sort 更省心）。
	sortByScoreDesc(scored)
	if len(scored) > k {
		scored = scored[:k]
	}
	return scored
}

func sortByScoreDesc(s []Scored) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Score > s[j-1].Score; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
