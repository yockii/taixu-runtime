// Package toolrunner 生命体能力工具集（docs/TECH-STACK §8）单例。
//
// Phase 0 内置 4 类：http / fs（限 /sandbox/）/ script（容器内 spawn 60s timeout）/ time。
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
	"sync"
	"time"

	"mindverse/internal/bus"
	"mindverse/internal/storage"
)

const (
	SandboxRoot   = "/sandbox"
	ScriptTimeout = 60 * time.Second
	HTTPTimeout   = 30 * time.Second
	MaxBodyBytes  = 1 << 20
)

// Result 工具调用结果。
type Result struct {
	Output     string
	DurationMs int64
}

var (
	mu         sync.Mutex
	lifeID     string
	sandboxDir string
	httpClient = &http.Client{Timeout: HTTPTimeout}
)

// Init 绑定生命体 ID + 沙箱目录（容器内通常 /sandbox）。
func Init(id, sbox string) error {
	if id == "" {
		return errors.New("toolrunner: empty life id")
	}
	if sbox == "" {
		sbox = SandboxRoot
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	sandboxDir = sbox
	return nil
}

func TimeNow(cycleID int64) (Result, error) {
	return audit(cycleID, "time.now", "", func() (string, error) {
		return fmt.Sprintf("%d", time.Now().Unix()), nil
	})
}

func HTTPGet(cycleID int64, url string) (Result, error) {
	return audit(cycleID, "http.get", url, func() (string, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		resp, err := httpClient.Do(req)
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

func HTTPPost(cycleID int64, url, body string) (Result, error) {
	return audit(cycleID, "http.post", url, func() (string, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
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

func FsWrite(cycleID int64, relPath, content string) (Result, error) {
	return audit(cycleID, "fs.write", relPath, func() (string, error) {
		abs, err := checkSandbox(relPath)
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

func FsRead(cycleID int64, relPath string) (Result, error) {
	return audit(cycleID, "fs.read", relPath, func() (string, error) {
		abs, err := checkSandbox(relPath)
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

func FsList(cycleID int64, relPath string) (Result, error) {
	return audit(cycleID, "fs.list", relPath, func() (string, error) {
		abs, err := checkSandbox(relPath)
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

func FsMkdir(cycleID int64, relPath string) (Result, error) {
	return audit(cycleID, "fs.mkdir", relPath, func() (string, error) {
		abs, err := checkSandbox(relPath)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return "", err
		}
		return "ok", nil
	})
}

func ScriptShell(cycleID int64, cmd string) (Result, error) {
	return runScript(cycleID, "script.shell", cmd, "sh", "-c", cmd)
}

func ScriptPython(cycleID int64, code string) (Result, error) {
	return runScript(cycleID, "script.python", code, "python3", "-c", code)
}

func ScriptNode(cycleID int64, code string) (Result, error) {
	return runScript(cycleID, "script.node", code, "node", "-e", code)
}

func runScript(cycleID int64, toolName, args string, name string, scriptArgs ...string) (Result, error) {
	return audit(cycleID, toolName, args, func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), ScriptTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, name, scriptArgs...)
		cmd.Dir = sandboxDir
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

func checkSandbox(relPath string) (string, error) {
	if filepath.IsAbs(relPath) {
		return "", errors.New("path must be relative to /sandbox/")
	}
	abs := filepath.Join(sandboxDir, relPath)
	cleanAbs, _ := filepath.Abs(abs)
	cleanRoot, _ := filepath.Abs(sandboxDir)
	if !strings.HasPrefix(cleanAbs, cleanRoot) {
		return "", errors.New("path escapes sandbox")
	}
	return cleanAbs, nil
}

func audit(cycleID int64, toolName, argsSummary string, fn func() (string, error)) (Result, error) {
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
	_ = storage.AppendToolAudit(lifeID, cycleID, toolName, argsSummary, resultSummary,
		duration.Milliseconds(), success, errStr, start.Unix())
	bus.Publish(bus.ToolAudited{
		LifeID:     lifeID,
		ToolName:   toolName,
		Success:    success,
		DurationMs: duration.Milliseconds(),
	})
	return Result{Output: out, DurationMs: duration.Milliseconds()}, err
}
