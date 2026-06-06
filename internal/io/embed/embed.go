// Package embed 本地嵌入模型适配器（docs/TECH-STACK §5）单例。
//
// 选型：Qwen3-Embedding-0.6B（Apache-2.0 / 1024 维 / 官方 GGUF / 中英 retrieval 强），
// 经 llama.cpp embedding server 以 OpenAI 兼容 /v1/embeddings 协议提供服务。
//
// 首要原则——优雅降级：所有嵌入调用都是 best-effort。embedding server 未配置 / 不可达 /
// 超时 / 出错时，Embed 返回 error，由调用方决定降级（写入侧跳过向量、检索侧回退关键词召回）。
// 生命体绝不因嵌入失败而阻塞或崩溃。
//
// Qwen3 用法（关键）：
//   - query 端要加 instruct 前缀："Instruct: <task>\nQuery: <text>"（task 用通用检索指令）。
//   - doc（被检索的语料）端不加任何前缀，原文直接嵌入。
//
// 向量编解码：float32 小端 []byte，与 storage 各表 embedding BLOB 列一致（见 codec.go）。
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Dim 是 Qwen3-Embedding-0.6B 的输出维度，固定 1024。
const Dim = 1024

// queryInstruct 是 Qwen3 query 端的通用检索指令（task 部分）。
// 官方建议为不同任务定制 task；Phase 0 单生命跨记忆层通用语义召回，用一条通用检索指令即可。
const queryInstruct = "Given a query, retrieve relevant memories, knowledge and reflections that help answer it"

// Config 嵌入服务配置。
type Config struct {
	BaseURL string        // 如 http://localhost:11435（OpenAI 兼容根，自动补 /v1/embeddings）
	Model   string        // 服务端模型名（llama.cpp server 可任意，仅占位）
	Timeout time.Duration // 单次请求超时
}

var (
	mu     sync.Mutex
	cfg    Config
	client *http.Client
	ready  bool
)

// Init 装配嵌入 client。BaseURL 为空视为未配置（Configured() 返回 false，上层走降级）。
// 可重复调用（替换配置）。
func Init(c Config) {
	mu.Lock()
	defer mu.Unlock()
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if c.Model == "" {
		c.Model = "qwen3-embedding-0.6b"
	}
	cfg = c
	if c.BaseURL == "" {
		client = nil
		ready = false
		return
	}
	client = &http.Client{Timeout: c.Timeout}
	ready = true
}

// Configured 是否已配置嵌入服务（BaseURL 非空）。未配置时上层直接走降级，不必发请求。
func Configured() bool {
	mu.Lock()
	defer mu.Unlock()
	return ready
}

// Available best-effort 探活：向 BaseURL 发一次轻量请求确认 server 可达。
// 仅供回填等"先确认再批量"的场景用；普通调用直接 Embed 即可（失败自然降级）。
func Available(ctx context.Context) bool {
	mu.Lock()
	c := cfg
	cli := client
	ok := ready
	mu.Unlock()
	if !ok || cli == nil {
		return false
	}
	// 用一条最短文本探一次嵌入；成功即视为可用。
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	_, err := embedOnce(ctx, cli, c, []string{"ping"})
	return err == nil
}

// Embed 对 texts 批量求嵌入向量。isQuery=true 时给每条加 Qwen3 instruct 前缀（query 端），
// false 时原文嵌入（doc 端）。返回与 texts 等长的 [][]float32。
//
// best-effort：未配置 / server 不可达 / 维度不符均返回 error，调用方据此降级。
func Embed(ctx context.Context, texts []string, isQuery bool) ([][]float32, error) {
	mu.Lock()
	c := cfg
	cli := client
	ok := ready
	mu.Unlock()
	if !ok || cli == nil {
		return nil, errors.New("embed: not configured")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	input := make([]string, len(texts))
	for i, t := range texts {
		if isQuery {
			input[i] = fmt.Sprintf("Instruct: %s\nQuery: %s", queryInstruct, t)
		} else {
			input[i] = t
		}
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	return embedOnce(ctx, cli, c, input)
}

// EmbedOne 单条便捷封装。
func EmbedOne(ctx context.Context, text string, isQuery bool) ([]float32, error) {
	vs, err := Embed(ctx, []string{text}, isQuery)
	if err != nil {
		return nil, err
	}
	if len(vs) != 1 {
		return nil, fmt.Errorf("embed: expected 1 vector, got %d", len(vs))
	}
	return vs[0], nil
}

// --- OpenAI 兼容 /v1/embeddings 线缆类型 ---

type embedReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// embedOnce 发一次 POST <BaseURL>/v1/embeddings（OpenAI 兼容；llama.cpp server 以
// --embedding 启动即提供此端点）。返回按 index 还原顺序的向量，并校验维度 == Dim。
func embedOnce(ctx context.Context, cli *http.Client, c Config, input []string) ([][]float32, error) {
	body, err := json.Marshal(embedReq{Model: c.Model, Input: input})
	if err != nil {
		return nil, fmt.Errorf("embed: marshal: %w", err)
	}
	url := c.BaseURL + "/v1/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed: status %d", resp.StatusCode)
	}
	var er embedResp
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, fmt.Errorf("embed: decode: %w", err)
	}
	if len(er.Data) != len(input) {
		return nil, fmt.Errorf("embed: expected %d vectors, got %d", len(input), len(er.Data))
	}
	out := make([][]float32, len(input))
	for _, d := range er.Data {
		if d.Index < 0 || d.Index >= len(input) {
			return nil, fmt.Errorf("embed: bad index %d", d.Index)
		}
		if len(d.Embedding) != Dim {
			return nil, fmt.Errorf("embed: dim mismatch: got %d want %d", len(d.Embedding), Dim)
		}
		out[d.Index] = d.Embedding
	}
	for i, v := range out {
		if v == nil {
			return nil, fmt.Errorf("embed: missing vector at index %d", i)
		}
	}
	return out, nil
}
