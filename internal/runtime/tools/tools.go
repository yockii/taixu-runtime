// Package tools 工具注册中心（docs/SKILLS-AND-TOOLS §6）单例。
//
// 两 lane 模型：
//
//	LaneReflex       反射对话即时回应（轻量、零外部副作用、≤8 轮 agent loop）
//	LaneDeliberative 慎思自主行动（重、可消耗资源、可外部副作用、scheduler 驱动）
//
// 同一 tool 可挂多个 lane（声明在 Tool.Lanes）。Skill 装载时按其 SKILL.md
// frontmatter `lanes` 注册暴露的 tool 子集；core runtime tool 永驻。
//
// 设计纪律：
//   - ListLLMTools 输出按 name 排序 → 让 LLM prompt cache 命中稳定
//   - Register 重名直接报错（不静默覆盖）
//   - Handler 返回字符串供 LLM 看；错误同时返 error 供本地日志
//   - 不在 Handler 内拼 shell 命令（→ H09），需要 exec 走 toolrunner 子包
package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"mindverse/internal/io/llm"
)

// Lane 工具所属通道。
type Lane string

const (
	LaneReflex       Lane = "reflex"
	LaneDeliberative Lane = "deliberative"
)

// Context 调度上下文；reflex / deliberative 共用，按需填字段。
type Context struct {
	LifeID  string
	Channel string // "feishu" / "cli" / "web" / ""（deliberative 通常空）
	From    string // 反射通道：用户标识（飞书 open_id 等）
	CycleID int64  // 慎思通道：当前 cycle
	GoalID  int64  // 慎思通道：当前 goal
}

// Handler 工具执行函数。
//
//	ctx       Go context（含 timeout / 取消信号）
//	tctx      调度上下文（生命体 / 通道 / cycle 等）
//	argsJSON  LLM 返回的原始 JSON 参数串
//
// 返回值：
//
//	result 字符串：会作为 tool 角色消息回灌给 LLM（建议 JSON 化便于 LLM 解析）
//	err    本地错误：用于日志 / 计量；result 仍应填可让 LLM 理解的字符串
type Handler func(ctx context.Context, tctx Context, argsJSON string) (string, error)

// Tool 工具定义。
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema
	Lanes       []Lane
	Handler     Handler
}

// ErrUnknownTool Dispatch 未找到工具。
var ErrUnknownTool = errors.New("tools: unknown tool")

// ErrLaneNotAllowed tool 未注册到该 lane。
var ErrLaneNotAllowed = errors.New("tools: tool not registered on lane")

var (
	mu     sync.RWMutex
	byLane map[Lane]map[string]Tool
)

// Init 初始化空 registry。多次调用会清空（用于测试）。
func Init() error {
	mu.Lock()
	defer mu.Unlock()
	byLane = map[Lane]map[string]Tool{
		LaneReflex:       {},
		LaneDeliberative: {},
	}
	return nil
}

// Register 注册一个工具到其声明的所有 lane。重名报错。
func Register(t Tool) error {
	if t.Name == "" {
		return errors.New("tools: empty name")
	}
	if t.Handler == nil {
		return fmt.Errorf("tools: nil handler for %q", t.Name)
	}
	if len(t.Lanes) == 0 {
		return fmt.Errorf("tools: empty lanes for %q", t.Name)
	}
	mu.Lock()
	defer mu.Unlock()
	if byLane == nil {
		byLane = map[Lane]map[string]Tool{
			LaneReflex:       {},
			LaneDeliberative: {},
		}
	}
	for _, lane := range t.Lanes {
		bucket, ok := byLane[lane]
		if !ok {
			bucket = map[string]Tool{}
			byLane[lane] = bucket
		}
		if _, exists := bucket[t.Name]; exists {
			return fmt.Errorf("tools: %q already registered on lane %q", t.Name, lane)
		}
		bucket[t.Name] = t
	}
	return nil
}

// Unregister 从一个 lane 移除一个工具。skill 卸载时用。
func Unregister(lane Lane, name string) {
	mu.Lock()
	defer mu.Unlock()
	if b, ok := byLane[lane]; ok {
		delete(b, name)
	}
}

// UnregisterAll 从所有 lane 移除一个 tool。skill 卸载常用。
func UnregisterAll(name string) {
	mu.Lock()
	defer mu.Unlock()
	for _, b := range byLane {
		delete(b, name)
	}
}

// ListLLMTools 返回 lane 内所有工具的 llm.Tool 形态（按 name 升序，prompt cache 稳定）。
func ListLLMTools(lane Lane) []llm.Tool {
	mu.RLock()
	defer mu.RUnlock()
	b := byLane[lane]
	out := make([]llm.Tool, 0, len(b))
	for _, t := range b {
		out = append(out, llm.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Names 返回 lane 内已注册的 tool 名（按升序）；观察 / 审计用。
func Names(lane Lane) []string {
	mu.RLock()
	defer mu.RUnlock()
	b := byLane[lane]
	out := make([]string, 0, len(b))
	for k := range b {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Has 判断一个 tool 是否注册到 lane。
func Has(lane Lane, name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if b, ok := byLane[lane]; ok {
		_, exists := b[name]
		return exists
	}
	return false
}

// Dispatch 在 lane 内调用 name 工具。
//
// 返回的 result 字符串始终安全：未找到时返简短 JSON 错误串 + ErrUnknownTool。
// Handler 自身错误同样返 JSON 错误串 + Handler 原 error，供 caller 决定是否继续 agent loop。
func Dispatch(ctx context.Context, lane Lane, tctx Context, name, argsJSON string) (string, error) {
	mu.RLock()
	bucket := byLane[lane]
	t, ok := bucket[name]
	mu.RUnlock()
	if !ok {
		return fmt.Sprintf(`{"ok":false,"err":"unknown tool %q on lane %q"}`, name, lane), ErrUnknownTool
	}
	return t.Handler(ctx, tctx, argsJSON)
}
