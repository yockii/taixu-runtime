// Package embedsvc 本地嵌入服务的「面板自管」生命周期管理器（单例）。
//
// 它把嵌入能力做成生命体控制面（:3000）上的一个开关：用户勾选「嵌入增强记忆」即可——
//   - 本地已有模型 GGUF → 直接拉起 llama-server 子进程；
//   - 没有 → 按网络自动下载（hf-mirror 优先，hf.co 兜底），带实时进度；
//   - 就绪后自动 embed.Init 指向本机端口，并触发一次历史回填。
//
// 与独立 embed 容器方案的区别：llama-server 二进制随 runtime 镜像分发，由 runtime 自身
// 以子进程拉起/杀掉（无需 docker 权限、单容器、模型下到生命体自己的数据卷）。
//
// 首要原则仍是优雅降级：管理器从不阻塞主循环；下载/启动失败只置 error 态，
// 检索回退关键词召回，生命体绝不因嵌入失败而崩溃。开关状态持久化，重启自恢复。
package embedsvc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"taixu.icu/runtime/internal/io/embed"
	"taixu.icu/runtime/internal/storage"
)

// 持久化配置键（复用 storage config KV）。
const (
	cfgEnabled = "embed_enabled"
	cfgQuant   = "embed_quant"
)

// localPort 是 llama-server 监听的本机端口（仅 127.0.0.1，不对外）。
const localPort = 11435

// State 是嵌入服务状态机。
type State string

const (
	StateDisabled    State = "disabled"    // 未启用
	StateDownloading State = "downloading" // 正在下载模型
	StateStarting    State = "starting"    // 子进程启动中（载模型/探活）
	StateReady       State = "ready"       // 就绪，可嵌入
	StateError       State = "error"       // 出错（已降级）
)

// quant 是一个量化档的元信息。
type quant struct {
	Name   string // 档名（Q8_0 / Q4_K_M）
	File   string // GGUF 文件名
	SizeMB int    // 下载体积（MB，约）
	MemMB  int    // 子进程载入约需内存（MB，约）
}

// hfRepo 是 Qwen3 嵌入模型的 HuggingFace 仓库。
const hfRepo = "Qwen/Qwen3-Embedding-0.6B-GGUF"

// quants 支持的量化档。Q8_0 默认（最高质量），Q4_K_M 省内存。
var quants = map[string]quant{
	"Q8_0":   {Name: "Q8_0", File: "Qwen3-Embedding-0.6B-Q8_0.gguf", SizeMB: 639, MemMB: 1536},
	"Q4_K_M": {Name: "Q4_K_M", File: "Qwen3-Embedding-0.6B-Q4_K_M.gguf", SizeMB: 397, MemMB: 1024},
}

// DefaultQuant 默认量化档。
const DefaultQuant = "Q8_0"

// downloadMirrors 下载源（按序尝试）：CN 友好的 hf-mirror 优先，官方兜底。
// %s = repo，%s = 文件名。
var downloadMirrors = []string{
	"https://hf-mirror.com/%s/resolve/main/%s",
	"https://huggingface.co/%s/resolve/main/%s",
}

// Status 是对外（API/面板）暴露的快照。
type Status struct {
	Enabled       bool    `json:"enabled"`
	State         State   `json:"state"`
	Quant         string  `json:"quant"`
	ModelPresent  bool    `json:"model_present"`
	MemEstimateMB int     `json:"mem_estimate_mb"`
	SizeMB        int     `json:"size_mb"`
	Dim           int     `json:"dim"`
	Err           string  `json:"err,omitempty"`
	DownDone      int64   `json:"download_done"`
	DownTotal     int64   `json:"download_total"`
	DownPct       float64 `json:"download_pct"`
}

var (
	mu        sync.Mutex // 保护下列字段
	modelsDir string
	binPath   string
	state     = StateDisabled
	quantName = DefaultQuant
	lastErr   string
	downDone  int64
	downTotal int64
	proc      *exec.Cmd

	opMu    sync.Mutex // 串行化 enable/disable 长操作，避免并发拉起多个子进程
	onReady func()     // 就绪回调（主程序注入：触发历史回填）
	managed bool       // 是否由本管理器接管（外部 TAIXU_EMBED_URL 覆盖时为 false）
)

