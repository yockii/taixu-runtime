package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"taixu.icu/runtime/internal/runtime/lifecfg"
	"taixu.icu/runtime/internal/storage"
)

// 诞生模式：未配置 LLM 的裸二进制首启时，main 起 ServeGenesis 只 serve 诞生页 SPA + 诞生端点，
// 不起感知循环。用户在网页填 LLM + 母语 + 令牌、测连通、提交后 commit 关闭 done，main 停本服务、继续正常 boot。
// 无 token 守卫——首启尚无令牌（令牌正是这一步设的）；提示用户首启在本机完成防公网抢注。

// ServeGenesis 启动诞生模式最小服务。返回 srv（main 用于 commit 后 Shutdown 释放端口）+ done（commit 成功即关闭）。
func ServeGenesis(addr string, web fs.FS) (*http.Server, <-chan struct{}) {
	done := make(chan struct{})
	var once sync.Once
	mux := http.NewServeMux()
	if web != nil {
		mux.Handle("/", spaHandler(web))
	}
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("genesis")) })
	mux.HandleFunc("/api/genesis/status", genesisStatus)
	mux.HandleFunc("/api/genesis/test", genesisTest)
	mux.HandleFunc("/api/genesis/commit", func(w http.ResponseWriter, r *http.Request) {
		if genesisCommit(w, r) {
			once.Do(func() { close(done) })
		}
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("genesis http server", "err", err)
		}
	}()
	slog.Info("genesis mode: web onboarding at", "addr", addr)
	return srv, done
}

// genesisStatus GET /api/genesis/status —— 诞生页据此判断要不要走配置流程 + 预填随机令牌 + 可选母语。
func genesisStatus(w http.ResponseWriter, r *http.Request) {
	hasGenome, _ := storage.HasGenome()
	writeJSON(w, http.StatusOK, map[string]any{
		"needs_config":    !lifecfg.LLMConfigured(),
		"has_genome":      hasGenome,
		"suggested_token": randToken(),
		"langs":           []string{"zh", "en", "ja", "ko", "es", "fr", "de"},
	})
}

type genesisReq struct {
	BaseURL     string `json:"base_url"`
	APIKey      string `json:"api_key"`
	Model       string `json:"model"`
	Temperature string `json:"temperature"`
	Lang        string `json:"lang"`
	Token       string `json:"token"`
}

// genesisTest POST /api/genesis/test —— 测 LLM 连通（不写库）。回 {ok,error?}。
func genesisTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req genesisReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := lifecfg.TestLLM(r.Context(), req.BaseURL, req.APIKey, req.Model); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// genesisCommit POST /api/genesis/commit —— 写全套配置 + 装配 LLM。成功返 true（触发 done，继续 boot）。
// 失败回 200+ok:false（前端读 error）。返回 bool 给 ServeGenesis 决定是否关闭 done。
func genesisCommit(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	var req genesisReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	if req.BaseURL == "" || req.APIKey == "" || req.Model == "" || req.Token == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "base_url / api_key / model / token 必填"})
		return false
	}
	if err := lifecfg.Commit(r.Context(), req.BaseURL, req.APIKey, req.Model, req.Temperature, req.Lang, req.Token); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return false
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return true
}

// randToken 16 字节十六进制随机令牌（诞生页预填，用户可改）。
func randToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "taixu-local"
	}
	return hex.EncodeToString(b)
}
