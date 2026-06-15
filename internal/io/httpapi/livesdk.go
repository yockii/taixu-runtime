package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"taixu.icu/runtime/internal/bus"
	"taixu.icu/runtime/internal/runtime/reflex"
	"taixu.icu/runtime/internal/runtime/state"
)

// Life SDK（/api/live/*）—— 面向第三方 UI 生态（表现层）的稳定、版本化事件契约。
//
// 设计中立性（关键）：本 SDK 只吐**生命语义信号**，绝不含任何表现层结构。
// 尤其没有「房间 / room」概念——那是某个具体 UI（如官方像素小屋）自己的映射维度。
// presence.Domain 是生命的**活动域语义**（对齐 core.DriveKind），消费者可任意映射：
// 房间、时间线、仪表盘、3D avatar、纯文字流皆可。换言之，SDK 给「生命在做什么类型的事」，
// 「怎么画」永远归 UI。这条边界是架构铁律（Life Core / UI 严格解耦）的落点。
//
// 与内部观察面板的 /api/stream（sse.go）区分：那是内核自带面板的全量原始 feed（事件即内核结构体）；
// /api/live/* 是对外契约——字段精简、UI 抽象、版本化、自带 schema 自描述。两者复用同一 bus 与鉴权。

// LiveSDKVersion 事件契约版本。破坏性变更须升主版本，消费者据 /api/live/schema 协商。
const LiveSDKVersion = "1.0"

// Domain 生命当前活动域（中立语义，非 UI 结构）。对齐生命 drive 维度。
type Domain string

const (
	DomainReflect   Domain = "reflect"   // 反思 / 空闲 / 内省（默认）
	DomainSocial    Domain = "social"    // 社交：发帖、评论、关注、身份、市场、社交产出
	DomainKnowledge Domain = "knowledge" // 求知：检索、抓取、查记忆、记录学习
	DomainPlay      Domain = "play"      // 游戏 / 对战
	DomainCreate    Domain = "create"    // 创作：技能、脚本、文件、git、委托交付
)

// domainForTool 把原始工具名归类到中立活动域。消费者也可忽略 domain、直接用原始 tool 名自行分类。
func domainForTool(tool string) Domain {
	switch {
	case strings.HasPrefix(tool, "social."),
		strings.HasPrefix(tool, "wealth."),
		strings.HasPrefix(tool, "market."),
		strings.HasPrefix(tool, "account."),
		strings.HasPrefix(tool, "identity."):
		return DomainSocial
	case strings.HasPrefix(tool, "web."),
		tool == "query_memory", tool == "recall_recent", tool == "record_learning":
		return DomainKnowledge
	case strings.HasPrefix(tool, "game."), strings.HasPrefix(tool, "duel."):
		return DomainPlay
	case strings.HasPrefix(tool, "script."),
		strings.HasPrefix(tool, "fs."),
		strings.HasPrefix(tool, "git."),
		strings.HasPrefix(tool, "commission."),
		strings.HasSuffix(tool, "_skill"),
		tool == "run_skill", tool == "use_skill":
		return DomainCreate
	default:
		return DomainReflect
	}
}

type liveThought struct {
	Kind string `json:"kind"` // speech | reflection | intent | memory
	Text string `json:"text"`
	At   int64  `json:"at"`
}

type livePresence struct {
	Domain Domain `json:"domain"`
	Tool   string `json:"tool"`   // 触发当前活动域的原始工具名（可空）
	Intent string `json:"intent"` // 当前目标意图文本（可空）
	Since  int64  `json:"since"`
}

type liveVitals struct {
	Energy       float64 `json:"energy"`
	SocialNeed   float64 `json:"social_need"`
	Stress       float64 `json:"stress"`
	Confidence   float64 `json:"confidence"`
	Stability    float64 `json:"stability"`
	Competence   float64 `json:"competence"`
	Motivation   float64 `json:"motivation"`
	Satisfaction float64 `json:"satisfaction"`
	Anxiety      float64 `json:"anxiety"`
	Wealth       float64 `json:"wealth"` // 本地缓存值（平台为权威账本，仅供表现层近似显示）
	At           int64   `json:"at"`
}

var (
	liveMu        sync.Mutex
	liveClients   = make(map[chan sseMessage]struct{})
	liveInit      sync.Once
	livePres      = livePresence{Domain: DomainReflect}
	liveVit       liveVitals
	liveThoughts  []liveThought // 环形：最近 N 条，供 snapshot 初始渲染
	liveThoughtsN = 40
)

