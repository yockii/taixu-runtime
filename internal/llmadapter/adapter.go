// Package llmadapter LLM 适配层（docs/04 §3.2 / §6 / TECH-STACK §6）。
//
// 哲学（docs/04 §3.2.1）：
//   - LLM 是工具，不是生命体本身
//   - token 永不暴露给 Agent；usage 由 adapter 内部读出并翻译为 energy
//   - Reason / Embed / Summarize / Critique 四能力为对 LLM 的统一抽象
//
// Phase 0.3 仅启用 Reason + Summarize（Embed 待 bge-m3 接入；Critique 待 Phase 2）。
package llmadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// Adapter LLM 适配器。
type Adapter struct {
	cfg    Config
	client *openai.Client
}

// New 构造。
func New(cfg Config) *Adapter {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	clientCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientCfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	}
	return &Adapter{cfg: cfg, client: openai.NewClientWithConfig(clientCfg)}
}

// Message 一轮对话。
type Message struct {
	Role    string // "system" / "user" / "assistant"
	Content string
}

// Usage Adapter 内部使用的用量统计；用于翻译为 energy。
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

// Reason Chat Completions 主要能力。
func (a *Adapter) Reason(ctx context.Context, msgs []Message) (ReasonResult, error) {
	if a.cfg.APIKey == "" {
		return ReasonResult{}, errors.New("llm: empty api key")
	}
	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
	defer cancel()

	oaiMsgs := make([]openai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		oaiMsgs = append(oaiMsgs, openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       a.cfg.Model,
		Messages:    oaiMsgs,
		Temperature: a.cfg.Temperature,
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

// Summarize 总结。复用 Reason。
func (a *Adapter) Summarize(ctx context.Context, raw string) (ReasonResult, error) {
	return a.Reason(ctx, []Message{
		{Role: "system", Content: "你是一个简洁的事件摘要器。把输入的事件流概括为不超过两句话。"},
		{Role: "user", Content: raw},
	})
}

// TokensToEnergy 翻译用量为 energy 消耗。
// Phase 0 公式（TECH-STACK §6.3）：
//
//   energy_consumed = (prompt + completion) * 0.0001
func TokensToEnergy(u Usage) float64 {
	total := u.PromptTokens + u.CompletionTokens
	if total == 0 {
		total = u.TotalTokens
	}
	return float64(total) * 0.0001
}
