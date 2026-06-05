// Package lark 飞书 IM 接入（lark-oapi-sdk-go v3 + WebSocket LongConnection）单例。
//
// 入站消息（OnP2MessageReceiveV1）：text / post / image / file / audio —— 富类型经
// MessageResource 下载落 inbox 并合成一句文本提示，统一以文本流喂给：
//   - 反射通道：reflex.Handle 立即处理（agent loop + tool calls + 分段回复）
//   - 慎思感知：perception.Inject 让 scheduler 看到"用户在场"，影响节拍因子
//
// 入站卡片回调（OnP2CardActionTrigger，走长连接，无需公网 webhook）：用户点交互卡片按钮
//   → handleCardAction 解析 action+value → 交注册的 cardActionHandler（main 接 skill 批准/拒绝）
//   → 返回 Toast 即时反馈。
//
// 出站：SpeechEvent → Send（文本）；SendCard/SendApprovalCard（交互卡片）；SendPost/SendImageKey。
package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"mindverse/internal/runtime/perception"
	"mindverse/internal/runtime/reflex"
)

// Config 飞书配置。
type Config struct {
	AppID     string
	AppSecret string
	InboxDir  string // 入站附件（image/file/audio）落地目录；空则不下载、仅记类型提示
}

// CardActionFunc 处理一次卡片按钮回调。返回 (toast 文案, 成功与否)。
// 由 main 注册，转调 skill.ApproveDeps/RejectDeps 等（保持 lark 与业务解耦）。
type CardActionFunc func(action string, value map[string]any) (toast string, ok bool)

var (
	mu                sync.Mutex
	cfg               Config
	appCli            *lark.Client
	wsCli             *larkws.Client
	lastOpenID        string
	ready             bool
	inboxDir          string
	cardActionHandler CardActionFunc
)

// Init 一次性初始化。
func Init(c Config) error {
	if c.AppID == "" || c.AppSecret == "" {
		return errors.New("lark: empty app id / secret")
	}
	mu.Lock()
	defer mu.Unlock()
	cfg = c
	inboxDir = c.InboxDir
	appCli = lark.NewClient(c.AppID, c.AppSecret)
	handler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(handleMessage).
		OnP2CardActionTrigger(handleCardAction)
	wsCli = larkws.NewClient(c.AppID, c.AppSecret, larkws.WithEventHandler(handler))
	ready = true
	return nil
}

// SetCardActionHandler 注册卡片按钮回调处理器（main 调用，接 skill 批准/拒绝）。
func SetCardActionHandler(f CardActionFunc) {
	mu.Lock()
	cardActionHandler = f
	mu.Unlock()
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
	contentJSON, err := json.Marshal(map[string]string{"text": content})
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}
	return sendRaw(openID, larkim.MsgTypeText, string(contentJSON))
}

// SendCard 发送一张交互卡片（cardJSON 为飞书卡片 schema 的 JSON 串）。openID 空则用 lastOpenID。
func SendCard(openID, cardJSON string) error {
	return sendRaw(openID, larkim.MsgTypeInteractive, cardJSON)
}

// SendPost 发送富文本（单段落简化版：标题 + 一段文本）。openID 空则用 lastOpenID。
func SendPost(openID, title, text string) error {
	post := map[string]any{
		"zh_cn": map[string]any{
			"title": title,
			"content": [][]map[string]any{
				{{"tag": "text", "text": text}},
			},
		},
	}
	b, err := json.Marshal(post)
	if err != nil {
		return fmt.Errorf("marshal post: %w", err)
	}
	return sendRaw(openID, larkim.MsgTypePost, string(b))
}

// SendImageKey 发送一张已上传得到 image_key 的图片。openID 空则用 lastOpenID。
func SendImageKey(openID, imageKey string) error {
	b, err := json.Marshal(map[string]string{"image_key": imageKey})
	if err != nil {
		return fmt.Errorf("marshal image: %w", err)
	}
	return sendRaw(openID, larkim.MsgTypeImage, string(b))
}

// SendApprovalCard 发一张技能依赖审批卡片（批准/拒绝按钮，value 带 action+skill_id）。
func SendApprovalCard(openID, skillID, skillName, deps string) error {
	return SendCard(openID, buildApprovalCard(skillID, skillName, deps))
}

