// Package selfupdate runtime 自更新（平台托管通道）：周期查平台 /api/agent/runtime-version 比对内嵌 version，
// 有新版则下载对应平台二进制、校验 sha256、原子替换自身、re-exec。用户可选自动升级 / 否则通知确认。
//
// 安全：走平台 TLS 通道 + sha256 校验（平台是用户的控制面，可信）。二进制签名为后续加固项。
// 跨平台：Windows 可重命名运行中 exe（先挪 .old 再落新）；Unix 原子 rename 覆盖（旧 inode 保留至 exec）。
package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Asset 一份平台二进制（某 os/arch）。
type Asset struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type manifest struct {
	Enabled bool    `json:"enabled"`
	Version string  `json:"version"`
	Notes   string  `json:"notes"`
	Assets  []Asset `json:"assets"`
}

// Update 一个可用更新（已匹配本平台 asset）。
type Update struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
	Asset   Asset  `json:"-"`
}

var (
	httpClient = &http.Client{Timeout: 60 * time.Second}
	mu         sync.RWMutex
	available  *Update // 最近一次 check 发现的可用更新（供面板/通知读）
)

// Available 当前是否有待应用的新版（面板/通知查）。nil=无。
func Available() *Update {
	mu.RLock()
	defer mu.RUnlock()
	return available
}

func setAvailable(u *Update) { mu.Lock(); available = u; mu.Unlock() }

// Check 查平台清单，比对 current，返回本平台可用更新（无/不可用/同版 → nil）。
func Check(ctx context.Context, platformURL, current string) (*Update, error) {
	base := strings.TrimRight(strings.TrimSpace(platformURL), "/")
	if base == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/agent/runtime-version", nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runtime-version status %d", resp.StatusCode)
	}
	var m manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	if !m.Enabled || m.Version == "" || !newer(m.Version, current) {
		setAvailable(nil)
		return nil, nil
	}
	// 匹配本平台 asset。
	for _, a := range m.Assets {
		if a.OS == runtime.GOOS && a.Arch == runtime.GOARCH {
			u := &Update{Version: m.Version, Notes: m.Notes, Asset: a}
			setAvailable(u)
			return u, nil
		}
	}
	setAvailable(nil)
	return nil, nil // 平台没出本平台的 bin
}

// Apply 下载 update 的二进制 → 校验 sha256 → 原子替换运行中的自身。不 re-exec（调用方决定）。
func Apply(ctx context.Context, platformURL string, u *Update) error {
	if u == nil {
		return fmt.Errorf("no update")
	}
	base := strings.TrimRight(strings.TrimSpace(platformURL), "/")
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	dir := filepath.Dir(exe)
	tmp := filepath.Join(dir, ".taixu-update.tmp")

	url := fmt.Sprintf("%s/api/agent/runtime-download?os=%s&arch=%s", base, runtime.GOOS, runtime.GOARCH)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status %d", resp.StatusCode)
	}
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, u.Asset.SHA256) {
		os.Remove(tmp)
		return fmt.Errorf("sha256 不匹配：期望 %s 得 %s", u.Asset.SHA256, got)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return err
	}
	// 原子替换。Windows 不能直接覆盖运行中 exe → 先把现 exe 挪到 .old，再把新 bin 落到 exe 名。
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("挪旧 exe: %w", err)
		}
		if err := os.Rename(tmp, exe); err != nil {
			_ = os.Rename(old, exe) // 回滚
			return fmt.Errorf("落新 exe: %w", err)
		}
	} else {
		// Unix：rename 原子覆盖；运行中进程持旧 inode 直到 exec，无碍。
		if err := os.Rename(tmp, exe); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("替换 exe: %w", err)
		}
	}
	slog.Info("selfupdate: 已替换二进制", "version", u.Version, "exe", exe)
	return nil
}

// ReExec 原地重启进程（应用新二进制后调；镜像 restart.go）。
func ReExec() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	slog.Info("selfupdate: re-exec into new version", "exe", exe)
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
	return reexecUnix(exe)
}

// Run 后台周期检查。autoUpgrade() 读用户设置（true=自动应用+重启；false=只通知，onUpdate 回调）。
// onUpdate 在发现新版且非自动时调（面板 banner / IM 通知）。首检延迟 startupDelay 避开 boot 抖动。
func Run(ctx context.Context, platformURL, current string, interval time.Duration, autoUpgrade func() bool, onUpdate func(*Update)) {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(90 * time.Second): // boot 后稍等再首检
		}
		check := func() {
			u, err := Check(ctx, platformURL, current)
			if err != nil {
				slog.Debug("selfupdate: check failed", "err", err)
				return
			}
			if u == nil {
				return
			}
			if autoUpgrade != nil && autoUpgrade() {
				slog.Info("selfupdate: 自动升级", "to", u.Version)
				if err := Apply(ctx, platformURL, u); err != nil {
					slog.Warn("selfupdate: 自动升级失败", "err", err)
					return
				}
				if err := ReExec(); err != nil {
					slog.Error("selfupdate: re-exec 失败", "err", err)
				}
				return
			}
			slog.Info("selfupdate: 新版可用（待用户确认）", "version", u.Version)
			if onUpdate != nil {
				onUpdate(u)
			}
		}
		check()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				check()
			}
		}
	}()
}

// newer 判 a 是否比 b 新（语义版本 vX.Y.Z；解析失败/dev → 按字符串不等即视为更新，但 b="" 时不更新）。
func newer(a, b string) bool {
	if b == "" {
		return false
	}
	pa, oka := parseSemver(a)
	pb, okb := parseSemver(b)
	if oka && okb {
		for i := 0; i < 3; i++ {
			if pa[i] != pb[i] {
				return pa[i] > pb[i]
			}
		}
		return false
	}
	// 任一解析失败（如 b="dev"）：a 非空且与 b 不同即视为有更新（dev 本地构建会被提示升到正式版）。
	return a != "" && a != b
}

func parseSemver(s string) ([3]int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.SplitN(s, ".", 3)
	var out [3]int
	if len(parts) != 3 {
		return out, false
	}
	for i := 0; i < 3; i++ {
		// 去掉 -rc / +build 后缀
		num := parts[i]
		for j, r := range num {
			if r < '0' || r > '9' {
				num = num[:j]
				break
			}
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
