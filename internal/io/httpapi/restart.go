package httpapi

import (
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// 自助重启：某些配置（飞书 ws 长连/事件分发在 boot 建立）改后需重启生效。
// 采用进程原地 re-exec —— 不依赖外部监管，裸二进制 / systemd / docker 通杀，单机也能自重启：
//   - Linux/macOS：syscall.Exec 替换当前进程镜像（PID 不变；Go listener 默认 O_CLOEXEC，端口自动释放，无双实例、无端口竞争）。
//   - Windows：无 exec，spawn 新进程 + 退出（裸跑即新实例；docker/systemd 下退出亦被拉起）。
// 数据卷持久（sqlite），重启不丢生命/配置。受 withAuth 写守卫。

// apiRestart POST /api/restart —— 先回 ok，再延迟 re-exec（留时间给响应送达 + 前端转入重连轮询）。
func apiRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	go func() {
		time.Sleep(700 * time.Millisecond)
		if err := reexec(); err != nil {
			slog.Error("self-restart failed", "err", err)
		}
	}()
}

// reexec 原地重启当前进程。
func reexec() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	slog.Info("self-restart: re-exec", "exe", exe)
	if runtime.GOOS == "windows" {
		cmd := exec.Command(exe, os.Args[1:]...)
		cmd.Env = os.Environ()
		cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
		if err := cmd.Start(); err != nil {
			return err
		}
		os.Exit(0)
		return nil
	}
	// Linux/macOS：原地替换镜像（不返回；失败才返回 err）。
	return syscall.Exec(exe, os.Args, os.Environ())
}
