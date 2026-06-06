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
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"mindverse/internal/io/embed"
	"mindverse/internal/lifepack"
	"mindverse/internal/runtime/memory"
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
	mux.HandleFunc("/api/skills/rescan", apiSkillRescan)
	mux.HandleFunc("/api/skills/approve", apiSkillApprove)
	mux.HandleFunc("/api/skills/reject", apiSkillReject)
	mux.HandleFunc("/api/contacts", apiContacts)
	mux.HandleFunc("/api/config/auto-approve-deps", apiAutoApproveDeps)
	mux.HandleFunc("/api/config/proactive-im", apiProactiveIM)
	mux.HandleFunc("/api/config/quiet", apiQuietHours)
	mux.HandleFunc("/api/dialogue", apiDialogue)
	mux.HandleFunc("/api/stream", apiStream)
	mux.HandleFunc("/api/external-request", apiExternalRequest)
	mux.HandleFunc("/api/embed/backfill", apiEmbedBackfill)
	mux.HandleFunc("/api/export", apiExport)

	accessToken = strings.TrimSpace(os.Getenv("MINDVERSE_ACCESS_TOKEN"))
	if accessToken != "" {
		slog.Info("http access token enabled — write/interactive ops require X-Mindverse-Token")
	}

	srv := &http.Server{Addr: addr, Handler: withAuth(mux), ReadHeaderTimeout: 5 * time.Second}
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

// accessToken 来自 MINDVERSE_ACCESS_TOKEN。非空时，所有写/交互操作需带匹配 token。
// 空（默认）则不鉴权——localhost dogfooding 直接用。
var accessToken string

// withAuth 是写操作鉴权中间件（用户 2026-06-05 提出：防生命体被暴露到公网后被陌生人交互）。
//
// 策略（方法级，面向未来）：token 已设时，/api/ 下的**变更类方法**（POST/PUT/PATCH/DELETE）
// 必须带匹配的 X-Mindverse-Token。读操作（GET/HEAD，含 SSE /api/stream）与静态资源永远开放——
// 暴露面板看看无妨，但注入消息 / 改 dangerous-skip / 批准装依赖等必须授权。
func withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if accessToken != "" && strings.HasPrefix(r.URL.Path, "/api/") &&
			(isMutating(r.Method) || isProtectedRead(r)) {
			got := r.Header.Get("X-Mindverse-Token")
			if subtle.ConstantTimeCompare([]byte(got), []byte(accessToken)) != 1 {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"ok": false, "err": "unauthorized: missing or invalid access token",
				})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// isProtectedRead 标记含**用户隐私**的读端点（R87 补充：对话内容是用户与生命体的私密交流）。
// 即便 token 已设，统计/状态类读仍开放（看看数字生命无妨），但对话不行。
//
//	/api/actions?view=action     生命体自主行动 → 开放
//	/api/actions?view=dialogue   对话（含用户原话）→ 需令牌
//	/api/actions（无 view，含全部 kind = 含对话）→ 需令牌
func isProtectedRead(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.URL.Path == "/api/actions" {
		return r.URL.Query().Get("view") != "action"
	}
	if r.URL.Path == "/api/dialogue" { // 完整对话（用户原话 + 生命体回复）= 隐私
		return true
	}
	return false
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
	// view=dialogue → 对外言说（reflex/reflex_canned）；view=action → 内在作为（deliberate）；
	// 空 → 全部。分流让「说的」与「做的」分开展示（二者可背离）。
	var kinds []string
	switch r.URL.Query().Get("view") {
	case "dialogue":
		kinds = []string{storage.ActionKindReflex, storage.ActionKindReflexCanned}
	case "action":
		kinds = []string{storage.ActionKindDeliberate}
	}
	xs, err := storage.ListActionLogByKinds(lifeID, kinds, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, xs)
}

// apiDialogue 返回完整对话（用户原话 + 生命体回复，按时间正序），供对话面板区分双方。
// 之前对话面板只取 action_log 的 reflex 行（仅生命体单边）→ 看不出谁在说话（"你我不分"）。
func apiDialogue(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 30, 1, 200)
	turns, err := storage.RecentDialogueTurns(lifeID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, turns)
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
	// 环境信息（LLM 端点 / 飞书 app_id 等）属配置隐私：开了鉴权且未授权时不返回，
	// 只给 auth_required 让前端弹令牌输入（用户 2026-06-05；亦避免公网/截图泄漏）。
	resp := map[string]any{"auth_required": accessToken != ""}
	if authed(r) {
		resp["llm"] = map[string]any{
			"base_url":    os.Getenv("LLM_BASE_URL"),
			"model":       os.Getenv("LLM_MODEL"),
			"temperature": os.Getenv("LLM_TEMPERATURE"),
			"api_key":     maskSecret(os.Getenv("LLM_API_KEY")),
		}
		resp["feishu"] = map[string]any{
			"app_id":     os.Getenv("FEISHU_APP_ID"),
			"app_secret": maskSecret(os.Getenv("FEISHU_APP_SECRET")),
		}
		resp["skill_auto_approve_deps"] = storage.GetConfigBool("skill_auto_approve_deps", false)
		resp["proactive_im"] = storage.GetConfigBool("proactive_im", false)
		resp["proactive_quiet"] = map[string]any{
			"enabled":       storage.GetConfigBool("proactive_quiet_enabled", false),
			"start":         storage.GetConfigInt("proactive_quiet_start", 23),
			"end":           storage.GetConfigInt("proactive_quiet_end", 8),
			"tz_offset_min": storage.GetConfigInt("proactive_tz_offset_min", 0),
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// authed 请求是否通过鉴权（未设令牌则恒真；设了则需 header 匹配）。
func authed(r *http.Request) bool {
	if accessToken == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Mindverse-Token")), []byte(accessToken)) == 1
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

func apiSkillRescan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	n, err := skill.ScanDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"loaded": n})
}

