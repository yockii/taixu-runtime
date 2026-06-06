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
	"mindverse/internal/io/egress"
	"mindverse/internal/io/httpapi"
	"mindverse/internal/io/lark"
	"mindverse/internal/io/llm"
	"mindverse/internal/lifepack"
	"mindverse/internal/runtime/action"
	"mindverse/internal/runtime/reflex"
	"mindverse/internal/runtime/drives"
	"mindverse/internal/runtime/genesis"
	"mindverse/internal/runtime/goal"
	"mindverse/internal/runtime/idle"
	"mindverse/internal/runtime/ledger"
	"mindverse/internal/runtime/lifecycle"
	"mindverse/internal/runtime/memory"
	"mindverse/internal/runtime/perception"
	"mindverse/internal/runtime/reflect"
	"mindverse/internal/runtime/scheduler"
	"mindverse/internal/runtime/skill"
	"mindverse/internal/runtime/state"
	"mindverse/internal/runtime/tools"
	"mindverse/internal/runtime/tools/builtin"
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

	// 导入：若 MINDVERSE_IMPORT 指向一个 .mvlife 且当前库为空，则在打开库前还原。
	// 只往空卷导入——绝不覆盖活体（见 maybeImportLife）。重启幂等：导入后库已存在 → 跳过、正常启动。
	maybeImportLife(dbPath)

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
	mustInit("action", action.Init(lifeID, genome))
	mustInit("scheduler", scheduler.Init(lifeID))
	mustInit("idle", idle.Init(lifeID))
	mustInit("tools", tools.Init())
	mustInit("reflex", reflex.Init(lifeID, genome))

	mustInit("toolrunner", toolrunner.Init(lifeID, envOr("MINDVERSE_SANDBOX", "/workspace/sandbox")))
	mustInit("skill", skill.Init(lifeID, envOr("MINDVERSE_SKILLS", "/workspace/skills"),
		storage.GetConfigBool("skill_auto_approve_deps", false)))
	mustInit("tools.builtin", builtin.Register())
	if n, err := skill.ScanDir(); err != nil {
		slog.Warn("skill scan dir", "err", err)
	} else if n > 0 {
		slog.Info("skills loaded from dir", "count", n)
	}

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

	// 出站渠道路由：注册 web 渠道 egress（网页请求者经 SSE 收回复），并启动中央分发器
	// （订阅 ReplyEvent / ApprovalNeededEvent → 按 channel 路由到对应 egress，哪来回哪去）。
	httpapi.RegisterEgress()
	egress.StartDispatcher()

	wireLark(ctx, lifeID)

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

// RestEnergyThreshold 能量低于此值 → 本轮休息回血，不慎思执行目标（R86）。
// 慎思烧 LLM/energy，低能量硬磕会油尽灯枯；累了就歇，让能量恢复后再做。
const RestEnergyThreshold = 0.20

// BehaviorCooldownBaseSec 反思 / 新目标产生的最小间隔基线（R88 降频）。
// 实际间隔 = 基线 + (cycleID % 900)，即 15-30min 抖动，避免每 60s cycle 就反思/派新目标。
const BehaviorCooldownBaseSec = 900

// behaviorDue 距上次该行为是否已过冷却（schema_meta 记时间戳；jitter 用 cycleID 抖动到 30min）。
func behaviorDue(key string, now, jitter int64) bool {
	v, ok, err := storage.GetMeta(key)
	if err != nil || !ok {
		return true
	}
	last, _ := strconv.ParseInt(v, 10, 64)
	interval := BehaviorCooldownBaseSec + (jitter % 900)
	return now-last >= interval
}

func markBehavior(key string, now int64) {
	_ = storage.SetMeta(key, strconv.FormatInt(now, 10))
}

// 维护参数：raw_trail 在两消费游标之前再留 RawTrailKeepBuffer 条余量；working_memory 仅留最近若干条。
const (
	MaintenanceIntervalSec = 24 * 3600
	RawTrailKeepBuffer     = 500
	WorkingMemoryKeep      = 500
)

// maintenanceDue 距上次日度维护是否已过 24h（首次/重启即跑一次，剪枝幂等且廉价）。
func maintenanceDue(lifeID string, now int64) bool {
	v, ok, err := storage.GetMeta("maintenance_last:" + lifeID)
	if err != nil || !ok {
		return true
	}
	last, _ := strconv.ParseInt(v, 10, 64)
	return now-last >= MaintenanceIntervalSec
}

