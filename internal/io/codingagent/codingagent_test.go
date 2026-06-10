package codingagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"taixu.icu/runtime/internal/runtime/tools"
)

// TestCodingAgentRoundTrip 验 C7：coding_agent 工具把任务带 bearer token POST 给 bridge，回传输出。
func TestCodingAgentRoundTrip(t *testing.T) {
	_ = tools.Init() // 清空注册表（Init 会 Register）

	var gotAuth, gotPath string
	var gotBody invokeReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"output":"done: wrote run.py","exit_code":0}`))
	}))
	defer srv.Close()

	Init(srv.URL, "tok123", "claude")
	if !Configured() {
		t.Fatal("Init 后应 Configured")
	}

	out, err := handleCodingAgent(context.Background(), tools.Context{}, `{"task":"实现一个加法函数","agent":"codex","workdir":"proj1"}`)
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if gotPath != "/invoke" {
		t.Errorf("应 POST /invoke, got %q", gotPath)
	}
	if gotAuth != "Bearer tok123" {
		t.Errorf("应带 bearer token, got %q", gotAuth)
	}
	if gotBody.Task != "实现一个加法函数" || gotBody.Agent != "codex" || gotBody.Workdir != "proj1" {
		t.Errorf("任务转发不符: %+v", gotBody)
	}
	if !strings.Contains(out, "wrote run.py") {
		t.Errorf("应回传 bridge 输出, got %q", out)
	}
}

// TestCodingAgentDefaultAgent 验未指定 agent 时用默认。
func TestCodingAgentDefaultAgent(t *testing.T) {
	_ = tools.Init()
	var gotAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b invokeReq
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &b)
		gotAgent = b.Agent
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	Init(srv.URL, "", "claude") // 默认 agent=claude，无 token
	if _, err := handleCodingAgent(context.Background(), tools.Context{}, `{"task":"x"}`); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if gotAgent != "claude" {
		t.Errorf("未指定应用默认 claude, got %q", gotAgent)
	}
}
