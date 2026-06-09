// Package llm OpenAI 兼容协议 LLM 适配器（docs/TECH-STACK §6）单例。
//
// Phase 0.3 仅启 Reason + Summarize。Embed 待 bge-m3 接入；Critique 待 Phase 2。
// token 永不暴露给 Agent；usage 由本包内部读出并翻译为 energy。
//
// C1 多模型路由：除 default 模型外，可选配 strong 模型（更强/更贵）。慎思层把硬推理/编码派给
// strong，廉价环（反思洞见 / 摘要 / 对话）仍走 default——「向最强者租单任务天花板」（战略
// project_runtime_strategy_vs_agents 的 now 杠杆①）。未配 strong → 一切回退 default，行为不变。
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

// 模型档名。ModelDefault 必配；ModelStrong 可选（未配则 resolveModel 回退 default）。
const (
	ModelDefault = "default"
	ModelStrong  = "strong"
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
//
//	Role: "system" / "user" / "assistant" / "tool"
//	Content: 文本内容（assistant 可同时有 ToolCalls；tool 角色填 tool_call 结果）
//	ToolCallID: 当 Role=="tool" 时必填，与对应 assistant 之 ToolCall.ID 一致
//	ToolCalls: 当 Role=="assistant" 且模型返回时，承载工具调用
type Message struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []ToolCall
}

// ToolCall LLM 决定调用的一个工具。
type ToolCall struct {
	ID       string
	Name     string
	ArgsJSON string // raw JSON args (per OpenAI function calling)
}

// Tool 工具定义（function calling）。
type Tool struct {
	Name        string
	Description string
	Parameters  any // JSON schema object（用 map[string]any 构造）
}

// Usage 用量统计；用于翻译为 energy。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ReasonResult 推理结果。
type ReasonResult struct {
	Text      string
	ToolCalls []ToolCall
	Usage     Usage
}

// modelClient 一个具名模型档的 client + 配置。
type modelClient struct {
	cfg    Config
	client *openai.Client
}

var (
	mu     sync.Mutex
	models = map[string]*modelClient{} // 档名 → client（"default" 必有；"strong" 可选）
)

// Init 装配 default 模型。可调多次（替换配置）。
func Init(c Config) error { return InitModel(ModelDefault, c) }

// InitModel 装配一个具名模型档（如 ModelStrong）。BaseURL/APIKey/Model 必填。
func InitModel(name string, c Config) error {
	if c.BaseURL == "" || c.APIKey == "" || c.Model == "" {
		return errors.New("llm: incomplete config")
	}
	if c.Timeout == 0 {
		c.Timeout = 60 * time.Second
	}
	clientCfg := openai.DefaultConfig(c.APIKey)
	clientCfg.BaseURL = strings.TrimRight(c.BaseURL, "/")
	mu.Lock()
	defer mu.Unlock()
	models[name] = &modelClient{cfg: c, client: openai.NewClientWithConfig(clientCfg)}
	return nil
}

// Configured 是否已配 default 模型。
func Configured() bool {
	mu.Lock()
	defer mu.Unlock()
	_, ok := models[ModelDefault]
	return ok
}

// HasModel 某具名模型档是否已配（如判断 strong 是否可用、据此决定路由）。
func HasModel(name string) bool {
	mu.Lock()
	defer mu.Unlock()
	_, ok := models[name]
	return ok
}

// resolveModel 取具名模型档；不存在则回退 default。
func resolveModel(name string) (*modelClient, bool) {
	mu.Lock()
	defer mu.Unlock()
	if mc, ok := models[name]; ok {
		return mc, true
	}
	mc, ok := models[ModelDefault]
	return mc, ok
}

// Reason Chat Completions（无 tool，default 模型）。
func Reason(ctx context.Context, msgs []Message) (ReasonResult, error) {
	return reasonInternal(ctx, ModelDefault, msgs, nil)
}

// ReasonWithTools Chat Completions + function calling（default 模型）。
// 返回 ReasonResult 含 Text 与 ToolCalls；调用方负责 agent loop 直至 ToolCalls 空。
func ReasonWithTools(ctx context.Context, msgs []Message, tools []Tool) (ReasonResult, error) {
	return reasonInternal(ctx, ModelDefault, msgs, tools)
}

// ReasonModel 指定模型档的无 tool 推理（如 ModelStrong；未配该档自动回退 default）。
func ReasonModel(ctx context.Context, model string, msgs []Message) (ReasonResult, error) {
	return reasonInternal(ctx, model, msgs, nil)
}

// ReasonWithToolsModel 指定模型档的 function-calling 推理（慎思层把硬推理/编码派给 ModelStrong）。
// 未配该档自动回退 default——故调用方可无脑传 ModelStrong，没配也安全。
func ReasonWithToolsModel(ctx context.Context, model string, msgs []Message, tools []Tool) (ReasonResult, error) {
	return reasonInternal(ctx, model, msgs, tools)
}

func reasonInternal(ctx context.Context, model string, msgs []Message, tools []Tool) (ReasonResult, error) {
	mc, ok := resolveModel(model)
	if !ok {
		return ReasonResult{}, errors.New("llm: not configured")
	}
	c := mc.cfg
	cli := mc.client
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	oaiMsgs := make([]openai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		om := openai.ChatCompletionMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		if len(m.ToolCalls) > 0 {
			om.ToolCalls = make([]openai.ToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				om.ToolCalls = append(om.ToolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.ArgsJSON,
					},
				})
			}
		}
		oaiMsgs = append(oaiMsgs, om)
	}

	req := openai.ChatCompletionRequest{
		Model:       c.Model,
		Messages:    oaiMsgs,
		Temperature: c.Temperature,
	}
	if len(tools) > 0 {
		req.Tools = make([]openai.Tool, 0, len(tools))
		for _, t := range tools {
			req.Tools = append(req.Tools, openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}

	resp, err := cli.CreateChatCompletion(ctx, req)
	if err != nil {
		return ReasonResult{}, fmt.Errorf("llm chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return ReasonResult{}, errors.New("llm: empty choices")
	}
	msg := resp.Choices[0].Message
	out := ReasonResult{
		Text: msg.Content,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
	for _, tc := range msg.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:       tc.ID,
			Name:     tc.Function.Name,
			ArgsJSON: tc.Function.Arguments,
		})
	}
	return out, nil
}

// Summarize 总结（default 模型）。
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
