// Mindverse Runtime · Phase 0.2 入口（9 步循环 + 自适应节拍）。
//
// 9 步：Perceive → UpdateState → RecordMemory → ConsiderReflect → CollectGoals → Arbitrate
//        → Plan → Act → Feedback（Plan/Act/Feedback 三段在 ActionExecutor.Execute 内合）
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

	"strconv"
	"time"

	"mindverse/internal/actionexecutor"
	"mindverse/internal/core"
	"mindverse/internal/eventbus"
	"mindverse/internal/genesis"
	"mindverse/internal/goalarbitrator"
	"mindverse/internal/imadapter"
	"mindverse/internal/lifecyclemanager"
	"mindverse/internal/llmadapter"
	"mindverse/internal/memoryengine"
	"mindverse/internal/perception"
	"mindverse/internal/reflectionengine"
	"mindverse/internal/resourceledger"
	"mindverse/internal/scheduler"
	"mindverse/internal/shared"
	"mindverse/internal/skillregistry/toolrunner"
	"mindverse/internal/statemanager"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	dbPath := envOr("MINDVERSE_DB", filepath.Join(dataDir(), "mindverse.db"))
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		slog.Error("ensure data dir", "err", err)
		os.Exit(1)
	}
	slog.Info("runtime starting", "db", dbPath, "phase", "0.2")

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
		slog.Info("lifecycle", "from", ev.FromState, "to", ev.ToState, "reason", ev.Reason)
	})

	genome, lifeID, err := loadOrBear(store, bus)
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

	sm, err := statemanager.New(store, bus, lifeID)
	if err != nil {
		slog.Error("statemanager init", "err", err)
		os.Exit(1)
	}
	mem, err := memoryengine.NewEngine(store, lifeID)
	if err != nil {
		slog.Error("memory engine init", "err", err)
		os.Exit(1)
	}
	ledger, err := resourceledger.New(store, sm, lifeID)
	if err != nil {
		slog.Error("ledger init", "err", err)
		os.Exit(1)
	}
	reflector := reflectionengine.New(store, mem, lifeID)
	arbitrator := goalarbitrator.New(store, lifeID)

	sandboxDir := envOr("MINDVERSE_SANDBOX", "/sandbox")
	tools := toolrunner.New(store, lifeID, sandboxDir)
	executor := actionexecutor.New(store, sm, tools, lifeID).WithBus(bus).WithLedger(ledger)
	if llm := buildLLM(); llm != nil {
		executor = executor.WithLLM(llm)
		slog.Info("llm wired", "model", os.Getenv("LLM_MODEL"))
	} else {
		slog.Warn("llm not configured; respond_to_user falls back to dummy")
	}
	perceiver := perception.New(sm)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	httpAddr := envOr("MINDVERSE_HTTP", ":3000")
	_ = startHTTP(ctx, httpAddr, perceiver, sm)
	slog.Info("http listening", "addr", httpAddr)

	// 接入飞书（可选；缺凭证则跳过）。
	if appID := os.Getenv("FEISHU_APP_ID"); appID != "" {
		if err := imadapter.Init(imadapter.Config{
			AppID:     appID,
			AppSecret: os.Getenv("FEISHU_APP_SECRET"),
		}, perceiver.Inject); err != nil {
			slog.Error("imadapter init", "err", err)
		} else {
			bus.Subscribe(actionexecutor.SpeechEvent{}, func(e eventbus.Event) {
				ev := e.(actionexecutor.SpeechEvent)
				// 只把飞书发起的 SpeechEvent 或已知 last 用户的回复发去飞书
				if ev.Channel != "" && ev.Channel != "feishu" {
					return
				}
				if ev.To == "" && imadapter.LastSenderOpenID() == "" {
					return
				}
				if err := imadapter.Send(ev.To, ev.Content); err != nil {
					slog.Error("feishu send", "err", err)
				}
			})
			go func() {
				slog.Info("feishu ws starting", "app_id", appID)
				if err := imadapter.Run(ctx); err != nil {
					slog.Error("feishu ws", "err", err)
				}
			}()
		}
	} else {
		slog.Info("feishu not configured; skip")
	}

	sched := scheduler.New(bus, store, sm, lifeID, func() core.LifecycleState {
		s, _ := lm.Current(lifeID)
		return s
	}, perceiver)
	if err := sched.Resume(); err != nil {
		slog.Error("scheduler resume", "err", err)
	}

	onTick := func(cycleID int64) {
		runCycle(cycleID, lifeID, genome, store, sm, mem, ledger, reflector, arbitrator, executor, perceiver, tools)
	}
	if err := sched.Run(ctx, onTick); err != nil {
		slog.Error("scheduler run", "err", err)
	}

	if err := lm.Transition(lifeID, core.StateDormant, "shutdown"); err != nil {
		slog.Error("dormant", "err", err)
	}
	slog.Info("runtime stopped")
}

