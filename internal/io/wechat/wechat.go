// Package wechat 个人微信接入——直连腾讯官方 iLink/ClawBot 协议（ilinkai.weixin.qq.com，纯 HTTP/JSON）。
// 非企业微信、非逆向：官方个人号 Bot API，扫码登录拿持久 bot_token。结构对齐 internal/io/lark。
// Phase 0 仅文本（item type 1）；媒体（图/语音/文件/视频，AES-128-ECB + CDN）后续。
package wechat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"taixu.icu/runtime/internal/runtime/perception"
	"taixu.icu/runtime/internal/runtime/reflex"
)

const baseURL = "https://ilinkai.weixin.qq.com"

// ChannelName 微信渠道名（入站/出站对齐）。
const ChannelName = "wechat"

var (
	mu        sync.Mutex
	botToken  string
	ctxTokens = map[string]string{} // from_user_id → 最近 context_token（回复必带，回复定位对话窗口）
	httpCli   = &http.Client{Timeout: 45 * time.Second} // 长轮询 hold ≤35s，留余量
)

// Config 微信配置。BotToken 来自扫码登录（lifecfg 持久）。
type Config struct{ BotToken string }

// Init 装配微信渠道。BotToken 空则报错（未登录）。
func Init(c Config) error {
	if c.BotToken == "" {
		return errors.New("wechat: empty bot_token")
	}
	mu.Lock()
	botToken = c.BotToken
	mu.Unlock()
	return nil
}

// Configured 是否已配 bot_token。
func Configured() bool {
	mu.Lock()
	defer mu.Unlock()
	return botToken != ""
}

// headers iLink 固定请求头。X-WECHAT-UIN = base64(随机 uint32 的十进制字符串)，每次变（防重放）。
func headers(token string) http.Header {
	var b [4]byte
	_, _ = rand.Read(b[:])
	uin := strconv.FormatUint(uint64(binary.BigEndian.Uint32(b[:])), 10)
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("AuthorizationType", "ilink_bot_token")
	h.Set("X-WECHAT-UIN", base64.StdEncoding.EncodeToString([]byte(uin)))
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	return h
}

func apiPost(ctx context.Context, path string, body any, token string, out any) error {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header = headers(token)
	resp, err := httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wechat %s: http %d: %s", path, resp.StatusCode, string(data))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// —— 消息结构（WeixinMessage 子集，文本） ——

type item struct {
	Type     int `json:"type"`
	TextItem struct {
		Text string `json:"text"`
	} `json:"text_item"`
}
type weixinMessage struct {
	FromUserID   string `json:"from_user_id"`
	ToUserID     string `json:"to_user_id"`
	MessageType  int    `json:"message_type"` // 1=用户入站
	ContextToken string `json:"context_token"`
	ItemList     []item `json:"item_list"`
}

// Start 长轮询收消息（getupdates，服务器 hold ≤35s）。阻塞，调用方 go Start(ctx)。
// 文本消息 → 存 context_token + perception.Inject（慎思感知在场）+ reflex.Handle（反射即时回）。
func Start(ctx context.Context) {
	var buf string
	for {
		if ctx.Err() != nil {
			return
		}
		mu.Lock()
		tok := botToken
		mu.Unlock()
		var resp struct {
			Ret          int             `json:"ret"`
			Msgs         []weixinMessage `json:"msgs"`
			GetUpdatesBuf string         `json:"get_updates_buf"`
		}
		body := map[string]any{"get_updates_buf": buf, "base_info": map[string]any{"channel_version": "1.0.2"}}
		if err := apiPost(ctx, "/ilink/bot/getupdates", body, tok, &resp); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("wechat getupdates", "err", err)
			time.Sleep(3 * time.Second)
			continue
		}
		if resp.GetUpdatesBuf != "" {
			buf = resp.GetUpdatesBuf // 游标必须每次更新，否则重复收
		}
		for _, m := range resp.Msgs {
			if m.MessageType != 1 || m.FromUserID == "" {
				continue // 仅用户入站
			}
			text := ""
			for _, it := range m.ItemList {
				if it.Type == 1 && it.TextItem.Text != "" {
					text = it.TextItem.Text
					break
				}
			}
			if text == "" {
				continue // 非文本暂跳过（媒体后续）
			}
			mu.Lock()
			ctxTokens[m.FromUserID] = m.ContextToken // 回复定位用
			mu.Unlock()
			perception.Inject(perception.ExternalRequest{
				ID: m.ContextToken, Channel: ChannelName, From: m.FromUserID, Content: text,
			})
			reflex.Handle(reflex.IncomingRequest{
				Channel: ChannelName, ChatType: "direct", From: m.FromUserID, Content: text,
			})
		}
	}
}

// Send 发送文本到某用户（egress 出站）。to=from_user_id；自动带其最近 context_token（必填，否则消息不关联对话）。
func Send(to, content string) error {
	mu.Lock()
	tok, ctok := botToken, ctxTokens[to]
	mu.Unlock()
	if tok == "" {
		return errors.New("wechat: not configured")
	}
	msg := map[string]any{
		"msg": map[string]any{
			"to_user_id":    to,
			"message_type":  2, // BOT 发出
			"message_state": 2, // FINISH
			"context_token": ctok,
			"item_list":     []map[string]any{{"type": 1, "text_item": map[string]any{"text": content}}},
		},
	}
	return apiPost(context.Background(), "/ilink/bot/sendmessage", msg, tok, nil)
}