// buildApprovalCard 构造审批卡片 JSON（拆出便于单测）。
func buildApprovalCard(skillID, skillName, deps string) string {
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": "orange",
			"title":    map[string]any{"tag": "plain_text", "content": "技能依赖审批"},
		},
		"elements": []any{
			map[string]any{
				"tag":  "div",
				"text": map[string]any{"tag": "lark_md", "content": fmt.Sprintf("技能 **%s** 想安装以下依赖：\n%s", skillName, deps)},
			},
			map[string]any{
				"tag": "action",
				"actions": []any{
					map[string]any{
						"tag":   "button",
						"text":  map[string]any{"tag": "plain_text", "content": "批准一次"},
						"type":  "default",
						"value": map[string]any{"action": "skill_approve", "skill_id": skillID},
					},
					map[string]any{
						"tag":   "button",
						"text":  map[string]any{"tag": "plain_text", "content": "批准类似请求"},
						"type":  "primary",
						"value": map[string]any{"action": "skill_approve_all", "skill_id": skillID},
					},
					map[string]any{
						"tag":   "button",
						"text":  map[string]any{"tag": "plain_text", "content": "拒绝"},
						"type":  "danger",
						"value": map[string]any{"action": "skill_reject", "skill_id": skillID},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(card)
	return string(b)
}

// sendRaw 出站消息统一发送（msgType + content JSON 串）。
func sendRaw(openID, msgType, contentJSON string) error {
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
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("open_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(openID).
			MsgType(msgType).
			Content(contentJSON).
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
	// Phase 0：仅单聊（p2p）。群聊（group）入站暂丢弃——群聊需 @ 检测 / 多方归属 / 群 id 作会话键，
	// 留 Phase 4（Life Network 多方在场）启用。当前单聊路径下 ChatType="direct"。
	if msg.ChatType != nil && *msg.ChatType != "p2p" {
		return nil
	}
	if msg.MessageType == nil {
		return nil
	}
	msgID := ""
	if msg.MessageId != nil {
		msgID = *msg.MessageId
	}

	// 按消息类型合成一条文本喂给反射/慎思（Phase 0.5 无视觉/转写，富类型先落地 + 文本提示）。
	var text string
	switch *msg.MessageType {
	case larkim.MsgTypeText:
		text = extractText(msg.Content)
	case larkim.MsgTypePost:
		text = extractPostText(msg.Content)
	case larkim.MsgTypeImage:
		key := extractKey(msg.Content, "image_key")
		text = ingestResource(ctx, msgID, key, "image", "图片")
	case larkim.MsgTypeFile:
		key := extractKey(msg.Content, "file_key")
		text = ingestResource(ctx, msgID, key, "file", "文件")
	case larkim.MsgTypeAudio:
		key := extractKey(msg.Content, "file_key")
		text = ingestResource(ctx, msgID, key, "file", "语音")
	default:
		return nil // 其余类型（media/sticker/share_*）Phase 0.5 暂不处理
	}
	if text == "" {
		return nil
	}

	openID := ""
	if ev.Event.Sender != nil && ev.Event.Sender.SenderId != nil && ev.Event.Sender.SenderId.OpenId != nil {
		openID = *ev.Event.Sender.SenderId.OpenId
	}
	mu.Lock()
	if openID != "" {
		lastOpenID = openID
	}
	mu.Unlock()

	// 慎思层感知用户在场（影响节拍因子）
	perception.Inject(perception.ExternalRequest{
		ID:      msgID,
		Channel: "feishu",
		From:    openID,
		Content: text,
	})

	// 反射层即时处理对话（goroutine）
	reflex.Handle(reflex.IncomingRequest{
		Channel:  "feishu",
		ChatType: "direct", // Phase 0 仅放行 p2p 单聊
		From:     openID,
		Content:  text,
	})
	return nil
}

// handleCardAction 处理卡片按钮回调（走长连接）。读 action+value，交注册处理器，返回 Toast。
func handleCardAction(ctx context.Context, ev *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	if ev == nil || ev.Event == nil || ev.Event.Action == nil {
		return nil, nil
	}
	value := ev.Event.Action.Value
	action, _ := value["action"].(string)
	if action == "" {
		return nil, nil
	}
	mu.Lock()
	h := cardActionHandler
	mu.Unlock()
	if h == nil {
		return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "warning", Content: "暂无处理器"}}, nil
	}
	msg, ok := h(action, value)
	typ := "success"
	if !ok {
		typ = "error"
	}
	// 异步把原卡片 Patch 成结果卡片（撤掉按钮、显示处理结果），避免按钮一直挂着可重复点。
	// 不在回调响应里塞 card（v1 schema 作 card_json data 会被拒：err 200672），改用消息 Patch API。
	mid := ""
	if ev.Event.Context != nil {
		mid = ev.Event.Context.OpenMessageID
	}
	slog.Info("card action", "action", action, "ok", ok, "open_message_id", mid)
	if mid != "" {
		go patchCardResult(mid, msg, ok)
	}
	return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: typ, Content: msg}}, nil
}

