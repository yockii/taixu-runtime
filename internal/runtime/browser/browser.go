// Package browser 浏览器 agent 层（C 阶梯④兜底 / D：拟人浏览器操作）单例。
//
// 让数字生命体像真人用户一样操作真实浏览器（导航/读页/点按/输入），用于「无结构化通道（MCP/skill）
// 可用时」的兜底社交/对外行动（注册账号、发帖等）。基于已内置的 rod + headless chromium，不引重栈。
//
// 防 LLM 上下文超限（调研 arXiv 2511.19477）：observe 不吐整页 HTML/大截图，而是用 CDP 无障碍树
// （Accessibility.getFullAXTree）抽出**可交互元素的编号表**（一页约 500~1000 token），act 用编号反查
// 节点；每次只回**最近一张快照**（覆盖式，不累积历史）。
//
// 安全（调研：危险动作必须确定性程序拦截，不靠 LLM 概率判断）：
//   - 整体默认**关闭**（config browser_enabled=false）；用户显式开启才注册这些工具。
//   - 危险动作（注册/登录提交/发布/支付/删除）→ 不直接执行，发审批事件（D.2 接异步审批闸 + 拟人输入）。
//
// 本文件是 D.1：observe/navigate/read/click/type 核心 + 危险动作拦截占位。拟人输入层、异步审批
// 落库恢复、接社交阶梯④与外部自注册流程见 D.2 / D.3。
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"taixu.icu/runtime/internal/bus"
	"taixu.icu/runtime/internal/runtime/tools"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// cfgEnabled 总开关（config KV）。默认 false——浏览器操作能力强、风险高，须用户显式开。
const cfgEnabled = "browser_enabled"

// maxElements observe 一次最多回多少个交互元素（控 context）。
const maxElements = 60

var (
	lifeID string

	mu      sync.Mutex
	browser *rod.Browser
	page    *rod.Page
	refs    map[int]proto.DOMBackendNodeID // observe 编号 → 后端节点 ID（act 反查）
	snapVer int
)

// dangerWords 危险动作关键词（元素名/URL 命中即需审批）：注册/登录提交/发布/支付/删除等不可逆或对外动作。
var dangerWords = []string{
	"注册", "登录", "登陆", "提交", "发布", "发送", "支付", "付款", "购买", "删除", "确认",
	"register", "sign up", "signup", "log in", "login", "sign in", "submit", "post", "publish",
	"pay", "buy", "checkout", "delete", "confirm", "subscribe",
}

// DangerNeedsApprovalEvent 危险浏览器动作请求人工审批（D.2 接异步审批闸）。
type DangerNeedsApprovalEvent struct {
	LifeID  string
	Action  string // click / type / open
	Target  string // 元素名 / URL
	Detail  string
	Channel string
	To      string
}

func (DangerNeedsApprovalEvent) EventName() string { return "browser.danger_needs_approval" }

// Init 装配。lifeID 绑定单例。工具是否注册由 config browser_enabled 决定（默认关）。
func Init(id string) error {
	lifeID = id
	refs = map[int]proto.DOMBackendNodeID{}
	if !storage.GetConfigBool(cfgEnabled, false) {
		return nil // 默认关：不注册任何浏览器工具
	}
	registerTools()
	return nil
}

// Shutdown 关闭浏览器（进程退出时调）。
func Shutdown() {
	mu.Lock()
	defer mu.Unlock()
	if browser != nil {
		_ = browser.Close()
		browser, page = nil, nil
	}
}

func registerTools() {
	reg := func(name, desc string, params map[string]any, h tools.Handler) {
		_ = tools.Register(tools.Tool{Name: name, Description: desc, Parameters: params,
			Lanes: []tools.Lane{tools.LaneDeliberative}, Handler: h})
	}
	reg("browser.open", "打开一个网址（真实浏览器），返回页面可交互元素的编号表。",
		obj(map[string]any{"url": str("要打开的网址")}, "url"), handleOpen)
	reg("browser.observe", "重新读取当前页面的可交互元素编号表（导航/输入后看最新状态）。",
		obj(nil), handleObserve)
	reg("browser.read", "读取当前页面的主要可见文本（了解页面内容时用）。",
		obj(nil), handleRead)
	reg("browser.click", "点击当前页面某个编号的元素（编号来自 observe/open）。",
		obj(map[string]any{"ref": map[string]any{"type": "integer", "description": "元素编号"}}, "ref"), handleClick)
	reg("browser.type", "在当前页面某个编号的输入框里输入文字。",
		obj(map[string]any{"ref": map[string]any{"type": "integer", "description": "输入框编号"}, "text": str("要输入的文字")}, "ref", "text"), handleType)
}