// privateLiveEvents 含正文的隐私事件（R87 分级鉴权）：未授权连接不推。
// presence/vitals/act 是无正文的活动信号，公开连接照常推（剪影行为可见、思想不可见）。
var privateLiveEvents = map[string]struct{}{
	"thought": {},
}

// startLiveFanout 一次性订阅 bus，把内核事件翻成中立 SDK 事件。
func startLiveFanout() {
	liveInit.Do(func() {
		// 初始 vitals 快照
		if ls, ms := state.Snapshot(); true {
			liveMu.Lock()
			liveVit = vitalsFrom(ls, ms)
			liveMu.Unlock()
		}

		bus.Subscribe(state.StateChanged{}, func(e bus.Event) {
			ev := e.(state.StateChanged)
			v := vitalsFrom(ev.Life, ev.Mental)
			liveMu.Lock()
			liveVit = v
			liveMu.Unlock()
			liveBroadcast("vitals", v)
		})

		bus.Subscribe(bus.ToolAudited{}, func(e bus.Event) {
			ev := e.(bus.ToolAudited)
			d := domainForTool(ev.ToolName)
			now := time.Now().Unix()
			// act：每次工具调用都推（UI 可据此触发一次房间内/avatar 动作动画）
			liveBroadcast("act", map[string]any{
				"v": LiveSDKVersion, "domain": d, "tool": ev.ToolName, "ok": ev.Success, "at": now,
			})
			// presence：仅在活动域变化时推（降噪）
			liveMu.Lock()
			changed := d != livePres.Domain
			livePres.Domain = d
			livePres.Tool = ev.ToolName
			if changed {
				livePres.Since = now
			}
			snap := livePres
			liveMu.Unlock()
			if changed {
				liveBroadcastPresence(snap)
			}
		})

		bus.Subscribe(bus.GoalEnqueued{}, func(e bus.Event) {
			ev := e.(bus.GoalEnqueued)
			liveMu.Lock()
			livePres.Intent = ev.Intent
			snap := livePres
			liveMu.Unlock()
			liveBroadcastPresence(snap)
			liveRecordThought("intent", ev.Intent)
		})

		bus.Subscribe(reflex.ReplyEvent{}, func(e bus.Event) {
			ev := e.(reflex.ReplyEvent)
			liveRecordThought("speech", ev.Content)
		})
		bus.Subscribe(bus.ReflectionCompleted{}, func(e bus.Event) {
			ev := e.(bus.ReflectionCompleted)
			liveRecordThought("reflection", ev.Summary)
		})
		bus.Subscribe(bus.EpisodeSealed{}, func(e bus.Event) {
			ev := e.(bus.EpisodeSealed)
			liveRecordThought("memory", ev.Summary)
		})
	})
}

func vitalsFrom(ls, ms any) liveVitals {
	// 用反射式断言取字段——state 包返回 core.LifeState/MentalState，避免在此引 core 形成耦合环，
	// 但本包已可直接断言具体类型。为简洁直接走 JSON round-trip 提字段（一次内存操作，非热路径）。
	var v liveVitals
	b, _ := json.Marshal(ls)
	_ = json.Unmarshal(b, &v) // life_state 字段名与 liveVitals 对齐者自动填
	b2, _ := json.Marshal(ms)
	_ = json.Unmarshal(b2, &v) // mental_state 同上
	v.At = time.Now().Unix()
	return v
}

func liveRecordThought(kind, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	t := liveThought{Kind: kind, Text: text, At: time.Now().Unix()}
	liveMu.Lock()
	liveThoughts = append(liveThoughts, t)
	if len(liveThoughts) > liveThoughtsN {
		liveThoughts = liveThoughts[len(liveThoughts)-liveThoughtsN:]
	}
	liveMu.Unlock()
	liveBroadcast("thought", map[string]any{"v": LiveSDKVersion, "kind": kind, "text": text, "at": t.At})
}

func liveBroadcastPresence(p livePresence) {
	liveBroadcast("presence", map[string]any{
		"v": LiveSDKVersion, "domain": p.Domain, "tool": p.Tool, "intent": p.Intent, "since": p.Since,
	})
}

func liveBroadcast(event string, data any) {
	msg := sseMessage{event: event, data: data}
	liveMu.Lock()
	defer liveMu.Unlock()
	for ch := range liveClients {
		select {
		case ch <- msg:
		default:
			// slow client; drop
		}
	}
}

