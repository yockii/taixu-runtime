// webfetch：网页抓取分层（docs/SKILLS-AND-TOOLS §9 / R71）。
//
//	Tier 1  http GET 原始 HTML
//	Tier 2  trafilatura（python baseline 包）提取正文 → markdown
//	Tier 3  rod + headless chromium 渲染 JS → 再 trafilatura 提取
//
// 自动升级：Tier 1+2 提取正文过短（< minExtractChars，多半是 SPA 空壳）→ 升 Tier 3。
// 全部经 audit() 落 tool_audit_log，与其他工具一致。
package toolrunner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const (
	// minExtractChars Tier1+2 提取正文低于此值视为失败 → 升 Tier3。
	minExtractChars = 200
	// maxFetchBytes 单页抓取上限（原始 HTML）。
	maxFetchBytes = 4 << 20 // 4MB
	// renderTimeout Tier3 headless 渲染超时。
	renderTimeout = 25 * time.Second
	// trafilaturaTimeout 提取子进程超时。
	trafilaturaTimeout = 20 * time.Second
)

// WebFetch Tier1+2 → 必要时 Tier3。返回提取后的正文 markdown。
//
// result 形如："[tier=2 chars=3401]\n\n<markdown>"，让 LLM 知道走了哪层。
func WebFetch(cycleID int64, url string) (Result, error) {
	return audit(cycleID, "web.fetch", url, func() (string, error) {
		html, err := httpGetBody(url)
		if err != nil {
			return "", err
		}
		md := extractMarkdown(html)
		if len([]rune(md)) >= minExtractChars {
			return fmt.Sprintf("[tier=2 chars=%d]\n\n%s", len([]rune(md)), md), nil
		}
		// 升 Tier 3
		rendered, rerr := renderHTML(url)
		if rerr != nil {
			// Tier3 失败：退回 Tier2 结果（即便短），附说明
			return fmt.Sprintf("[tier=2 chars=%d render_failed=%v]\n\n%s", len([]rune(md)), rerr, md), nil
		}
		md3 := extractMarkdown(rendered)
		return fmt.Sprintf("[tier=3 chars=%d]\n\n%s", len([]rune(md3)), md3), nil
	})
}

// WebRender 强制 Tier3：rod 渲染后提取。用于已知 SPA 站点。
func WebRender(cycleID int64, url string) (Result, error) {
	return audit(cycleID, "web.render", url, func() (string, error) {
		rendered, err := renderHTML(url)
		if err != nil {
			return "", err
		}
		md := extractMarkdown(rendered)
		return fmt.Sprintf("[tier=3 chars=%d]\n\n%s", len([]rune(md)), md), nil
	})
}

// httpGetBody 拉原始 HTML（含 UA，限 maxFetchBytes）。
func httpGetBody(url string) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MindverseBot/0.5; digital-life-runtime)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// extractMarkdown 调 trafilatura（python baseline）把 HTML 提取为正文 markdown。
// 失败或空返回 ""。
func extractMarkdown(html string) string {
	if strings.TrimSpace(html) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), trafilaturaTimeout)
	defer cancel()
	// trafilatura.extract: 从 stdin 读 HTML，输出 markdown；失败返 None（空）。
	const py = `import sys, trafilatura
html = sys.stdin.read()
out = trafilatura.extract(html, output_format="markdown", include_links=True, include_tables=True)
sys.stdout.write(out or "")`
	cmd := exec.CommandContext(ctx, "python3", "-c", py)
	cmd.Stdin = strings.NewReader(html)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// renderHTML Tier3：rod + headless chromium 渲染页面，返回渲染后 DOM HTML。
func renderHTML(url string) (string, error) {
	path := os.Getenv("ROD_BROWSER_BIN")
	if path == "" {
		var ok bool
		path, ok = launcher.LookPath()
		if !ok {
			return "", fmt.Errorf("no chromium found for rod")
		}
	}
	u, err := launcher.New().Bin(path).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Set("headless").
		Launch()
	if err != nil {
		return "", fmt.Errorf("launch chromium: %w", err)
	}
	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}
	defer browser.MustClose()

	page, err := browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return "", fmt.Errorf("open page: %w", err)
	}
	defer func() { _ = page.Close() }() // 防 page 句柄泄漏（browser.Close 前显式关）
	// 等待页面稳定（DOM + 网络空闲），带超时。
	page = page.Timeout(renderTimeout)
	if err := page.WaitStable(800 * time.Millisecond); err != nil {
		// 不致命：可能长轮询站点；取当前 DOM
		_ = err
	}
	html, err := page.HTML()
	if err != nil {
		return "", fmt.Errorf("dump html: %w", err)
	}
	return html, nil
}
