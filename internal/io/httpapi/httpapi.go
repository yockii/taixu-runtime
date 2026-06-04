// Package httpapi 观察 API（Phase 0.2 最小；Phase 0.4 + SvelteKit 扩充）。
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"mindverse/internal/runtime/perception"
	"mindverse/internal/runtime/state"
)

// Start 启动 HTTP 服务（非阻塞 + ctx 取消时 shutdown）。
func Start(ctx context.Context, addr string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		ls, ms := state.Snapshot()
		writeJSON(w, http.StatusOK, map[string]any{"life": ls, "mental": ms})
	})

	mux.HandleFunc("/api/external-request", func(w http.ResponseWriter, r *http.Request) {
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
		perception.Inject(req)
		writeJSON(w, http.StatusAccepted, map[string]any{"id": req.ID, "queued_at": req.ReceivedAt})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
