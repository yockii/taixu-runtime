package toolrunner

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// 搜索引擎用 rod（真浏览器）跑，无 key、CN 友好（用户决策 2026-06-08：减少注册 key 心智）。
// 轮流试多个常见引擎，谁先出结果用谁——像真人「搜索→看结果列表→判断挑选→点进去读」的第一步。
//
// 设计意图：原来生命体没有搜索工具，只能让 LLM 从自己（可能过时/臆造）的记忆吐一个 URL 直接
// web.fetch，既不真实又常猜错（证书/EOF 失败）。有了 web.search，research 变成
// 「web.search 多关键词 → 据标题/摘要判断哪些源靠谱 → web.fetch 进去读」。

// perEngineTimeout 单引擎页面加载/抓取超时。收紧到 9s（原 25s）：浏览器只启一次后，
// 三引擎依次试须挤进 action.ToolDispatchTimeout=30s 预算（启动~4s + 3×9s ≈ 31s 最坏，
// 常见 bing 首发命中 ~9-13s）。原「每引擎冷启 chromium×3 + 25s」必爆 30s → "context deadline exceeded"。
const perEngineTimeout = 9 * time.Second

// searchEngine 一个引擎的查询 URL 模板 + 结果元素选择器。
type searchEngine struct {
	name     string
	urlTmpl  string // %s = url-encoded query
	itemSel  string // 单条结果容器
	titleSel string // 容器内 标题<a>（取 text + href）
	snippet  string // 容器内 摘要
}

// 按序尝试（CN 可达优先）。Bing 结构最稳，Baidu/Sogou 兜底。
var engines = []searchEngine{
	{"bing", "https://cn.bing.com/search?q=%s", "li.b_algo", "h2 a", ".b_caption p"},
	{"sogou", "https://www.sogou.com/web?query=%s", ".vrwrap", "h3 a", ".star-wiki, .fz-mid, .space-txt"},
	{"baidu", "https://www.baidu.com/s?wd=%s", ".result, .c-container", "h3 a", ".c-abstract, [class*=content-right]"},
}

// searchResult 一条结果。
type searchResult struct {
	Title, URL, Snippet string
}

// WebSearch 搜索 query，返回给 LLM 的结果列表文本（标题/URL/摘要）。轮流试引擎，首个出结果者胜。
func WebSearch(cycleID int64, query string) (Result, error) {
	return audit(cycleID, "web.search", query, func() (string, error) {
		q := strings.TrimSpace(query)
		if q == "" {
			return "", fmt.Errorf("empty query")
		}
		// 浏览器只启一次，三引擎复用同一实例（修：原 searchOne 每引擎冷启一个 chromium，
		// 3 次启动开销常超 ToolDispatchTimeout=30s → 全引擎失败）。
		path := os.Getenv("ROD_BROWSER_BIN")
		if path == "" {
			var ok bool
			if path, ok = launcher.LookPath(); !ok {
				return "", fmt.Errorf("no chromium for rod")
			}
		}
		u, err := launcher.New().Bin(path).
			Set("no-sandbox").Set("disable-gpu").Set("disable-dev-shm-usage").
			Set("disable-blink-features", "AutomationControlled").
			Set("headless").Launch()
		if err != nil {
			return "", fmt.Errorf("launch browser: %w", err)
		}
		browser := rod.New().ControlURL(u)
		if err := browser.Connect(); err != nil {
			return "", fmt.Errorf("connect browser: %w", err)
		}
		defer browser.MustClose()

		var lastErr error
		for _, e := range engines {
			results, err := searchOne(browser, e, q)
			if err != nil {
				lastErr = err
				continue
			}
			if len(results) == 0 {
				continue
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("搜索「%s」结果（来源 %s）：\n", q, e.name))
			for i, r := range results {
				sb.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, r.Title, r.URL))
				if r.Snippet != "" {
					sb.WriteString("   " + truncateStr(r.Snippet, 160) + "\n")
				}
			}
			sb.WriteString("（判断哪些来源靠谱——跳过内容农场/垃圾站，优先权威/高质量源——再 web.fetch 进去读。可换关键词再 web.search。）")
			return sb.String(), nil
		}
		if lastErr != nil {
			return "", fmt.Errorf("all engines failed: %w", lastErr)
		}
		return fmt.Sprintf("搜索「%s」没有结果，换个关键词再试。", q), nil
	})
}

// searchOne 在已启动的 browser 上跑一个引擎（新开 page），提取至多 8 条结果。
func searchOne(browser *rod.Browser, e searchEngine, query string) ([]searchResult, error) {
	target := fmt.Sprintf(e.urlTmpl, url.QueryEscape(query))
	page, err := browser.Page(proto.TargetCreateTarget{URL: target})
	if err != nil {
		return nil, err
	}
	defer func() { _ = page.Close() }()
	page = page.Timeout(perEngineTimeout)
	// 等「结果容器出现」即可，绝不等整页 network-idle：搜索页的广告/追踪/实时块永不静默，
	// 原 page.WaitStable(800ms) 会一直等到 page 超时 → "all engines failed: context deadline exceeded"。
	// 实证根因：chromium --dump-dom 数秒取到 cn.bing.com 101KB DOM、li.b_algo×10 选择器命中——
	// 网络/浏览器/选择器全无碍，唯 WaitStable 卡。改为等首个 itemSel 出现（命中即返，未现则该引擎超时换下一个）。
	if _, err := page.Element(e.itemSel); err != nil {
		return nil, err
	}
	items, err := page.Elements(e.itemSel)
	if err != nil {
		return nil, err
	}
	var out []searchResult
	for _, it := range items {
		if len(out) >= 8 {
			break
		}
		a, err := it.Element(e.titleSel)
		if err != nil {
			continue
		}
		title, _ := a.Text()
		href, _ := a.Attribute("href")
		title = strings.TrimSpace(title)
		if title == "" || href == nil || *href == "" || !strings.HasPrefix(*href, "http") {
			continue
		}
		snip := ""
		if se, err := it.Element(e.snippet); err == nil {
			if t, err := se.Text(); err == nil {
				snip = strings.Join(strings.Fields(t), " ")
			}
		}
		out = append(out, searchResult{Title: title, URL: *href, Snippet: snip})
	}
	return out, nil
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
