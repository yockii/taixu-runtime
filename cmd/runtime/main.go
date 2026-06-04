// Mindverse Runtime · 单例分层版入口。
//
// 启动顺序（依赖低 → 高）：
//   storage → bus（隐式）→ genesis(首次) → 加载 Genome
//   → lifecycle → state → ledger → memory → reflect → goal → action
//   → toolrunner → llm（可选）→ httpapi → lark（可选）
//   → scheduler.Run
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"mindverse/internal/bus"
	"mindverse/internal/core"
	"mindverse/internal/io/httpapi"
	"mindverse/internal/io/lark"
	"mindverse/internal/io/llm"
	"mindverse/internal/runtime/action"
	"mindverse/internal/runtime/reflex"
	"mindverse/internal/runtime/drives"
	"mindverse/internal/runtime/genesis"
	"mindverse/internal/runtime/goal"
	"mindverse/internal/runtime/ledger"
	"mindverse/internal/runtime/lifecycle"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/runtime/perception"
	"mindverse/internal/runtime/reflect"
	"mindverse/internal/runtime/scheduler"
	"mindverse/internal/runtime/state"
	"mindverse/internal/runtime/tools"
	"mindverse/internal/shared"
	"mindverse/internal/skill/toolrunner"
	"mindverse/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	dbPath := envOr("MINDVERSE_DB", filepath.Join(dataDir(), "mindverse.db"))
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		fatal("ensure data dir", err)
	}
	slog.Info("runtime starting", "db", dbPath, "phase", "0.4")

	// storage 单例：进程内仅 1 DB。
	if err := storage.Init(dbPath); err != nil {
		fatal("storage", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			slog.Error("close storage", "err", err)
		}
	}()

	bus.Subscribe(bus.LifecycleTransitioned{}, func(e bus.Event) {
		ev := e.(bus.LifecycleTransitioned)
		slog.Info("lifecycle", "from", ev.FromState, "to", ev.ToState, "reason", ev.Reason)
	})

	genome, lifeID, err := loadOrBear()
	if err != nil {
		fatal("genesis", err)
	}

	// runtime 子模块：均绑定该 lifeID 单例。
	mustInit("lifecycle", lifecycle.Init(lifeID))
	mustInit("state", state.Init(lifeID))
	mustInit("ledger", ledger.Init(lifeID))
	mustInit("memory", memory.Init(lifeID))
	mustInit("reflect", reflect.Init(lifeID))
	mustInit("goal", goal.Init(lifeID))
	mustInit("action", action.Init(lifeID))
	mustInit("scheduler", scheduler.Init(lifeID))
	mustInit("tools", tools.Init())
	mustInit("reflex", reflex.Init(lifeID))

	mustInit("toolrunner", toolrunner.Init(lifeID, envOr("MINDVERSE_SANDBOX", "/sandbox")))

	cur, err := lifecycle.Current()
	if err != nil {
		fatal("load lifecycle", err)
	}
	switch cur {
	case core.StateEmbryonic:
		if err := lifecycle.Transition(core.StateActive, "boot"); err != nil {
			fatal("activate", err)
		}
	case core.StateDormant:
		if err := lifecycle.Transition(core.StateActive, "wake"); err != nil {
			fatal("wake", err)
		}
	}

	if err := buildLLM(); err != nil {
		slog.Warn("llm not configured; respond_to_user falls back to dummy", "err", err)
	} else {
		slog.Info("llm wired", "model", os.Getenv("LLM_MODEL"))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mustInit("httpapi", httpapi.Init(lifeID, webStaticFS()))
	httpAddr := envOr("MINDVERSE_HTTP", ":3000")
	_ = httpapi.Start(ctx, httpAddr)
	slog.Info("http listening", "addr", httpAddr)

	wireLark(ctx)

	if err := scheduler.Run(ctx, func(cycleID int64) {
		runCycle(cycleID, lifeID, genome)
	}); err != nil {
		slog.Error("scheduler run", "err", err)
	}

	if err := lifecycle.Transition(core.StateDormant, "shutdown"); err != nil {
		slog.Error("dormant", "err", err)
	}
	slog.Info("runtime stopped")
}

