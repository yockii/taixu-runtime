// Command codingbridge 宿主侧编码 agent 桥（C7）。
//
// 容器内 runtime 无法直接拉起宿主上强力的编码 agent（claude / codex）。本服务跑在**宿主机**
// （或远程编码机），监听 HTTP，容器内 runtime 经 host.docker.internal:<port>（或远程 URL）POST
// 一个编码任务过来 → 本服务 headless 拉起编码 agent 在**受限工作目录**里干活 → 回结果。
//
// 安全模型（容器→宿主，跨信任边界，特权升级面，必须收紧）：
//   - bearer token 鉴权：CODINGBRIDGE_TOKEN 双端一致，缺/错一律 401。空 token 拒绝启动。
//   - workdir jail：工作目录强制落在 CODINGBRIDGE_WORKROOT 下，越界路径直接拒（agent 写不出沙箱）。
//   - binary allowlist + 别名：只 claude / codex 两个逻辑 agent 可跑，各自实际二进制名由 env 配
//     （CODINGBRIDGE_BIN_CLAUDE / _CODEX），支持 claude.exe 改名 claude-V153.exe 之类。无任意 shell。
//   - 危险动作默认拒：写仓外 / git 提交 / 推送等 mutating 操作（allow_danger=true）当前**一律拒**
//     （审批闸未接），只允许在 jail 内产出改动供人审/手动应用。结构上默认安全。
//
// ⚠ 已知局限（v1，必须知晓）：workdir jail 只限定 agent 的**启动目录(CWD)**，并**不沙箱化 agent
// 自身的文件系统访问**——claude/codex 凭它自己的工具可读写宿主上 CWD 以外的任意路径。本 v1 的控制集
// = ① token(只有授权容器能投递) ② 危险动作默认拒 ③ CWD jail + 符号链接围栏。**真正的 agent 沙箱化
// （把 agent 关进容器/chroot/受限用户）是后续工作**。另：保持编码 agent 自身的权限确认开着（勿加
// --dangerously-skip-permissions）= 又一层外部控制。部署者须把 codingbridge 跑在可接受此风险的机器上。
//
// 远程链接：CODINGBRIDGE_ADDR 可绑可达地址（默认 127.0.0.1 仅本机）；容器侧 TAIXU_CODINGBRIDGE_URL
// 指向本机 host.docker.internal:<port> 或远程 URL，token 走网络鉴权。
//
// 起：CODINGBRIDGE_TOKEN=xxx go run ./cmd/codingbridge（或 build 出 binary 在宿主跑）。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type config struct {
	addr     string
	token    string
	workRoot string
	bins     map[string]string // 逻辑 agent 名 → 实际二进制名/路径（别名支持）
	timeout  time.Duration
}