// --- schema 小助手 ---

func str(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
func obj(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

// ensurePage 懒启动浏览器 + 复用单页（持久 profile 复用登录态）。
func ensurePage() (*rod.Page, error) {
	if browser != nil && page != nil {
		return page, nil
	}
	path := envBrowserBin()
	l := launcher.New().Bin(path).
		Set("no-sandbox").Set("disable-gpu").Set("disable-dev-shm-usage").
		Set("disable-blink-features", "AutomationControlled"). // 弱化自动化指纹（拟人第一步）
		Set("headless").
		UserDataDir("/app/data/browser-profile") // 持久化：复用登录会话
	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch chromium: %w", err)
	}
	b := rod.New().ControlURL(u)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	p, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		_ = b.Close()
		return nil, fmt.Errorf("new page: %w", err)
	}
	browser, page = b, p
	return page, nil
}

func envBrowserBin() string {
	if v, ok := launcher.LookPath(); ok {
		return v
	}
	return "/usr/bin/chromium"
}

// observe 抽当前页 a11y 树 → 编号交互元素表（覆盖刷新 refs）。
func observe(p *rod.Page) (string, error) {
	res, err := proto.AccessibilityGetFullAXTree{}.Call(p)
	if err != nil {
		return "", fmt.Errorf("a11y tree: %w", err)
	}
	mu.Lock()
	refs = map[int]proto.DOMBackendNodeID{}
	snapVer++
	mu.Unlock()

	var sb strings.Builder
	info, _ := p.Info()
	if info != nil {
		sb.WriteString("URL: " + info.URL + "\nTITLE: " + info.Title + "\n可交互元素：\n")
	}
	n := 0
	for _, node := range res.Nodes {
		if node == nil || node.Ignored {
			continue
		}
		role := axVal(node.Role)
		if !interactiveRole(role) {
			continue
		}
		name := axVal(node.Name)
		if node.BackendDOMNodeID == 0 {
			continue
		}
		n++
		mu.Lock()
		refs[n] = node.BackendDOMNodeID
		mu.Unlock()
		line := fmt.Sprintf("[%d] %s", n, role)
		if name != "" {
			line += " \"" + truncate(name, 60) + "\""
		}
		sb.WriteString(line + "\n")
		if n >= maxElements {
			sb.WriteString("…(更多元素已略)\n")
			break
		}
	}
	if n == 0 {
		sb.WriteString("（没找到可交互元素，可能是纯展示页或还没加载完，可 browser.read 看正文）\n")
	}
	return sb.String(), nil
}

func interactiveRole(role string) bool {
	switch role {
	case "button", "link", "textbox", "searchbox", "checkbox", "radio", "combobox",
		"menuitem", "tab", "switch", "slider", "textarea", "option":
		return true
	}
	return false
}

func axVal(v *proto.AccessibilityAXValue) string {
	if v == nil {
		return ""
	}
	return v.Value.Str() // gson.JSON：非字符串值返回空串
}

// elementByRef 编号 → rod 元素（经 backendNodeID resolve）。
func elementByRef(p *rod.Page, ref int) (*rod.Element, error) {
	mu.Lock()
	bid, ok := refs[ref]
	mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("编号 %d 不存在（先 browser.observe 看最新编号）", ref)
	}
	obj, err := proto.DOMResolveNode{BackendNodeID: bid}.Call(p)
	if err != nil || obj.Object == nil {
		return nil, fmt.Errorf("resolve node %d: %w", ref, err)
	}
	el, err := p.ElementFromObject(obj.Object)
	if err != nil {
		return nil, fmt.Errorf("element from node %d: %w", ref, err)
	}
	return el, nil
}

