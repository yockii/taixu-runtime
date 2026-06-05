package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

// TestBuildApprovalCard 校验审批卡片结构：两按钮，value 带 action+skill_id。
func TestBuildApprovalCard(t *testing.T) {
	raw := buildApprovalCard("abc123", "my-skill", "python: numpy")
	var card map[string]any
	if err := json.Unmarshal([]byte(raw), &card); err != nil {
		t.Fatalf("card not valid json: %v", err)
	}
	elements, _ := card["elements"].([]any)
	if len(elements) < 2 {
		t.Fatalf("want >=2 elements, got %d", len(elements))
	}
	// 找 action 元素
	var actions []any
	for _, el := range elements {
		m, _ := el.(map[string]any)
		if m["tag"] == "action" {
			actions, _ = m["actions"].([]any)
		}
	}
	if len(actions) != 2 {
		t.Fatalf("want 2 buttons, got %d", len(actions))
	}
	gotActions := map[string]string{}
	for _, a := range actions {
		m, _ := a.(map[string]any)
		v, _ := m["value"].(map[string]any)
		act, _ := v["action"].(string)
		sid, _ := v["skill_id"].(string)
		if sid != "abc123" {
			t.Errorf("button skill_id=%q want abc123", sid)
		}
		gotActions[act] = sid
	}
	if _, ok := gotActions["skill_approve"]; !ok {
		t.Error("missing skill_approve button")
	}
	if _, ok := gotActions["skill_reject"]; !ok {
		t.Error("missing skill_reject button")
	}
	if !strings.Contains(raw, "my-skill") || !strings.Contains(raw, "numpy") {
		t.Error("card should mention skill name + deps")
	}
}

func TestExtractKey(t *testing.T) {
	img := `{"image_key":"img_xyz"}`
	if got := extractKey(&img, "image_key"); got != "img_xyz" {
		t.Errorf("image_key=%q want img_xyz", got)
	}
	file := `{"file_key":"file_abc","file_name":"a.pdf"}`
	if got := extractKey(&file, "file_key"); got != "file_abc" {
		t.Errorf("file_key=%q want file_abc", got)
	}
	if got := extractKey(&img, "file_key"); got != "" {
		t.Errorf("missing field should be empty, got %q", got)
	}
}

func TestExtractPostText(t *testing.T) {
	// 语言包裹形状
	wrapped := `{"zh_cn":{"title":"标题","content":[[{"tag":"text","text":"你好"},{"tag":"a","text":"链接","href":"http://x"}],[{"tag":"at","user_name":"张三"},{"tag":"img","image_key":"k"}]]}}`
	got := extractPostText(&wrapped)
	for _, want := range []string{"标题", "你好", "链接", "http://x", "@张三", "[图片]"} {
		if !strings.Contains(got, want) {
			t.Errorf("wrapped post missing %q in %q", want, got)
		}
	}

	// 直接形状（无语言包裹）
	direct := `{"title":"T","content":[[{"tag":"text","text":"hi"}]]}`
	if got := extractPostText(&direct); !strings.Contains(got, "hi") || !strings.Contains(got, "T") {
		t.Errorf("direct post flatten failed: %q", got)
	}

	// 非法 JSON → 退化为原串
	bad := `not json`
	if got := extractPostText(&bad); got != "not json" {
		t.Errorf("bad json should fall back to raw, got %q", got)
	}
}

func TestExtractText(t *testing.T) {
	c := `{"text":"  hi  "}`
	if got := extractText(&c); got != "hi" {
		t.Errorf("extractText=%q want hi", got)
	}
}

// TestInitWiring 实跑 Init（含 OnP2CardActionTrigger 注册）+ 卡片回调处理器，防运行时接线 panic。
func TestInitWiring(t *testing.T) {
	if err := Init(Config{AppID: "dummy", AppSecret: "dummy"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if !Configured() {
		t.Fatal("should be configured after Init")
	}
	got := ""
	SetCardActionHandler(func(action string, value map[string]any) (string, bool) {
		got = action + ":" + fmt.Sprint(value["skill_id"])
		return "ok", true
	})
	// 模拟一次卡片回调
	ev := &callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Action: &callback.CallBackAction{Value: map[string]any{"action": "skill_approve", "skill_id": "s1"}},
		},
	}
	resp, err := handleCardAction(context.Background(), ev)
	if err != nil {
		t.Fatalf("handleCardAction: %v", err)
	}
	if got != "skill_approve:s1" {
		t.Errorf("handler got %q want skill_approve:s1", got)
	}
	if resp == nil || resp.Toast == nil || resp.Toast.Type != "success" {
		t.Errorf("want success toast, got %+v", resp)
	}
}
