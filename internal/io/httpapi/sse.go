package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"mindverse/internal/bus"
	"mindverse/internal/runtime/action"
	"mindverse/internal/runtime/state"
)

// SSE 客户端：每连接一个 channel。
var (
	sseMu      sync.Mutex
	sseClients = make(map[chan sseMessage]struct{})
	sseInit    sync.Once
)

type sseMessage struct {
	event string
	data  any
}

// startSSEFanout 一次性订阅 bus 事件转 SSE 广播。
func startSSEFanout() {
	sseInit.Do(func() {
		bus.Subscribe(state.StateChanged{}, func(e bus.Event) {
			ev := e.(state.StateChanged)
			broadcast("state", map[string]any{"life": ev.Life, "mental": ev.Mental, "reason": ev.Reason})
		})
		bus.Subscribe(bus.LifecycleTransitioned{}, func(e bus.Event) {
			ev := e.(bus.LifecycleTransitioned)
			broadcast("lifecycle", ev)
		})
		bus.Subscribe(bus.TickStarted{}, func(e bus.Event) {
			ev := e.(bus.TickStarted)
			broadcast("tick", ev)
		})
		bus.Subscribe(action.SpeechEvent{}, func(e bus.Event) {
			ev := e.(action.SpeechEvent)
			broadcast("speech", ev)
		})
		bus.Subscribe(bus.EpisodeSealed{}, func(e bus.Event) {
			broadcast("episode_sealed", e)
		})
		bus.Subscribe(bus.ReflectionCompleted{}, func(e bus.Event) {
			broadcast("reflection", e)
		})
		bus.Subscribe(bus.GoalEnqueued{}, func(e bus.Event) {
			broadcast("goal_enqueued", e)
		})
		bus.Subscribe(bus.ActionDone{}, func(e bus.Event) {
			broadcast("action_done", e)
		})
		bus.Subscribe(bus.ToolAudited{}, func(e bus.Event) {
			broadcast("tool_audited", e)
		})
	})
}

func broadcast(event string, data any) {
	msg := sseMessage{event: event, data: data}
	sseMu.Lock()
	defer sseMu.Unlock()
	for ch := range sseClients {
		select {
		case ch <- msg:
		default:
			// slow client; drop
		}
	}
}

func apiStream(w http.ResponseWriter, r *http.Request) {
	startSSEFanout()

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
	sseMu.Lock()
	sseClients[ch] = struct{}{}
	sseMu.Unlock()
	defer func() {
		sseMu.Lock()
		delete(sseClients, ch)
		close(ch)
		sseMu.Unlock()
	}()

	// 心跳 + 初始 state 快照
	if ls, ms := state.Snapshot(); true {
		writeSSE(w, "state", map[string]any{"life": ls, "mental": ms, "reason": "initial"})
		flusher.Flush()
	}

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			writeSSE(w, msg.event, msg.data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", b)
}
