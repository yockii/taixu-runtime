package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"taixu.icu/runtime/internal/runtime/lifecfg"
	"taixu.icu/runtime/internal/runtime/selfupdate"
)

// runtime 自更新面板端点：查状态 / 应用更新 / 切自动升级开关。
// 平台托管升级通道（selfupdate 包负责 check/下载/校验/替换/re-exec）。

var (
	updCurrentVersion = "dev"
	updPlatformURL    string
)

// ConfigureUpdate boot 注入当前版本 + 平台 URL（供 apply 下载）。
func ConfigureUpdate(currentVersion, platformURL string) {
	if currentVersion != "" {
		updCurrentVersion = currentVersion
	}
	updPlatformURL = platformURL
}

// apiUpdateStatus GET /api/update/status —— 当前版本 + 是否有可用新版 + 自动升级开关。
func apiUpdateStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"current_version": updCurrentVersion,
		"auto_upgrade":    lifecfg.AutoUpgrade(),
	}
	if u := selfupdate.Available(); u != nil {
		resp["available"] = map[string]any{"version": u.Version, "notes": u.Notes}
	} else {
		resp["available"] = nil
	}
	writeJSON(w, http.StatusOK, resp)
}

// apiUpdateApply POST /api/update/apply —— 用户确认升级：下载校验替换 → 700ms 后 re-exec。
// 写操作（withAuth 守卫）。无可用更新先 check 一次。
func apiUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := selfupdate.Available()
	if u == nil {
		// 没缓存就现查一次（用户可能刚收到通知）。
		u, _ = selfupdate.Check(context.Background(), updPlatformURL, updCurrentVersion)
	}
	if u == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "err": "当前已是最新版（无可用更新）"})
		return
	}
	if err := selfupdate.Apply(context.Background(), updPlatformURL, u); err != nil {
		slog.Warn("update apply failed", "err", err)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "err": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": u.Version, "note": "已替换二进制，即将重启到新版"})
	go func() {
		time.Sleep(700 * time.Millisecond)
		if err := selfupdate.ReExec(); err != nil {
			slog.Error("update re-exec failed", "err", err)
		}
	}()
}

// apiUpdateAuto POST /api/update/auto {on:bool} —— 切自动升级开关。写操作。
func apiUpdateAuto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var a struct {
		On bool `json:"on"`
	}
	_ = json.NewDecoder(r.Body).Decode(&a)
	if err := lifecfg.SetAutoUpgrade(a.On); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "err": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "auto_upgrade": a.On})
}
