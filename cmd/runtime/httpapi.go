package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"mindverse/internal/perception"
	"mindverse/internal/statemanager"
)

// startHTTP 启动观察 + 注入 API。Phase 0.2 仅暴露最小路由；Phase 0.4 SvelteKit 接入扩充。
func startHTTP(ctx context.Context, addr string, p *perception.Perceiver, sm *statemanager.Manager) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		ls, ms := sm.Snapshot()
		writeJSON(w, http.StatusOK, map[string]any{
			"life":   ls,
			"mental": ms,
		})
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
		p.Inject(req)
		writeJSON(w, http.StatusAccepted, map[string]any{"id": req.ID, "queued_at": req.ReceivedAt})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
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