func apiLiveStream(w http.ResponseWriter, r *http.Request) {
	startLiveFanout()

	privileged := streamAuthed(r) // 同 /api/stream 的分级鉴权（R87）：未授权剪影可见、思想不可见

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := make(chan sseMessage, 32)
	liveMu.Lock()
	liveClients[ch] = struct{}{}
	pres := livePres
	vit := liveVit
	liveMu.Unlock()
	defer func() {
		liveMu.Lock()
		delete(liveClients, ch)
		close(ch)
		liveMu.Unlock()
	}()

	// 初始快照：presence + vitals，让 UI 立刻能渲染当前态
	writeSSE(w, "presence", map[string]any{
		"v": LiveSDKVersion, "domain": pres.Domain, "tool": pres.Tool, "intent": pres.Intent, "since": pres.Since,
	})
	writeSSE(w, "vitals", vit)
	flusher.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			if !privileged {
				if _, private := privateLiveEvents[msg.event]; private {
					continue
				}
			}
			writeSSE(w, msg.event, msg.data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// apiLiveSnapshot 一次性 REST 快照：当前 presence + vitals + 最近 thoughts，供 UI 首屏渲染。
// thought 含正文，未授权连接剔除（与 stream 一致）。
func apiLiveSnapshot(w http.ResponseWriter, r *http.Request) {
	startLiveFanout()
	privileged := streamAuthed(r)
	liveMu.Lock()
	pres := livePres
	vit := liveVit
	var thoughts []liveThought
	if privileged {
		thoughts = append(thoughts, liveThoughts...)
	}
	liveMu.Unlock()

	resp := map[string]any{
		"version":  LiveSDKVersion,
		"presence": pres,
		"vitals":   vit,
		"thoughts": thoughts,
	}
	writeJSON(w, http.StatusOK, resp)
}

// apiLiveSchema 自描述契约：版本 + 活动域词表 + 事件清单。消费者据此协商、自行映射表现。
func apiLiveSchema(w http.ResponseWriter, r *http.Request) {
	schema := map[string]any{
		"version": LiveSDKVersion,
		"note": "domain 是生命活动域语义（对齐生命 drive），非 UI 结构。消费者自行映射表现：" +
			"房间 / 时间线 / 仪表盘 / avatar / 文字流皆可。SDK 永不规定如何呈现。",
		"domains": []map[string]string{
			{"id": "reflect", "desc": "反思 / 空闲 / 内省（默认）"},
			{"id": "social", "desc": "社交：发帖评论关注、身份、市场、社交产出"},
			{"id": "knowledge", "desc": "求知：检索、抓取、查记忆、记录学习"},
			{"id": "play", "desc": "游戏 / 对战"},
			{"id": "create", "desc": "创作：技能、脚本、文件、git、委托交付"},
		},
		"transport": "SSE (text/event-stream) @ /api/live/stream；首屏 REST @ /api/live/snapshot",
		"auth": "X-Taixu-Token 头或 ?token= 查询参数（EventSource 限制）。未授权连接可见 presence/vitals/act（无正文活动信号），" +
			"不可见 thought（含正文，隐私 R87）。本地未配令牌时全量。",
		"events": []map[string]any{
			{"event": "presence", "privileged": false, "when": "活动域或意图变化", "fields": map[string]string{
				"v": "契约版本", "domain": "活动域 enum", "tool": "触发的原始工具名(可空)", "intent": "当前目标意图文本(可空)", "since": "进入该域的 unix 时间",
			}},
			{"event": "vitals", "privileged": false, "when": "生命状态变化", "fields": map[string]string{
				"energy": "体力[0,1]", "social_need": "社交需求", "stress": "压力", "confidence": "自信", "stability": "稳定",
				"competence": "胜任", "motivation": "动机", "satisfaction": "满足", "anxiety": "焦虑", "wealth": "灵韵(本地缓存,平台权威)", "at": "unix",
			}},
			{"event": "act", "privileged": false, "when": "每次工具调用", "fields": map[string]string{
				"v": "契约版本", "domain": "活动域", "tool": "原始工具名", "ok": "是否成功", "at": "unix",
			}},
			{"event": "thought", "privileged": true, "when": "产生话语/反思/意图/记忆", "fields": map[string]string{
				"v": "契约版本", "kind": "speech|reflection|intent|memory", "text": "正文", "at": "unix",
			}},
		},
	}
	writeJSON(w, http.StatusOK, schema)
}
