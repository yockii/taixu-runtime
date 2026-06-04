// Package llm OpenAI 兼容协议 LLM 适配器（docs/TECH-STACK §6）单例。
//
// Phase 0.3 仅启 Reason + Summarize。Embed 待 bge-m3 接入；Critique 待 Phase 2。
// token 永不暴露给 Agent；usage 由本包内部读出并翻译为 energy。
package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Config LLM 配置。
type Config struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float32
	Timeout     time.Duration
}

// Message 一轮对话。
type Message struct {
	Role    string
	Content string
}

// Usage 用量统计；用于翻译为 energy。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ReasonResult 推理结果。
type ReasonResult struct {
	Text  string
	Usage Usage
}

var (
	mu     sync.Mutex
	cfg    Config
	client *openai.Client
	ready  bool
)

// Init 装配 client。可调多次（替换配置）。
func Init(c Config) error {
	if c.BaseURL == "" || c.APIKey == "" || c.Model == "" {
		return errors.New("llm: incomplete config")
	}
	if c.Timeout == 0 {
		c.Timeout = 60 * time.Second
	}
	mu.Lock()
	defer mu.Unlock()
	clientCfg := openai.DefaultConfig(c.APIKey)
	clientCfg.BaseURL = strings.TrimRight(c.BaseURL, "/")
	cfg = c
	client = openai.NewClientWithConfig(clientCfg)
	ready = true
	return nil
}

// Configured 是否已 Init。
func Configured() bool {
	mu.Lock()
	defer mu.Unlock()
	return ready
}

// Reason Chat Completions。
func Reason(ctx context.Context, msgs []Message) (ReasonResult, error) {
	mu.Lock()
	cli := client
	c := cfg
	ok := ready
	mu.Unlock()
	if !ok {
		return ReasonResult{}, errors.New("llm: not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	oaiMsgs := make([]openai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		oaiMsgs = append(oaiMsgs, openai.ChatCompletionMessage{Role: m.Role, Content: m.Content})
	}

	resp, err := cli.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.Model,
		Messages:    oaiMsgs,
		Temperature: c.Temperature,
	})
	if err != nil {
		return ReasonResult{}, fmt.Errorf("llm chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return ReasonResult{}, errors.New("llm: empty choices")
	}
	return ReasonResult{
		Text: resp.Choices[0].Message.Content,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// Summarize 总结。
func Summarize(ctx context.Context, raw string) (ReasonResult, error) {
	return Reason(ctx, []Message{
		{Role: "system", Content: "你是一个简洁的事件摘要器。把输入的事件流概括为不超过两句话。"},
		{Role: "user", Content: raw},
	})
}

// TokensToEnergy 翻译为 energy 消耗。
//   energy_consumed = (prompt + completion) * 0.0001
func TokensToEnergy(u Usage) float64 {
	total := u.PromptTokens + u.CompletionTokens
	if total == 0 {
		total = u.TotalTokens
	}
	return float64(total) * 0.0001
}