// Init 装配管理器并按持久化开关自恢复。
//   - mDir：模型存放目录（数据卷内，持久 + 可下载）
//   - bin：llama-server 二进制路径
//   - ready：就绪回调（触发回填；可为 nil）
//
// 若持久化为「已启用」，异步执行 enable 流程（下载/启动），不阻塞启动。
func Init(mDir, bin string, ready func()) {
	mu.Lock()
	modelsDir = mDir
	binPath = bin
	onReady = ready
	managed = true
	quantName = storage.GetConfigString(cfgQuant, DefaultQuant)
	if _, ok := quants[quantName]; !ok {
		quantName = DefaultQuant
	}
	// 默认开（用户决策 2026-06-08）：嵌入是召回主用通道，开机自动启用（有模型直起 / 缺模型自动下）。
	// 关键词召回保留为「嵌入真挂了」的崩溃保险，不再是用户需手动开的并行模式。面板仍可显式关。
	enabled := storage.GetConfigBool(cfgEnabled, true)
	// env 硬开关（部署/观察生命可强制关嵌入：省内存、免下模型）。设了就压过 DB 配置。
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("TAIXU_EMBED_ENABLED"))); v != "" {
		enabled = v == "true" || v == "1" || v == "yes" || v == "on"
	}
	mu.Unlock()

	if enabled {
		go func() {
			if err := bringUp(context.Background(), quantName); err != nil {
				slog.Warn("embedsvc: auto-restore failed; degraded", "err", err)
			}
		}()
	} else {
		slog.Info("embedsvc: disabled; vector retrieval falls back to keyword recall")
	}
}

// Managed 是否由本管理器接管（外部 URL 覆盖时 false，面板开关不可用）。
func Managed() bool {
	mu.Lock()
	defer mu.Unlock()
	return managed
}

// Snapshot 返回当前状态快照（API/面板用）。
func Snapshot() Status {
	mu.Lock()
	defer mu.Unlock()
	q := quants[quantName]
	st := Status{
		Enabled:       state != StateDisabled,
		State:         state,
		Quant:         quantName,
		ModelPresent:  modelExists(quantName),
		MemEstimateMB: q.MemMB,
		SizeMB:        q.SizeMB,
		Dim:           embed.Dim,
		Err:           lastErr,
		DownDone:      downDone,
		DownTotal:     downTotal,
	}
	if downTotal > 0 {
		st.DownPct = float64(downDone) / float64(downTotal) * 100
	}
	return st
}

// Quants 列出支持的量化档（面板下拉用）。
func Quants() []quant {
	return []quant{quants["Q8_0"], quants["Q4_K_M"]}
}

// Enable 启用嵌入（持久化开关 + 异步拉起）。quant 为空用当前/默认档。
func Enable(quantSel string) error {
	mu.Lock()
	if quantSel == "" {
		quantSel = quantName
	}
	if _, ok := quants[quantSel]; !ok {
		mu.Unlock()
		return fmt.Errorf("embedsvc: unknown quant %q", quantSel)
	}
	if !managed {
		mu.Unlock()
		return errors.New("embedsvc: external TAIXU_EMBED_URL override active; panel toggle disabled")
	}
	mu.Unlock()

	if err := storage.SetConfigBool(cfgEnabled, true); err != nil {
		return err
	}
	if err := storage.SetConfigString(cfgQuant, quantSel); err != nil {
		return err
	}
	go func() {
		if err := bringUp(context.Background(), quantSel); err != nil {
			slog.Warn("embedsvc: enable failed; degraded", "err", err)
		}
	}()
	return nil
}

// Disable 停用嵌入（持久化 + 杀子进程 + 解除 embed 配置）。
func Disable() error {
	if err := storage.SetConfigBool(cfgEnabled, false); err != nil {
		return err
	}
	opMu.Lock()
	defer opMu.Unlock()
	stopServer()
	embed.Init(embed.Config{}) // 解除：BaseURL 空 → Configured()=false → 回退关键词召回
	mu.Lock()
	state = StateDisabled
	lastErr = ""
	downDone, downTotal = 0, 0
	mu.Unlock()
	slog.Info("embedsvc: disabled")
	return nil
}

// bringUp 串行执行「确保模型 → 启动子进程 → 探活 → 接线 embed → 回填」。
func bringUp(ctx context.Context, quantSel string) error {
	opMu.Lock()
	defer opMu.Unlock()

	mu.Lock()
	quantName = quantSel
	lastErr = ""
	mu.Unlock()

	if err := ensureModel(ctx, quantSel); err != nil {
		setErr(err)
		return err
	}
	if err := startServer(ctx, quantSel); err != nil {
		setErr(err)
		return err
	}
	mu.Lock()
	state = StateReady
	mu.Unlock()
	slog.Info("embedsvc: ready", "quant", quantSel, "port", localPort)

	if onReady != nil {
		go onReady() // 历史回填，不阻塞
	}
	return nil
}

