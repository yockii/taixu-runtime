// Package lark 飞书 IM 接入（lark-oapi-sdk-go v3 + WebSocket LongConnection）单例。
//
// 入站消息 → perception.Inject（Phase 0 仅 P2 私聊文本）。
// 出站 SpeechEvent → Send（默认收件人为最近一位入站发送者）。
package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"mindverse/internal/runtime/perception"
)

// Config 飞书配置。
type Config struct {
	AppID     string
	AppSecret string
}

var (
	mu         sync.Mutex
	cfg        Config
	appCli     *lark.Client
	wsCli      *larkws.Client
	lastOpenID string
	ready      bool
)

// Init 一次性初始化。
func Init(c Config) error {
	if c.AppID == "" || c.AppSecret == "" {
		return errors.New("lark: empty app id / secret")
	}
	mu.Lock()
	defer mu.Unlock()
	cfg = c
	appCli = lark.NewClient(c.AppID, c.AppSecret)
	handler := dispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(handleMessage)
	wsCli = larkws.NewClient(c.AppID, c.AppSecret, larkws.WithEventHandler(handler))
	ready = true
	return nil
}

// Configured 是否已 Init。
func Configured() bool {
	mu.Lock()
	defer mu.Unlock()
	return ready
}

// Run 阻塞启动 WS。
func Run(ctx context.Context) error {
	mu.Lock()
	c := wsCli
	mu.Unlock()
	if c == nil {
		return errors.New("lark: not initialized")
	}
	return c.Start(ctx)
}

// LastSenderOpenID 最近一次入站消息的发送者 open_id。
func LastSenderOpenID() string {
	mu.Lock()
	defer mu.Unlock()
	return lastOpenID
}

// Send 发送文本。openID 空则用 lastOpenID。
func Send(openID, content string) error {
	mu.Lock()
	cli := appCli
	if openID == "" {
		openID = lastOpenID
	}
	mu.Unlock()
	if cli == nil {
		return errors.New("lark: not initialized")
	}
	if openID == "" {
		return errors.New("lark: no openID and no last sender")
	}
	contentJSON, err := json.Marshal(map[string]string{"text": content})
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("open_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(openID).
			MsgType(larkim.MsgTypeText).
			Content(string(contentJSON)).
			Build()).
		Build()
	resp, err := cli.Im.V1.Message.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("lark send: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("lark send rsp: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func handleMessage(ctx context.Context, ev *larkim.P2MessageReceiveV1) error {
	if ev == nil || ev.Event == nil || ev.Event.Message == nil {
		return nil
	}
	msg := ev.Event.Message
	if msg.ChatType != nil && *msg.ChatType != "p2p" {
		return nil
	}
	if msg.MessageType == nil || *msg.MessageType != "text" {
		return nil
	}
	text := extractText(msg.Content)
	if text == "" {
		return nil
	}
	openID := ""
	if ev.Event.Sender != nil && ev.Event.Sender.SenderId != nil && ev.Event.Sender.SenderId.OpenId != nil {
		openID = *ev.Event.Sender.SenderId.OpenId
	}
	msgID := ""
	if msg.MessageId != nil {
		msgID = *msg.MessageId
	}
	req := perception.ExternalRequest{
		ID:      msgID,
		Channel: "feishu",
		From:    openID,
		Content: text,
	}
	mu.Lock()
	if openID != "" {
		lastOpenID = openID
	}
	mu.Unlock()
	perception.Inject(req)
	return nil
}

func extractText(content *string) string {
	if content == nil {
		return ""
	}
	var v struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(*content), &v); err != nil {
		return strings.TrimSpace(*content)
	}
	return strings.TrimSpace(v.Text)
}