// runCycle 9 步循环。
func runCycle(cycleID int64, lifeID string, genome core.Genome) {
	// 1. Perceive
	frame := perception.Perceive(cycleID)
	_ = memory.AppendEvent(cycleID, "cycle.start", map[string]any{
		"externals": len(frame.Externals),
		"energy":    frame.Life.Energy,
	})

	// 2. UpdateState（外部交互降 social_need + 提 motivation）
	for _, ext := range frame.Externals {
		_ = memory.AppendEvent(cycleID, "external.request", map[string]any{
			"id":      ext.ID,
			"channel": ext.Channel,
			"from":    ext.From,
			"content": ext.Content,
		})
		sn := -0.08
		mot := 0.04
		_ = state.Apply(state.Delta{SocialNeed: &sn, Motivation: &mot, Reason: "external.request"})
	}

	// 3. RecordMemory（工作记忆汇总）
	memory.PutWorking(cycleID, "perceive.summary", frame.SummaryLine())

	// 4. ConsiderReflect
	ls, ms := state.Snapshot()
	if reflect.ShouldReflect(genome, ls, ms) {
		promoted, rid, err := reflect.Run("scheduler.cycle")
		if err != nil {
			slog.Warn("reflect", "err", err)
		} else {
			_ = memory.AppendEvent(cycleID, "reflect", map[string]any{"promoted": promoted, "id": rid})
			slog.Info("reflect", "promoted", promoted, "id", rid)
		}
	}

	// 5. CollectGoals
	ds := drives.Derive(genome, ls, ms, lifeID)
	cands := goal.CollectCandidates(frame, ds)

	// 6. Arbitrate
	values, err := storage.LoadValues(lifeID)
	if err != nil {
		slog.Warn("load values", "err", err)
		values = &core.Values{LifeID: lifeID, Weights: map[string]float64{}}
	}
	ids, err := goal.Arbitrate(cands, values, 3)
	if err != nil {
		slog.Warn("arbitrate", "err", err)
	}
	if len(ids) > 0 {
		_ = memory.AppendEvent(cycleID, "goals.enqueued", map[string]any{"ids": ids})
	}

	// 7-8-9. Plan / Act / Feedback
	now := shared.SystemClock.UnixSec()
	g, err := storage.NextPendingGoal(lifeID, now)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("next pending goal", "err", err)
	}
	if g != nil {
		res, err := action.Execute(g, cycleID)
		if err != nil {
			slog.Warn("execute", "err", err, "goal", g.ID)
		}
		_ = memory.AppendEvent(cycleID, "action.done", map[string]any{
			"goal_id": g.ID,
			"success": res.Success,
			"action":  res.Action,
		})
		if res.Success {
			_ = memory.AppendEvent(cycleID, "tool.success", res.Action)
			_ = ledger.Earn(ledger.Knowledge, 0.01, "action.success", "goal", "")
		}
		_ = ledger.Spend(ledger.Energy, 0.02, "action.cost", "goal", "")
	}

	// 后台维护
	if ep, err := memory.ConsiderSealEpisode(); err == nil && ep != nil {
		slog.Info("episode sealed", "id", ep.ID, "events_in_seg", ep.RawEndID-ep.RawStartID+1)
		_ = memory.AppendEvent(cycleID, "episode.sealed", map[string]any{"id": ep.ID})
	}
	if added, err := memory.ExtractSemantic(); err == nil && added > 0 {
		_ = memory.AppendEvent(cycleID, "semantic.candidates", map[string]any{"added": added})
	}
	if reset, err := ledger.MaybeResetEnergyDailyCap(); err == nil && reset {
		slog.Info("energy daily cap reset")
		_ = memory.AppendEvent(cycleID, "energy.cap_reset", nil)
	}

	memory.ResetWorking()
}

func loadOrBear() (core.Genome, string, error) {
	g, err := storage.LoadGenome()
	if err == nil {
		slog.Info("genome loaded",
			"life_id", g.LifeID,
			"curiosity", g.Curiosity,
			"sociability", g.Sociability,
			"creativity", g.Creativity,
			"persistence", g.Persistence,
			"risk_taking", g.RiskTaking,
			"empathy", g.Empathy,
		)
		return *g, g.LifeID, nil
	}
	if !errors.Is(err, storage.ErrNoRows) {
		return core.Genome{}, "", err
	}
	slog.Info("no genome; starting genesis")
	lifeID, err := genesis.Bear()
	if err != nil {
		return core.Genome{}, "", err
	}
	bus.Publish(bus.GenesisCompleted{LifeID: lifeID})
	g2, err := storage.LoadGenome()
	if err != nil {
		return core.Genome{}, lifeID, err
	}
	slog.Info("genesis completed", "life_id", lifeID)
	return *g2, lifeID, nil
}

func buildLLM() error {
	base := os.Getenv("LLM_BASE_URL")
	key := os.Getenv("LLM_API_KEY")
	model := os.Getenv("LLM_MODEL")
	if base == "" || key == "" || model == "" {
		return errors.New("missing LLM_BASE_URL / LLM_API_KEY / LLM_MODEL")
	}
	temp := float32(0.7)
	if v := os.Getenv("LLM_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			temp = float32(f)
		}
	}
	return llm.Init(llm.Config{
		BaseURL:     base,
		APIKey:      key,
		Model:       model,
		Temperature: temp,
		Timeout:     90 * time.Second,
	})
}

func wireLark(ctx context.Context) {
	appID := os.Getenv("FEISHU_APP_ID")
	if appID == "" {
		slog.Info("feishu not configured; skip")
		return
	}
	if err := lark.Init(lark.Config{
		AppID:     appID,
		AppSecret: os.Getenv("FEISHU_APP_SECRET"),
	}); err != nil {
		slog.Error("lark init", "err", err)
		return
	}
	// 反射对话每一轮的 content → 单独发飞书消息（自然分段）
	bus.Subscribe(reflex.ReplyEvent{}, func(e bus.Event) {
		ev := e.(reflex.ReplyEvent)
		if ev.Channel != "" && ev.Channel != "feishu" {
			return
		}
		if ev.To == "" && lark.LastSenderOpenID() == "" {
			return
		}
		if err := lark.Send(ev.To, ev.Content); err != nil {
			slog.Error("feishu send", "err", err)
		}
	})
	go func() {
		slog.Info("feishu ws starting", "app_id", appID)
		if err := lark.Run(ctx); err != nil {
			slog.Error("feishu ws", "err", err)
		}
	}()
}

func mustInit(name string, err error) {
	if err != nil {
		fatal(name+" init", err)
	}
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
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
