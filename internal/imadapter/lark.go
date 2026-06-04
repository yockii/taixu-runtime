// Package imadapter 飞书 IM 接入（Phase 0.3 唯一 IM；Phase 3 加多家适配）。
//
// 协议：lark-oapi-sdk-go v3 + WebSocket LongConnection。
//   - 无需公网 IP / Webhook
//   - SDK 自带心跳 + 自动重连
//
// 包级单例风格（用户偏好 2026-06-04）：
//   - Init(cfg, onIncoming) 一次性配置
//   - Run(ctx) 阻塞启动 WS
//   - Send(openID, content) 发送消息
//
// 写权限：无 DB 写。仅事件流转：
//   入站消息 → onIncoming(perception.ExternalRequest)
//   出站 SpeechEvent → Send（由 EventBus 订阅触发）
package imadapter

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

	"mindverse/internal/perception"
)

// Config 飞书配置（来自 .env）。
type Config struct {
	AppID     string
	AppSecret string
}

// OnIncoming 入站消息回调签名。
type OnIncoming func(perception.ExternalRequest)

// 包级单例。
var (
	mu          sync.Mutex
	cfg         Config
	appCli      *lark.Client
	wsCli       *larkws.Client
	onMsg       OnIncoming
	lastOpenID  string // Phase 0 仅 1v1 私聊；记录最近一位用户作为默认收件人
)

// Init 一次性初始化（必备 AppID/AppSecret + onIncoming）。
func Init(c Config, onIncoming OnIncoming) error {
	mu.Lock()
	defer mu.Unlock()
	if c.AppID == "" || c.AppSecret == "" {
		return errors.New("imadapter: empty app id / secret")
	}
	if onIncoming == nil {
		return errors.New("imadapter: nil onIncoming")
	}
	cfg = c
	onMsg = onIncoming
	appCli = lark.NewClient(c.AppID, c.AppSecret)
	handler := dispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(handleMessage)
	wsCli = larkws.NewClient(c.AppID, c.AppSecret, larkws.WithEventHandler(handler))
	return nil
}

// Configured 是否已初始化。
func Configured() bool {
	mu.Lock()
	defer mu.Unlock()
	return appCli != nil
}

// Run 阻塞启动 WS 长连接。ctx 取消则断开。
func Run(ctx context.Context) error {
	mu.Lock()
	c := wsCli
	mu.Unlock()
	if c == nil {
		return errors.New("imadapter: not initialized")
	}
	return c.Start(ctx)
}

// LastSenderOpenID 最近一次入站消息的发送者 open_id（默认收件人）。
func LastSenderOpenID() string {
	mu.Lock()
	defer mu.Unlock()
	return lastOpenID
}

// Send 向 openID 发送一条文本消息。openID 为空则用最近一次入站的发送者。
func Send(openID, content string) error {
	mu.Lock()
	cli := appCli
	if openID == "" {
		openID = lastOpenID
	}
	mu.Unlock()
	if cli == nil {
		return errors.New("imadapter: not initialized")
	}
	if openID == "" {
		return errors.New("imadapter: no openID and no last sender")
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

// handleMessage WS 接收消息回调。
func handleMessage(ctx context.Context, ev *larkim.P2MessageReceiveV1) error {
	if ev == nil || ev.Event == nil || ev.Event.Message == nil {
		return nil
	}
	msg := ev.Event.Message

	// Phase 0.3 仅处理文本消息私聊（chat_type=p2p）。
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
	cb := onMsg
	if openID != "" {
		lastOpenID = openID
	}
	mu.Unlock()
	if cb != nil {
		cb(req)
	}
	return nil
}

// extractText 飞书消息 content 字段是 JSON 字符串如 {"text":"hi"}。
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