func apiContacts(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 50, 1, 200)
	xs, err := storage.ListContacts(lifeID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, xs)
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

func apiProactiveIM(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct {
			Value bool `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := storage.SetConfigBool("proactive_im", body.Value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"proactive_im": storage.GetConfigBool("proactive_im", false),
	})
}

// apiQuietHours 读/写主动消息静默时段（R92）。POST {enabled,start,end,tz_offset_min}。
func apiQuietHours(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct {
			Enabled     bool `json:"enabled"`
			Start       int  `json:"start"`
			End         int  `json:"end"`
			TzOffsetMin int  `json:"tz_offset_min"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = storage.SetConfigBool("proactive_quiet_enabled", body.Enabled)
		_ = storage.SetConfigInt("proactive_quiet_start", clampHour(body.Start))
		_ = storage.SetConfigInt("proactive_quiet_end", clampHour(body.End))
		_ = storage.SetConfigInt("proactive_tz_offset_min", clampOffset(body.TzOffsetMin))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":       storage.GetConfigBool("proactive_quiet_enabled", false),
		"start":         storage.GetConfigInt("proactive_quiet_start", 23),
		"end":           storage.GetConfigInt("proactive_quiet_end", 8),
		"tz_offset_min": storage.GetConfigInt("proactive_tz_offset_min", 0),
	})
}

func clampHour(h int) int {
	if h < 0 {
		return 0
	}
	if h > 23 {
		return 23
	}
	return h
}

func clampOffset(m int) int {
	if m < -720 {
		return -720
	}
	if m > 840 {
		return 840
	}
	return m
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

// apiEmbedBackfill 手动触发历史记忆向量回填（给锁定生命的旧记忆补向量）。
// 有界 + best-effort + 可重入：嵌入服务不可用 → 跳过返回 0；?max= 控每层上限（默认 256）。
// 写操作，token 已设时需带 X-Mindverse-Token（withAuth 拦截）。
func apiEmbedBackfill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if !embed.Configured() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "filled": 0, "note": "embed not configured; skipped"})
		return
	}
	maxPerLayer := 256
	if v := r.URL.Query().Get("max"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxPerLayer = n
		}
	}
	n := memory.BackfillEmbeddings(r.Context(), maxPerLayer)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "filled": n})
}

func apiExternalRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		From     string `json:"from"`
		Channel  string `json:"channel"`
		ChatType string `json:"chat_type"` // "direct"（默认）/ "group"
		Content  string `json:"content"`
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
		Channel:  body.Channel,
		ChatType: body.ChatType,
		From:     body.From,
		Content:  body.Content,
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"id": req.ID, "queued_at": req.ReceivedAt})
}

// apiExport 导出加密生命包（.mvlife）。POST {passphrase} → 流式下载。
//
// 鉴权：POST = isMutating，token 已设时自动需令牌（withAuth）。包含整库（记忆/对话）+ workspace，
// 是生命体的全部隐私 + 身份，绝不可裸奔。一致性：VACUUM INTO 取快照再打包（无需停写）。
// 口令是唯一钥匙、丢了不可恢复——前端须在导出时显著提示用户记牢（R17）。
func apiExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Passphrase string `json:"passphrase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(body.Passphrase) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "err": "passphrase too short (min 8 chars)"})
		return
	}

	// 一致快照到临时目录（VACUUM INTO 要求目标不存在）。
	tmpDir, err := os.MkdirTemp("", "mvexport-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "err": "temp dir: " + err.Error()})
		return
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	snap := filepath.Join(tmpDir, "snap.db")
	if err := storage.SnapshotInto(snap); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "err": "snapshot: " + err.Error()})
		return
	}

	man := lifepack.Manifest{
		AppVersion:    getenvOr("MINDVERSE_VERSION", "dev"),
		LifeID:        lifeID,
		ExportedAt:    time.Now().Unix(),
		GenomeVersion: "",
	}
	if g, err := storage.LoadGenome(); err == nil && g != nil {
		man.GenomeVersion = g.GenomeVersion
	}
	if v, ok, _ := storage.GetMeta("version"); ok {
		man.SchemaVersion = v
	}
	ws := getenvOr("MINDVERSE_WORKSPACE", "/workspace")

	// 先打到内存再下发：包失败可干净返回 500（DB 为 MB 级，可接受）。
	var buf bytes.Buffer
	if err := lifepack.Export(&buf, snap, ws, man, body.Passphrase); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "err": "export: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="mindverse-%s.mvlife"`, lifeID))
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
	slog.Info("life exported", "bytes", buf.Len(), "life", lifeID)
}

// getenvOr 读环境变量，空则回退默认。
func getenvOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
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