// runMaintenance 剪枝已消费的旧 raw_trail + 旧 working_memory（引擎侧，不动 episode/语义/反思等长期记忆）。
func runMaintenance(lifeID string) {
	if n, err := memory.PruneConsumedRawTrail(RawTrailKeepBuffer); err != nil {
		slog.Warn("maintenance prune raw_trail", "err", err)
	} else if n > 0 {
		slog.Info("maintenance pruned raw_trail", "deleted", n)
	}
	if n, err := storage.PruneWorkingMemoryKeepRecent(lifeID, WorkingMemoryKeep); err != nil {
		slog.Warn("maintenance prune working_memory", "err", err)
	} else if n > 0 {
		slog.Info("maintenance pruned working_memory", "deleted", n)
	}
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

	// 4. ConsiderReflect（降频 R88：反思按 15-30min 间隔，不再每 cycle）
	ls, ms := state.Snapshot()
	cycleNow := shared.SystemClock.UnixSec()
	if behaviorDue("reflect_last:"+lifeID, cycleNow, cycleID) && reflect.ShouldReflect(genome, ls, ms) {
		promoted, rid, err := reflect.Run("scheduler.cycle")
		if err != nil {
			slog.Warn("reflect", "err", err)
		} else {
			markBehavior("reflect_last:"+lifeID, cycleNow)
			_ = memory.AppendEvent(cycleID, "reflect", map[string]any{"promoted": promoted, "id": rid})
			slog.Info("reflect", "promoted", promoted, "id", rid)
		}
	}

	// 5-6. CollectGoals + Arbitrate（降频 R88：新目标按 15-30min 产生一次；
	// 已在队列的 pending 目标不受影响、照常执行——这里只节流"产生新目标"的频率）。
	if behaviorDue("goalgen_last:"+lifeID, cycleNow, cycleID) {
		ds := drives.Derive(genome, ls, ms, lifeID)
		cands := goal.CollectCandidates(frame, ds)
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
			markBehavior("goalgen_last:"+lifeID, cycleNow)
			_ = memory.AppendEvent(cycleID, "goals.enqueued", map[string]any{"ids": ids})
		}
	}

	// 7-8-9. Plan / Act / Feedback
	now := shared.SystemClock.UnixSec()
	g, err := storage.NextPendingGoal(lifeID, now)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("next pending goal", "err", err)
	}
	switch {
	case g != nil && frame.Life.Energy >= RestEnergyThreshold:
		// 有目标且体力够 → 慎思执行。
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
		idle.Reset() // 真的在做事 → 清零无聊
	case g != nil:
		// 有目标但太累 → 休息回血，目标留到能量恢复再做（R86）。
		// 关键：慎思烧 LLM=烧能量，低能量还硬磕会油尽灯枯。累了就该歇，让能量自然恢复，
		// 而非靠"放缓目标产生"治标。目标仍 pending，不消耗，醒来接着干。
		en, str := 0.05, -0.03
		_ = state.Apply(state.Delta{Energy: &en, Stress: &str, Reason: "cycle.rest"})
		_ = memory.AppendEvent(cycleID, "cycle.rest", map[string]any{
			"energy": frame.Life.Energy, "pending_goal": g.ID,
		})
		slog.Info("resting (too tired to deliberate)", "energy", frame.Life.Energy, "pending_goal", g.ID)
	default:
		// 无具体目标 → 发呆（state 演化 + boredom 累积 + 阈值自发兴趣）（R79）
		if spawned := idle.Tick(genome); spawned {
			_ = memory.AppendEvent(cycleID, "idle.spontaneous_interest", nil)
		}
	}

	// 后台维护
	if ep, err := memory.ConsiderSealEpisode(); err == nil && ep != nil {
		slog.Info("episode sealed", "id", ep.ID, "events_in_seg", ep.RawEndID-ep.RawStartID+1)
		_ = memory.AppendEvent(cycleID, "episode.sealed", map[string]any{"id": ep.ID})
	}
	if added, err := memory.ExtractSemantic(); err == nil && added > 0 {
		_ = memory.AppendEvent(cycleID, "semantic.candidates", map[string]any{"added": added})
	}
	// 日度维护：剪枝已消费的旧 raw_trail + 旧 working_memory，控长跑磁盘增长。
	if maintenanceDue(lifeID, cycleNow) {
		runMaintenance(lifeID)
		markBehavior("maintenance_last:"+lifeID, cycleNow)
	}
	if reset, err := ledger.MaybeResetEnergyDailyCap(); err == nil && reset {
		slog.Info("energy daily cap reset")
		_ = memory.AppendEvent(cycleID, "energy.cap_reset", nil)
	}
	// 遗忘衰减（R74 兴趣 / R82 技能）：长期不触及的兴趣 / 不用的技能逐渐淡去。
	now2 := shared.SystemClock.UnixSec()
	if err := storage.DecayInterests(lifeID, now2, 7.0); err != nil {
		slog.Warn("decay interests", "err", err)
	}
	if err := storage.DecaySkills(lifeID, now2, 30.0); err != nil {
		slog.Warn("decay skills", "err", err)
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

// approveSkillAsync 后台安装技能依赖（卡片回调 3s 截止内必须返回，安装异步跑）。
func approveSkillAsync(skillID string) {
	if err := skill.ApproveDeps(skillID, "user_approve"); err != nil {
		slog.Warn("card approve deps", "skill", skillID, "err", err)
		return
	}
	slog.Info("card approved skill", "skill", skillID)
}

func wireLark(ctx context.Context, lifeID string) {
	appID := os.Getenv("FEISHU_APP_ID")
	if appID == "" {
		slog.Info("feishu not configured; skip")
		return
	}
	if err := lark.Init(lark.Config{
		AppID:     appID,
		AppSecret: os.Getenv("FEISHU_APP_SECRET"),
		InboxDir:  filepath.Join(envOr("MINDVERSE_SANDBOX", "/workspace/sandbox"), "inbox"),
	}); err != nil {
		slog.Error("lark init", "err", err)
		return
	}
	// 注册飞书为 "feishu" 渠道的出站实现（Send/React/审批卡片）。出站事件由 egress 中央分发器
	// 按来源 channel 路由到这里——不再在本函数内散落 `!="feishu"` 守卫（哪来回哪去）。
	lark.RegisterEgress()

	// 卡片按钮回调（走长连接）→ skill 批准/拒绝（单一真相，不复制安装逻辑）。
	// 飞书卡片回调有 3s 响应硬截止：批准要装依赖（pip/npm，可达数十秒）必须异步，
	// 否则同步阻塞会让飞书提示"回调超时未响应"。装的结果反映在面板/技能状态。
	lark.SetCardActionHandler(func(action string, value map[string]any) (string, bool) {
		sid, _ := value["skill_id"].(string)
		switch action {
		case "skill_approve":
			go approveSkillAsync(sid)
			return "已批准，依赖后台安装中", true
		case "skill_approve_all":
			// 批准本次 + 后续同类（缺依赖）请求自动批准，不再逐次问。
			if err := storage.SetConfigBool("skill_auto_approve_deps", true); err != nil {
				slog.Warn("card approve_all set config", "err", err)
			}
			skill.SetAutoApprove(true)
			go approveSkillAsync(sid)
			return "已批准，后续同类请求将自动批准", true
		case "skill_reject":
			if err := skill.RejectDeps(sid); err != nil { // 仅置状态，快，可同步
				slog.Warn("card reject deps", "skill", sid, "err", err)
				return "拒绝失败", false
			}
			return "已拒绝该技能", true
		}
		return "未知操作", false
	})
	// 出站路由（审批卡片 / 对话回复 / 主动汇报）已统一交 egress 中央分发器按 channel 路由
	// （见 egress.StartDispatcher，在 main 里 wire 一处）。这里不再内联订阅 ReplyEvent /
	// ApprovalNeededEvent + `!="feishu"` 守卫——隐患①（审批卡误投飞书）与隐患②（全局
	// LastSenderOpenID 跨渠道串台）随之消除。
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

// maybeImportLife 在打开库前，按需从加密包还原一个生命体（docs/06 迁移 / 离线落地）。
//
// 触发：MINDVERSE_IMPORT 指向 .mvlife 文件 + MINDVERSE_IMPORT_PASSPHRASE 提供口令。
// 安全闸：仅当目标库**不存在**才导入——绝不覆盖正在生活的生命体（毁灭性不可逆）。
// 幂等：导入成功后库已存在，下次重启（IMPORT 仍设着）会跳过 → 正常以该生命体启动。
// 失败即 fatal：用户期望还原却失败时，不应静默改道去出生一个全新生命（会让人误以为旧生命丢了）。
func maybeImportLife(dbPath string) {
	importPath := os.Getenv("MINDVERSE_IMPORT")
	if importPath == "" {
		return
	}
	if _, err := os.Stat(dbPath); err == nil {
		slog.Warn("MINDVERSE_IMPORT set but db already exists — skipping import (won't overwrite a live life)", "db", dbPath)
		return
	}
	pass := os.Getenv("MINDVERSE_IMPORT_PASSPHRASE")
	if pass == "" {
		fatal("import life", errors.New("MINDVERSE_IMPORT set but MINDVERSE_IMPORT_PASSPHRASE is empty"))
	}
	f, err := os.Open(importPath)
	if err != nil {
		fatal("open import package", err)
	}
	defer f.Close()
	ws := envOr("MINDVERSE_WORKSPACE", "/workspace")
	man, err := lifepack.Import(f, pass, dbPath, ws)
	if err != nil {
		fatal("import life", err)
	}
	slog.Info("life imported from package", "life", man.LifeID, "genome", man.GenomeVersion, "schema", man.SchemaVersion, "exported_at", man.ExportedAt)
}
