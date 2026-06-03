// Mindverse Runtime · Phase 0.1 骨架入口。
//
// 流程：
//  1. 打开 SQLite + 应用 schema 迁移
//  2. 若无 Genome：Genesis 出生流程
//  3. 进入 Embryonic -> Active 转换
//  4. dummy 循环：每分钟一次 tick 打印日志（Phase 0.2 替换为完整 9 步循环）
//  5. 收到 SIGINT/SIGTERM：优雅关停（Active -> Dormant）
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"mindverse/internal/eventbus"
	"mindverse/internal/genesis"
	"mindverse/internal/lifecyclemanager"
	"mindverse/internal/memoryengine"

	"mindverse/internal/core"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	dbPath := envOr("MINDVERSE_DB", filepath.Join(dataDir(), "mindverse.db"))
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		slog.Error("ensure data dir", "err", err)
		os.Exit(1)
	}

	slog.Info("runtime starting", "db", dbPath, "phase", "0.1")

	store, err := memoryengine.Open(dbPath)
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			slog.Error("close store", "err", err)
		}
	}()

	bus := eventbus.New()
	bus.Subscribe(eventbus.LifecycleTransitioned{}, func(e eventbus.Event) {
		ev := e.(eventbus.LifecycleTransitioned)
		slog.Info("lifecycle", "from", ev.FromState, "to", ev.ToState, "reason", ev.Reason, "life", ev.LifeID)
	})

	lifeID, err := loadOrBear(store, bus)
	if err != nil {
		slog.Error("genesis", "err", err)
		os.Exit(1)
	}

	lm := lifecyclemanager.New(store, bus)
	cur, err := lm.Current(lifeID)
	if err != nil {
		slog.Error("load lifecycle", "err", err)
		os.Exit(1)
	}
	if cur == core.StateEmbryonic {
		if err := lm.Transition(lifeID, core.StateActive, "boot"); err != nil {
			slog.Error("activate", "err", err)
			os.Exit(1)
		}
	} else if cur == core.StateDormant {
		if err := lm.Transition(lifeID, core.StateActive, "wake"); err != nil {
			slog.Error("wake", "err", err)
			os.Exit(1)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := dummyLoop(ctx, store, lifeID, bus); err != nil {
		slog.Error("loop", "err", err)
	}

	if err := lm.Transition(lifeID, core.StateDormant, "shutdown"); err != nil {
		slog.Error("dormant", "err", err)
	}

	slog.Info("runtime stopped")
}

// loadOrBear 加载现有 Genome；不存在则触发 Genesis。
func loadOrBear(store *memoryengine.Store, bus *eventbus.Bus) (string, error) {
	g, err := store.LoadGenome()
	if err == nil {
		slog.Info("genome loaded",
			"life_id", g.LifeID,
			"curiosity", g.Curiosity,
			"sociability", g.Sociability,
			"creativity", g.Creativity,
			"persistence", g.Persistence,
			"risk_taking", g.RiskTaking,
			"empathy", g.Empathy,
			"born_at", g.BornAt,
		)
		return g.LifeID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	slog.Info("no genome; starting genesis")
	lifeID, err := genesis.Bear(store)
	if err != nil {
		return "", err
	}
	bus.Publish(eventbus.GenesisCompleted{LifeID: lifeID})
	slog.Info("genesis completed", "life_id", lifeID)
	return lifeID, nil
}

// dummyLoop Phase 0.1 占位循环：每分钟一次 tick。
// Phase 0.2 替换为 9 步完整循环 + 自适应节拍。
func dummyLoop(ctx context.Context, store *memoryengine.Store, lifeID string, bus *eventbus.Bus) error {
	var cycleID int64
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()

	tick := func() {
		cycleID++
		now := time.Now().Unix()
		bus.Publish(eventbus.TickStarted{LifeID: lifeID, CycleID: cycleID})
		if err := store.AppendRawTrail(lifeID, cycleID, "dummy.tick", `{"note":"phase 0.1 placeholder"}`, now); err != nil {
			slog.Warn("raw_trail append", "err", err)
		}
		slog.Info("tick", "cycle", cycleID)
		bus.Publish(eventbus.TickFinished{LifeID: lifeID, CycleID: cycleID})
	}

	tick()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			tick()
		}
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func dataDir() string {
	if v := os.Getenv("MINDVERSE_DATA"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "mindverse", "data")
	}
	return "./data"
}