// patchCardResult 把已发出的审批卡片更新为结果卡片（无按钮）。3s 截止外异步跑。
func patchCardResult(messageID, result string, ok bool) {
	mu.Lock()
	cli := appCli
	mu.Unlock()
	if cli == nil {
		return
	}
	b, _ := json.Marshal(buildResultCard(result, ok))
	resp, err := cli.Im.V1.Message.Patch(context.Background(), larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().Content(string(b)).Build()).
		Build())
	if err != nil {
		slog.Warn("lark patch card", "err", err)
		return
	}
	if !resp.Success() {
		slog.Warn("lark patch card rsp", "code", resp.Code, "msg", resp.Msg)
		return
	}
	slog.Info("lark patched card", "message_id", messageID)
}

// buildResultCard 处理后替换原审批卡片的结果卡片（无按钮）。
func buildResultCard(result string, ok bool) map[string]any {
	icon := "✅"
	if !ok {
		icon = "⚠️"
	}
	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": "grey",
			"title":    map[string]any{"tag": "plain_text", "content": "技能依赖审批"},
		},
		"elements": []any{
			map[string]any{
				"tag":  "div",
				"text": map[string]any{"tag": "lark_md", "content": icon + " " + result + "\n\n（详情见观察面板技能状态）"},
			},
		},
	}
}

// ingestResource 下载入站附件到 inbox 并返回一句文本提示；inbox 未配/下载失败则只返类型提示。
func ingestResource(ctx context.Context, msgID, key, resType, label string) string {
	if key == "" || msgID == "" {
		return "[" + label + "]"
	}
	mu.Lock()
	cli := appCli
	dir := inboxDir
	mu.Unlock()
	if cli == nil || dir == "" {
		return "[" + label + "]"
	}
	dlCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := cli.Im.V1.MessageResource.Get(dlCtx, larkim.NewGetMessageResourceReqBuilder().
		MessageId(msgID).FileKey(key).Type(resType).Build())
	if err != nil || !resp.Success() {
		slog.Warn("lark ingest resource", "key", key, "err", err)
		return "[" + label + "]"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "[" + label + "]"
	}
	name := resp.FileName
	if name == "" {
		name = key
	}
	path := filepath.Join(dir, msgID+"_"+sanitizeFileName(name))
	if err := resp.WriteFile(path); err != nil {
		slog.Warn("lark write resource", "path", path, "err", err)
		return "[" + label + "]"
	}
	slog.Info("lark ingested attachment", "type", resType, "path", path)
	return fmt.Sprintf("[%s：%s（已存 %s）]", label, name, path)
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

// extractKey 从 image/file/audio 的 Content JSON 取单个 key（image_key / file_key）。
func extractKey(content *string, field string) string {
	if content == nil {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(*content), &m); err != nil {
		return ""
	}
	return m[field]
}

type postBody struct {
	Title   string     `json:"title"`
	Content [][]postEl `json:"content"`
}

type postEl struct {
	Tag      string `json:"tag"`
	Text     string `json:"text"`
	Href     string `json:"href"`
	UserName string `json:"user_name"`
	ImageKey string `json:"image_key"`
	FileKey  string `json:"file_key"`
}

// extractPostText 把富文本（post）拍平成纯文本（嵌入图片/媒体以占位提示）。
// 容忍两种形状：语言包裹 {"zh_cn":{title,content}} 或直接 {title,content}。
func extractPostText(content *string) string {
	if content == nil {
		return ""
	}
	s := *content
	var langWrap map[string]postBody
	if err := json.Unmarshal([]byte(s), &langWrap); err == nil && len(langWrap) > 0 {
		for _, k := range []string{"zh_cn", "en_us", "ja_jp"} {
			if b, ok := langWrap[k]; ok {
				if t := flattenPost(b); t != "" {
					return t
				}
			}
		}
		for _, b := range langWrap {
			if t := flattenPost(b); t != "" {
				return t
			}
		}
	}
	var b postBody
	if err := json.Unmarshal([]byte(s), &b); err == nil {
		if t := flattenPost(b); t != "" {
			return t
		}
	}
	return strings.TrimSpace(s)
}

func flattenPost(b postBody) string {
	var sb strings.Builder
	if b.Title != "" {
		sb.WriteString(b.Title)
		sb.WriteString("\n")
	}
	for _, para := range b.Content {
		for _, el := range para {
			switch el.Tag {
			case "text":
				sb.WriteString(el.Text)
			case "a":
				if el.Text != "" {
					sb.WriteString(el.Text)
				}
				if el.Href != "" {
					sb.WriteString("(" + el.Href + ")")
				}
			case "at":
				sb.WriteString("@" + el.UserName)
			case "img":
				sb.WriteString("[图片]")
			case "media":
				sb.WriteString("[视频]")
			}
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

// sanitizeFileName 去掉路径分隔符等，防目录穿越。
func sanitizeFileName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	return name
}