// runCycle 9 步循环。
func runCycle(cycleID int64, lifeID string, genome core.Genome,
	store *memoryengine.Store, sm *statemanager.Manager, mem *memoryengine.Engine,
	ledger *resourceledger.Ledger, reflector *reflectionengine.Engine, arbitrator *goalarbitrator.Arbitrator,
	executor *actionexecutor.Executor, perceiver *perception.Perceiver, tools *toolrunner.Runner) {

	// 1. Perceive
	frame := perceiver.Perceive(cycleID)
	if err := mem.AppendEvent(cycleID, "cycle.start", map[string]any{
		"externals": len(frame.Externals),
		"energy":    frame.Life.Energy,
	}); err != nil {
		slog.Warn("append cycle.start", "err", err)
	}

	// 2. UpdateState（任何外部交互降 social_need、提 motivation；记录事件）
	for _, ext := range frame.Externals {
		_ = mem.AppendEvent(cycleID, "external.request", map[string]any{
			"id":      ext.ID,
			"channel": ext.Channel,
			"from":    ext.From,
			"content": ext.Content,
		})
		sndelta := -0.08
		motDelta := 0.04
		if err := sm.Apply(statemanager.Delta{
			SocialNeed: &sndelta, Motivation: &motDelta, Reason: "external.request",
		}); err != nil {
			slog.Warn("apply external delta", "err", err)
		}
	}

	// 3. RecordMemory（已通过 AppendEvent 写 raw_trail；这里追加工作记忆汇总）
	mem.PutWorking(cycleID, "perceive.summary", frame.SummaryLine())

	// 4. ConsiderReflect
	ls, ms := sm.Snapshot()
	if reflector.ShouldReflect(genome, ls, ms) {
		promoted, rid, err := reflector.Reflect("scheduler.cycle")
		if err != nil {
			slog.Warn("reflect", "err", err)
		} else {
			_ = mem.AppendEvent(cycleID, "reflect", map[string]any{"promoted": promoted, "id": rid})
			slog.Info("reflect", "promoted", promoted, "id", rid)
		}
	}

	// 5. CollectGoals
	drives := statemanager.DeriveDrives(genome, ls, ms)
	cands := arbitrator.CollectCandidates(frame, drives)

	// 6. Arbitrate
	values, err := store.LoadValues(lifeID)
	if err != nil {
		slog.Warn("load values", "err", err)
		values = &core.Values{LifeID: lifeID, Weights: map[string]float64{}}
	}
	ids, err := arbitrator.Arbitrate(cands, values, 3)
	if err != nil {
		slog.Warn("arbitrate", "err", err)
	}
	if len(ids) > 0 {
		_ = mem.AppendEvent(cycleID, "goals.enqueued", map[string]any{"ids": ids})
	}

	// 7-8-9. Plan / Act / Feedback — 最多取一个 pending goal 执行
	now := shared.SystemClock.UnixSec()
	g, err := store.NextPendingGoal(lifeID, now)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("next pending goal", "err", err)
	}
	if g != nil {
		res, err := executor.Execute(g, cycleID)
		if err != nil {
			slog.Warn("execute", "err", err, "goal", g.ID)
		}
		_ = mem.AppendEvent(cycleID, "action.done", map[string]any{
			"goal_id": g.ID,
			"success": res.Success,
			"action":  res.Action,
		})
		if res.Success {
			_ = mem.AppendEvent(cycleID, "tool.success", res.Action)
			_ = ledger.Earn(resourceledger.Knowledge, 0.01, "action.success", "goal", "")
		}
		_ = ledger.Spend(resourceledger.Energy, 0.02, "action.cost", "goal", "")
	}

	// 后台维护：考虑封段 + 抽取语义 + cap 重置
	if ep, err := mem.ConsiderSealEpisode(); err == nil && ep != nil {
		slog.Info("episode sealed", "id", ep.ID, "events_in_seg", ep.RawEndID-ep.RawStartID+1)
		_ = mem.AppendEvent(cycleID, "episode.sealed", map[string]any{"id": ep.ID})
	}
	if added, err := mem.ExtractSemantic(); err == nil && added > 0 {
		_ = mem.AppendEvent(cycleID, "semantic.candidates", map[string]any{"added": added})
	}
	if reset, err := ledger.MaybeResetEnergyDailyCap(); err == nil && reset {
		slog.Info("energy daily cap reset")
		_ = mem.AppendEvent(cycleID, "energy.cap_reset", nil)
	}

	mem.ResetWorking()
}

func loadOrBear(store *memoryengine.Store, bus *eventbus.Bus) (core.Genome, string, error) {
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
		)
		return *g, g.LifeID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return core.Genome{}, "", err
	}

	slog.Info("no genome; starting genesis")
	lifeID, err := genesis.Bear(store)
	if err != nil {
		return core.Genome{}, "", err
	}
	bus.Publish(eventbus.GenesisCompleted{LifeID: lifeID})
	g2, err := store.LoadGenome()
	if err != nil {
		return core.Genome{}, lifeID, err
	}
	slog.Info("genesis completed", "life_id", lifeID)
	return *g2, lifeID, nil
}

// buildLLM 从环境变量装配 LLMAdapter。
// 必备：LLM_BASE_URL / LLM_API_KEY / LLM_MODEL。
// 可选：LLM_TEMPERATURE（默认 0.7）。
func buildLLM() *llmadapter.Adapter {
	base := os.Getenv("LLM_BASE_URL")
	key := os.Getenv("LLM_API_KEY")
	model := os.Getenv("LLM_MODEL")
	if base == "" || key == "" || model == "" {
		return nil
	}
	temp := float32(0.7)
	if v := os.Getenv("LLM_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			temp = float32(f)
		}
	}
	return llmadapter.New(llmadapter.Config{
		BaseURL:     base,
		APIKey:      key,
		Model:       model,
		Temperature: temp,
		Timeout:     90 * time.Second,
	})
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
