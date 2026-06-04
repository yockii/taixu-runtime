// Package httpapi 观察 API（Phase 0.4 全套）单例。
//
// 路由：
//   GET  /api/state                  实时 LifeState/MentalState
//   GET  /api/genome                 静态 Genome
//   GET  /api/values                 价值观权重表
//   GET  /api/episodes?q=&limit=&offset=
//   GET  /api/goals?status=&limit=
//   GET  /api/reflections?limit=
//   GET  /api/actions?limit=
//   GET  /api/tools/audit?limit=
//   GET  /api/ledger?resource=&limit=
//   GET  /api/config                 LLM/飞书 sanitize
//   GET  /api/stream                 SSE 实时推送（state/ticks/speech/lifecycle）
//   POST /api/external-request       手动注入
//   GET  /healthz
//
// 前端 SPA 由 /（embed.FS）服务，由 cmd/runtime 注入 fs.FS。
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"mindverse/internal/runtime/perception"
	"mindverse/internal/runtime/reflex"
	"mindverse/internal/runtime/skill"
	"mindverse/internal/runtime/state"
	"mindverse/internal/storage"
)

var (
	mu     sync.Mutex
	lifeID string
	webFS  fs.FS // SvelteKit build (embed.FS root)
)

// Init 绑定生命体 ID。可可选传 SPA 静态文件系统。
func Init(id string, web fs.FS) error {
	if id == "" {
		return errors.New("httpapi: empty life id")
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	webFS = web
	return nil
}

// Start 启动 HTTP 服务。
func Start(ctx context.Context, addr string) *http.Server {
	mux := http.NewServeMux()

	// SPA / 静态资源
	mu.Lock()
	w := webFS
	mu.Unlock()
	if w != nil {
		mux.Handle("/", spaHandler(w))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte("Mindverse Runtime (no SPA embedded). Use /api/* endpoints."))
		})
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/api/state", apiState)
	mux.HandleFunc("/api/lifecycle", apiLifecycle)
	mux.HandleFunc("/api/genome", apiGenome)
	mux.HandleFunc("/api/values", apiValues)
	mux.HandleFunc("/api/episodes", apiEpisodes)
	mux.HandleFunc("/api/goals", apiGoals)
	mux.HandleFunc("/api/reflections", apiReflections)
	mux.HandleFunc("/api/actions", apiActions)
	mux.HandleFunc("/api/tools/audit", apiToolsAudit)
	mux.HandleFunc("/api/ledger", apiLedger)
	mux.HandleFunc("/api/interests", apiInterests)
	mux.HandleFunc("/api/config", apiConfig)
	mux.HandleFunc("/api/skills", apiSkills)
	mux.HandleFunc("/api/skills/load", apiSkillLoad)
	mux.HandleFunc("/api/skills/approve", apiSkillApprove)
	mux.HandleFunc("/api/skills/reject", apiSkillReject)
	mux.HandleFunc("/api/config/auto-approve-deps", apiAutoApproveDeps)
	mux.HandleFunc("/api/stream", apiStream)
	mux.HandleFunc("/api/external-request", apiExternalRequest)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "err", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	return srv
}

// -------- API handlers --------

func apiState(w http.ResponseWriter, r *http.Request) {
	ls, ms := state.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{"life": ls, "mental": ms})
}

func apiLifecycle(w http.ResponseWriter, r *http.Request) {
	cur, _, err := storage.LoadLifecycleState(lifeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": string(cur)})
}

func apiGenome(w http.ResponseWriter, r *http.Request) {
	g, err := storage.LoadGenome()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func apiValues(w http.ResponseWriter, r *http.Request) {
	v, err := storage.LoadValues(lifeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func apiEpisodes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := intParam(r, "limit", 50, 1, 500)
	offset := intParam(r, "offset", 0, 0, 100000)
	eps, err := storage.ListEpisodes(lifeID, q, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, eps)
}

func apiGoals(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := intParam(r, "limit", 50, 1, 500)
	gs, err := storage.ListGoals(lifeID, status, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, gs)
}

func apiReflections(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 50, 1, 500)
	rs, err := storage.ListReflections(lifeID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rs)
}

func apiActions(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 50, 1, 500)
	xs, err := storage.ListActionLog(lifeID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, xs)
}

func apiToolsAudit(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 50, 1, 500)
	xs, err := storage.ListToolAudit(lifeID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, xs)
}

func apiLedger(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	limit := intParam(r, "limit", 100, 1, 1000)
	xs, err := storage.ListLedger(lifeID, resource, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, xs)
}

func apiInterests(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 30, 1, 200)
	xs, err := storage.ListAllInterestSeeds(lifeID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, xs)
}

func apiConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"llm": map[string]any{
			"base_url":    os.Getenv("LLM_BASE_URL"),
			"model":       os.Getenv("LLM_MODEL"),
			"temperature": os.Getenv("LLM_TEMPERATURE"),
			"api_key":     maskSecret(os.Getenv("LLM_API_KEY")),
		},
		"feishu": map[string]any{
			"app_id":     os.Getenv("FEISHU_APP_ID"),
			"app_secret": maskSecret(os.Getenv("FEISHU_APP_SECRET")),
		},
		"skill_auto_approve_deps": storage.GetConfigBool("skill_auto_approve_deps", false),
	})
}

// -------- skill handlers (D.2 / D.3) --------

func apiSkills(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 50, 1, 200)
	xs, err := storage.ListSkillInstances(lifeID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, xs)
}

func apiSkillLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	inst, err := skill.Load(body.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, inst)
}

func apiSkillApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	id := skillIDParam(r)
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if err := skill.ApproveDeps(id, "user_approve"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	inst, _ := storage.GetSkillInstance(id)
	writeJSON(w, http.StatusOK, inst)
}

func apiSkillReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	id := skillIDParam(r)
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if err := skill.RejectDeps(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func apiAutoApproveDeps(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := storage.SetConfigBool("skill_auto_approve_deps", body.Value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		skill.SetAutoApprove(body.Value)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"skill_auto_approve_deps": storage.GetConfigBool("skill_auto_approve_deps", false),
	})
}

// skillIDParam 从 query ?id= 或 JSON body {id} 取 skill id。
func skillIDParam(r *http.Request) string {
	if id := r.URL.Query().Get("id"); id != "" {
		return id
	}
	var body struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.ID
}

func apiExternalRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		From    string `json:"from"`
		Channel string `json:"channel"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Channel == "" {
		body.Channel = "cli"
	}
	req := perception.ExternalRequest{
		ID:         fmt.Sprintf("ext-%d", time.Now().UnixNano()),
		Channel:    body.Channel,
		From:       body.From,
		Content:    body.Content,
		ReceivedAt: time.Now(),
	}
	// 慎思感知 + 反射即时回应
	perception.Inject(req)
	reflex.Handle(reflex.IncomingRequest{
		Channel: body.Channel,
		From:    body.From,
		Content: body.Content,
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"id": req.ID, "queued_at": req.ReceivedAt})
}

// -------- helpers --------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func intParam(r *http.Request, key string, def, min, max int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

// spaHandler 把 SvelteKit build 暴露在 /；未命中文件回退 index.html（SPA fallback）。
func spaHandler(web fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		f, err := web.Open(path)
		if err != nil {
			// SPA fallback
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		_ = f.Close()
		fileServer.ServeHTTP(w, r)
	})
}