// ensureModel 确保 GGUF 就位；缺失则下载（带进度）。
func ensureModel(ctx context.Context, quantSel string) error {
	if modelExists(quantSel) {
		return nil
	}
	q := quants[quantSel]
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		return fmt.Errorf("embedsvc: mkdir models: %w", err)
	}
	mu.Lock()
	state = StateDownloading
	downDone, downTotal = 0, 0
	mu.Unlock()

	var lastDlErr error
	for _, tmpl := range downloadMirrors {
		url := fmt.Sprintf(tmpl, hfRepo, q.File)
		slog.Info("embedsvc: downloading model", "quant", quantSel, "url", url)
		if err := downloadFile(ctx, url, filepath.Join(modelsDir, q.File)); err != nil {
			slog.Warn("embedsvc: download source failed, trying next", "url", url, "err", err)
			lastDlErr = err
			continue
		}
		slog.Info("embedsvc: model downloaded", "quant", quantSel, "file", q.File)
		return nil
	}
	return fmt.Errorf("embedsvc: all download sources failed: %w", lastDlErr)
}

// downloadFile 流式下载到 dest（先写 .part，完成后原子改名）。实时更新进度。
func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	cli := &http.Client{} // 大文件，不设整体超时；靠 ctx 取消
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	mu.Lock()
	downTotal = resp.ContentLength
	downDone = 0
	mu.Unlock()

	part := dest + ".part"
	f, err := os.Create(part)
	if err != nil {
		return err
	}
	pw := &progressWriter{}
	_, copyErr := io.Copy(io.MultiWriter(f, pw), resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(part)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(part)
		return closeErr
	}
	if err := os.Rename(part, dest); err != nil {
		return err
	}
	return nil
}

// progressWriter 计数写入器，更新全局下载进度。
type progressWriter struct{}

func (w *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	mu.Lock()
	downDone += int64(n)
	mu.Unlock()
	return n, nil
}

// startServer 拉起 llama-server 子进程并探活，成功后接线 embed 指向本机。
func startServer(ctx context.Context, quantSel string) error {
	q := quants[quantSel]
	mu.Lock()
	state = StateStarting
	bin := binPath
	model := filepath.Join(modelsDir, q.File)
	mu.Unlock()

	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("embedsvc: llama-server binary not found at %s: %w", bin, err)
	}

	stopServer() // 杀掉可能存在的旧子进程

	args := []string{
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", localPort),
		"--embedding", "--pooling", "last",
		"-m", model,
		"--ctx-size", "8192", "--batch-size", "8192",
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout = newLogWriter("llama-server")
	cmd.Stderr = newLogWriter("llama-server")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("embedsvc: start llama-server: %w", err)
	}
	mu.Lock()
	proc = cmd
	mu.Unlock()

	// 子进程意外退出监控：若仍处启用态而进程没了 → 置 error。
	go func(c *exec.Cmd) {
		_ = c.Wait()
		mu.Lock()
		stillMine := proc == c
		if stillMine && state != StateDisabled {
			state = StateError
			lastErr = "llama-server exited unexpectedly"
			proc = nil
		}
		mu.Unlock()
		if stillMine {
			slog.Warn("embedsvc: llama-server exited")
		}
	}(cmd)

	// 接线 embed 指向本机，再探活（模型载入需时间，轮询至多 ~90s）。
	embed.Init(embed.Config{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", localPort),
		Model:   "qwen3-embedding-0.6b",
		Timeout: 30 * time.Second,
	})
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		ok := embed.Available(probeCtx)
		cancel()
		if ok {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return errors.New("embedsvc: llama-server did not become ready in time")
}

// stopServer 杀掉当前子进程（若有）。持锁安全。
func stopServer() {
	mu.Lock()
	c := proc
	proc = nil
	mu.Unlock()
	if c == nil || c.Process == nil {
		return
	}
	_ = c.Process.Kill()
	_, _ = c.Process.Wait()
}

// Shutdown 进程退出时调用：杀子进程（不动持久化开关，下次启动自恢复）。
func Shutdown() {
	opMu.Lock()
	defer opMu.Unlock()
	stopServer()
}

func setErr(err error) {
	mu.Lock()
	state = StateError
	lastErr = err.Error()
	mu.Unlock()
}

// modelExists 调用方持 mu 或不持均可（仅读 modelsDir，启动后不变）。
func modelExists(quantSel string) bool {
	q, ok := quants[quantSel]
	if !ok {
		return false
	}
	fi, err := os.Stat(filepath.Join(modelsDir, q.File))
	return err == nil && fi.Size() > 0
}

// newLogWriter 把子进程输出转发到 slog（debug 级，避免刷屏）。
func newLogWriter(tag string) io.Writer {
	pr, pw := io.Pipe()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				slog.Debug(tag, "out", string(buf[:n]))
			}
			if err != nil {
				return
			}
		}
	}()
	return pw
}
