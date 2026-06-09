package memory

import (
	"strings"
	"testing"

	"taixu.icu/runtime/internal/core"
)

// TestSummarizeFiltersNoise 验证 episode 摘要过滤纯内部节拍噪声、浮出有内容事件的正文。
func TestSummarizeFiltersNoise(t *testing.T) {
	trail := []core.RawTrailEntry{
		{EventType: "cycle.start", Payload: `{"energy":1}`},
		{EventType: "idle.daydream", Payload: `{"boredom":3}`},
		{EventType: "reflex.received", Payload: `{"content":"你好呀，今天在忙什么"}`},
		{EventType: "knowledge.sedimented", Payload: `{"content":"共享内存是最快的 IPC","kind":"knowledge"}`},
		{EventType: "tool.success", Payload: `"web.fetch"`}, // 非对象 → 计数标记
		{EventType: "cycle.start", Payload: `{}`},
		{EventType: "episode.sealed", Payload: `{"id":1}`},
	}
	s := summarize(trail)
	if strings.Contains(s, "cycle.start") || strings.Contains(s, "idle.daydream") || strings.Contains(s, "episode.sealed") {
		t.Errorf("noise event leaked into summary: %q", s)
	}
	if !strings.Contains(s, "你好呀") || !strings.Contains(s, "共享内存") {
		t.Errorf("content not surfaced: %q", s)
	}
	if !strings.Contains(s, "tool.success") {
		t.Errorf("contentless substantive event should appear as marker: %q", s)
	}
	if !strings.Contains(s, "内部节拍") {
		t.Errorf("should note internal-tick count: %q", s)
	}
}

// TestSummarizeIdleOnly 纯 idle 段 → "休息/发呆"。
func TestSummarizeIdleOnly(t *testing.T) {
	trail := []core.RawTrailEntry{
		{EventType: "cycle.start"},
		{EventType: "idle.daydream"},
		{EventType: "idle.daydream"},
		{EventType: "cycle.start"},
	}
	s := summarize(trail)
	if !strings.Contains(s, "休息") {
		t.Errorf("idle-only segment should read as rest: %q", s)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("你好世界", 2); got != "你好…" {
		t.Errorf("truncateRunes=%q want 你好…", got)
	}
	if got := truncateRunes("abc", 5); got != "abc" {
		t.Errorf("short string unchanged, got %q", got)
	}
}
