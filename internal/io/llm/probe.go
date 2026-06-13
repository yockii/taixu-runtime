package llm

import (
	"context"
	"errors"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Probe 用候选配置发一次最小请求验连通——不改在用模型（throwaway client）。
// 诞生 / 界面换 LLM 时测 base/key/model 是否可用，返回 nil=通，err=具体失败（401/超时/模型不存在等）。
func Probe(ctx context.Context, c Config) error {
	if c.BaseURL == "" || c.APIKey == "" || c.Model == "" {
		return errors.New("llm: incomplete config (base/key/model)")
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	cc := openai.DefaultConfig(c.APIKey)
	cc.BaseURL = strings.TrimRight(c.BaseURL, "/")
	cli := openai.NewClientWithConfig(cc)
	cctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	_, err := cli.CreateChatCompletion(cctx, openai.ChatCompletionRequest{
		Model:     c.Model,
		MaxTokens: 1,
		Messages:  []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "ping"}},
	})
	return err
}