func loadConfig() (config, error) {
	c := config{
		addr:     envOr("CODINGBRIDGE_ADDR", "127.0.0.1:8765"),
		token:    os.Getenv("CODINGBRIDGE_TOKEN"),
		workRoot: envOr("CODINGBRIDGE_WORKROOT", "./agent-workspace"),
		bins: map[string]string{
			"claude": envOr("CODINGBRIDGE_BIN_CLAUDE", "claude"),
			"codex":  envOr("CODINGBRIDGE_BIN_CODEX", "codex"),
		},
		timeout: 300 * time.Second,
	}
	if d := os.Getenv("CODINGBRIDGE_TIMEOUT_SEC"); d != "" {
		if n := atoiDefault(d, 0); n > 0 {
			c.timeout = time.Duration(n) * time.Second
		}
	}
	if strings.TrimSpace(c.token) == "" {
		return c, errors.New("CODINGBRIDGE_TOKEN required (refuse to start without auth)")
	}
	abs, err := filepath.Abs(c.workRoot)
	if err != nil {
		return c, fmt.Errorf("workroot abs: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return c, fmt.Errorf("mkdir workroot: %w", err)
	}
	// 规范化 workroot（解析符号链接）→ 后续 jail 围栏据此判越界，免 root 本身是 symlink 时误判。
	if real, e := filepath.EvalSymlinks(abs); e == nil {
		abs = real
	}
	c.workRoot = abs
	return c, nil
}

// invokeReq 容器侧投递的任务。
type invokeReq struct {
	Task       string `json:"task"`        // 给编码 agent 的自然语言任务
	Agent      string `json:"agent"`       // claude / codex（默认 claude）
	Workdir    string `json:"workdir"`     // 相对 workroot 的子目录（jail）；空=default
	AllowDanger bool  `json:"allow_danger"` // 请求仓外/提交/推送等 mutating 动作（当前一律拒）
}

type invokeResp struct {
	OK         bool   `json:"ok"`
	Output     string `json:"output,omitempty"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Err        string `json:"err,omitempty"`
	Workdir    string `json:"workdir,omitempty"`
}

const maxOutput = 64 * 1024 // 回包输出上限，防爆容器上下文

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("codingbridge: config: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "agents": keys(cfg.bins)})
	})
	mux.HandleFunc("/invoke", cfg.handleInvoke)

	log.Printf("codingbridge listening on %s (workroot=%s agents=%v)", cfg.addr, cfg.workRoot, keys(cfg.bins))
	srv := &http.Server{Addr: cfg.addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("codingbridge: serve: %v", err)
	}
}

func (cfg config) handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, invokeResp{Err: "POST only"})
		return
	}
	// bearer token 鉴权（跨信任边界第一闸）。
	if !cfg.authOK(r) {
		writeJSON(w, 401, invokeResp{Err: "unauthorized"})
		return
	}
	var req invokeReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 400, invokeResp{Err: "bad json"})
		return
	}
	if strings.TrimSpace(req.Task) == "" {
		writeJSON(w, 400, invokeResp{Err: "empty task"})
		return
	}
	// 危险动作默认拒（审批闸未接）：mutating/仓外操作一律不放行。
	if req.AllowDanger {
		writeJSON(w, 403, invokeResp{Err: "danger ops (out-of-jail write / commit / push) require host approval — not yet wired; run in jail with allow_danger=false"})
		return
	}
	// binary allowlist + 别名解析。
	agent := req.Agent
	if agent == "" {
		agent = "claude"
	}
	bin, ok := cfg.bins[agent]
	if !ok {
		writeJSON(w, 400, invokeResp{Err: fmt.Sprintf("unknown agent %q (allowed: %v)", agent, keys(cfg.bins))})
		return
	}
	// workdir jail：强制落在 workroot 下。
	wd, err := cfg.jailWorkdir(req.Workdir)
	if err != nil {
		writeJSON(w, 400, invokeResp{Err: err.Error()})
		return
	}
	if err := os.MkdirAll(wd, 0o755); err != nil {
		writeJSON(w, 500, invokeResp{Err: "mkdir workdir: " + err.Error()})
		return
	}
	// 符号链接围栏：mkdir 后解析真实路径，确认仍在 workroot 内（防 jail 内被植入 symlink 指向仓外，
	// 如 agent 上次调用在 jail 里建了个指向 / 的软链）。越界即拒，不在符号链接目标里跑 agent。
	if real, e := filepath.EvalSymlinks(wd); e == nil {
		if real != cfg.workRoot && !strings.HasPrefix(real, cfg.workRoot+string(filepath.Separator)) {
			writeJSON(w, 403, invokeResp{Err: "workdir resolves (via symlink) outside workroot"})
			return
		}
	}

	out, code, dur := runAgent(cfg.timeout, bin, agent, req.Task, wd)
	writeJSON(w, 200, invokeResp{
		OK:         code == 0,
		Output:     truncate(out, maxOutput),
		ExitCode:   code,
		DurationMs: dur,
		Workdir:    wd,
	})
}

// runAgent headless 拉起编码 agent（无任意 shell，固定 arg 模板）。返回 (输出, 退出码, 耗时ms)。
func runAgent(timeout time.Duration, bin, agent, task, workdir string) (string, int, int64) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var argv []string
	switch agent {
	case "claude":
		argv = []string{"-p", task} // Claude Code headless print 模式
	case "codex":
		argv = []string{"exec", task} // codex 非交互执行
	default:
		argv = []string{"-p", task}
	}
	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, argv...)
	cmd.Dir = workdir
	outBytes, err := cmd.CombinedOutput()
	dur := time.Since(start).Milliseconds()
	code := 0
	if err != nil {
		code = 1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			// 二进制不存在/启动失败等
			return fmt.Sprintf("%s\n[bridge] spawn error: %v", string(outBytes), err), -1, dur
		}
	}
	return string(outBytes), code, dur
}

func (cfg config) authOK(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	want := "Bearer " + cfg.token
	return subtleEqual(h, want)
}

// jailWorkdir 解析相对 workroot 的子目录，拒绝越界（绝对路径 / ..）。
func (cfg config) jailWorkdir(sub string) (string, error) {
	sub = strings.TrimSpace(sub)
	if sub == "" {
		sub = "default"
	}
	if filepath.IsAbs(sub) {
		return "", errors.New("workdir must be relative to workroot")
	}
	target := filepath.Clean(filepath.Join(cfg.workRoot, sub))
	if target != cfg.workRoot && !strings.HasPrefix(target, cfg.workRoot+string(filepath.Separator)) {
		return "", errors.New("workdir escapes workroot")
	}
	return target, nil
}

// --- helpers ---

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n[truncated]"
}

// subtleEqual 常量时间比较，防 token 计时侧信道。
func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