func isDangerous(s string) bool {
	low := strings.ToLower(s)
	for _, w := range dangerWords {
		if strings.Contains(low, w) {
			return true
		}
	}
	return false
}

// requestApproval 危险动作 → 发审批事件 + 返回给 LLM「需审批、暂不执行」。D.2 接异步审批后真正执行。
func requestApproval(action, target string) string {
	bus.Publish(DangerNeedsApprovalEvent{
		LifeID: lifeID, Action: action, Target: truncate(target, 120),
		Detail: "浏览器危险动作待用户认可",
	})
	_ = storage.AppendActionLogKind(lifeID, 0, 0, storage.ActionKindDeliberate,
		"browser danger needs approval", "browser."+action, target, "", false,
		shared.SystemClock.UnixSec(), shared.SystemClock.UnixSec())
	return `{"ok":false,"need_approval":true,"note":"这是注册/提交/发布类危险动作，已请求用户认可；得到批准前不会执行。先做别的或等批准。"}`
}

// --- handlers ---

func handleOpen(_ context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		URL string `json:"url"`
	}
	parseArgs(argsJSON, &a)
	if a.URL == "" {
		return `{"ok":false,"err":"missing url"}`, nil
	}
	p, err := ensurePage()
	if err != nil {
		return jsonErr(err), err
	}
	pp := p.Timeout(30 * time.Second)
	if err := pp.Navigate(a.URL); err != nil {
		return jsonErr(err), nil
	}
	_ = pp.WaitStable(800 * time.Millisecond)
	snap, err := observe(p)
	if err != nil {
		return jsonErr(err), nil
	}
	return snap, nil
}

func handleObserve(_ context.Context, _ tools.Context, _ string) (string, error) {
	p, err := ensurePage()
	if err != nil {
		return jsonErr(err), err
	}
	snap, err := observe(p)
	if err != nil {
		return jsonErr(err), nil
	}
	return snap, nil
}

func handleRead(_ context.Context, _ tools.Context, _ string) (string, error) {
	p, err := ensurePage()
	if err != nil {
		return jsonErr(err), err
	}
	el, err := p.Timeout(10 * time.Second).Element("body")
	if err != nil {
		return jsonErr(err), nil
	}
	txt, err := el.Text()
	if err != nil {
		return jsonErr(err), nil
	}
	return truncate(strings.Join(strings.Fields(txt), " "), 4000), nil
}

func handleClick(_ context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		Ref int `json:"ref"`
	}
	parseArgs(argsJSON, &a)
	p, err := ensurePage()
	if err != nil {
		return jsonErr(err), err
	}
	el, err := elementByRef(p, a.Ref)
	if err != nil {
		return jsonErr(err), nil
	}
	label, _ := el.Text()
	if isDangerous(label) {
		return requestApproval("click", label), nil
	}
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return jsonErr(err), nil
	}
	_ = p.WaitStable(600 * time.Millisecond)
	snap, _ := observe(p)
	return "已点击。\n" + snap, nil
}

func handleType(_ context.Context, _ tools.Context, argsJSON string) (string, error) {
	var a struct {
		Ref  int    `json:"ref"`
		Text string `json:"text"`
	}
	parseArgs(argsJSON, &a)
	p, err := ensurePage()
	if err != nil {
		return jsonErr(err), err
	}
	el, err := elementByRef(p, a.Ref)
	if err != nil {
		return jsonErr(err), nil
	}
	// 输入密码类 / 危险字段不在此拦（输入本身不提交）；提交动作在 click 拦截。
	if err := el.Input(a.Text); err != nil {
		return jsonErr(err), nil
	}
	return `{"ok":true,"note":"已输入"}`, nil
}

// --- 小工具 ---

func parseArgs(argsJSON string, v any) {
	s := strings.TrimSpace(argsJSON)
	if s == "" {
		return
	}
	_ = json.Unmarshal([]byte(s), v)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func jsonErr(err error) string {
	return `{"ok":false,"err":` + strconv.Quote(err.Error()) + `}`
}
