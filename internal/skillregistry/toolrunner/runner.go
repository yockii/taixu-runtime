// Package toolrunner 生命体能力工具集（docs/TECH-STACK §8）。
//
// Phase 0 内置 4 类（不含 browser）：
//   - http : http.get / http.post
//   - fs   : fs.read / fs.write / fs.list / fs.mkdir（限定 /sandbox/）
//   - script: script.shell / script.python / script.node（容器内 spawn + 60s timeout）
//   - time : time.now / time.tz
//
// 所有调用 → tool_audit_log。
package toolrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mindverse/internal/memoryengine"
)

const (
	SandboxRoot   = "/sandbox"
	ScriptTimeout = 60 * time.Second
	HTTPTimeout   = 30 * time.Second
	MaxBodyBytes  = 1 << 20 // 1 MiB
)

// Result 工具调用结果。
type Result struct {
	Output     string
	DurationMs int64
}

// Runner 工具执行器。
type Runner struct {
	store      *memoryengine.Store
	lifeID     string
	sandboxDir string
	httpClient *http.Client
}

// New 构造。sandboxDir 可被覆盖（非容器内开发用）。
func New(store *memoryengine.Store, lifeID, sandboxDir string) *Runner {
	if sandboxDir == "" {
		sandboxDir = SandboxRoot
	}
	return &Runner{
		store:      store,
		lifeID:     lifeID,
		sandboxDir: sandboxDir,
		httpClient: &http.Client{Timeout: HTTPTimeout},
	}
}

// TimeNow 返回当前 Unix 秒。
func (r *Runner) TimeNow(cycleID int64) (Result, error) {
	return r.audit(cycleID, "time.now", "", func() (string, error) {
		return fmt.Sprintf("%d", time.Now().Unix()), nil
	})
}

// HTTPGet 简单 GET。
func (r *Runner) HTTPGet(cycleID int64, url string) (Result, error) {
	return r.audit(cycleID, "http.get", url, func() (string, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodyBytes))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("HTTP %d %d bytes", resp.StatusCode, len(body)), nil
	})
}

// HTTPPost 简单 POST application/json。
func (r *Runner) HTTPPost(cycleID int64, url, body string) (Result, error) {
	return r.audit(cycleID, "http.post", url, func() (string, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodyBytes))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("HTTP %d %d bytes", resp.StatusCode, len(respBody)), nil
	})
}

// FsWrite 写文件到 /sandbox/。
func (r *Runner) FsWrite(cycleID int64, relPath, content string) (Result, error) {
	return r.audit(cycleID, "fs.write", relPath, func() (string, error) {
		abs, err := r.checkSandbox(relPath)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("wrote %d bytes", len(content)), nil
	})
}

// FsRead 从 /sandbox/ 读文件。
func (r *Runner) FsRead(cycleID int64, relPath string) (Result, error) {
	return r.audit(cycleID, "fs.read", relPath, func() (string, error) {
		abs, err := r.checkSandbox(relPath)
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(abs)
		if err != nil {
			return "", err
		}
		if len(b) > MaxBodyBytes {
			return string(b[:MaxBodyBytes]) + "\n[truncated]", nil
		}
		return string(b), nil
	})
}

// FsList 列出 /sandbox/ 下某目录的条目。
func (r *Runner) FsList(cycleID int64, relPath string) (Result, error) {
	return r.audit(cycleID, "fs.list", relPath, func() (string, error) {
		abs, err := r.checkSandbox(relPath)
		if err != nil {
			return "", err
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			return "", err
		}
		var names []string
		for _, e := range entries {
			n := e.Name()
			if e.IsDir() {
				n += "/"
			}
			names = append(names, n)
		}
		return strings.Join(names, "\n"), nil
	})
}

// FsMkdir 在 /sandbox/ 内建目录。
func (r *Runner) FsMkdir(cycleID int64, relPath string) (Result, error) {
	return r.audit(cycleID, "fs.mkdir", relPath, func() (string, error) {
		abs, err := r.checkSandbox(relPath)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return "", err
		}
		return "ok", nil
	})
}

// ScriptShell 执行 shell 脚本（容器内 sh -c）。60s timeout。
func (r *Runner) ScriptShell(cycleID int64, cmd string) (Result, error) {
	return r.runScript(cycleID, "script.shell", cmd, "sh", "-c", cmd)
}

// ScriptPython 执行 python 脚本。
func (r *Runner) ScriptPython(cycleID int64, code string) (Result, error) {
	return r.runScript(cycleID, "script.python", code, "python3", "-c", code)
}

// ScriptNode 执行 node 脚本。
func (r *Runner) ScriptNode(cycleID int64, code string) (Result, error) {
	return r.runScript(cycleID, "script.node", code, "node", "-e", code)
}

func (r *Runner) runScript(cycleID int64, toolName, args string, name string, scriptArgs ...string) (Result, error) {
	return r.audit(cycleID, toolName, args, func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), ScriptTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, name, scriptArgs...)
		cmd.Dir = r.sandboxDir
		out, err := cmd.CombinedOutput()
		if len(out) > MaxBodyBytes {
			out = append(out[:MaxBodyBytes], []byte("\n[truncated]")...)
		}
		if err != nil {
			return string(out), fmt.Errorf("%s exec: %w", toolName, err)
		}
		return string(out), nil
	})
}

func (r *Runner) checkSandbox(relPath string) (string, error) {
	if filepath.IsAbs(relPath) {
		return "", errors.New("path must be relative to /sandbox/")
	}
	abs := filepath.Join(r.sandboxDir, relPath)
	cleanAbs, _ := filepath.Abs(abs)
	cleanRoot, _ := filepath.Abs(r.sandboxDir)
	if !strings.HasPrefix(cleanAbs, cleanRoot) {
		return "", errors.New("path escapes sandbox")
	}
	return cleanAbs, nil
}

// audit 包装实际执行并落 tool_audit_log。
func (r *Runner) audit(cycleID int64, toolName, argsSummary string, fn func() (string, error)) (Result, error) {
	start := time.Now()
	out, err := fn()
	duration := time.Since(start)
	success := err == nil
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	resultSummary := out
	if len(resultSummary) > 256 {
		resultSummary = resultSummary[:256] + "...[truncated]"
	}
	_ = r.store.AppendToolAudit(r.lifeID, cycleID, toolName, argsSummary, resultSummary,
		duration.Milliseconds(), success, errStr, start.Unix())
	return Result{Output: out, DurationMs: duration.Milliseconds()}, err
}
