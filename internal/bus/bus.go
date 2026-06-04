// Package bus 进程内事件总线（单例）。
//
// 任意模块可：bus.Subscribe(EventProto{}, handler) / bus.Publish(event)。
// 同步派发：handler 在 publisher goroutine 执行；耗时操作应自启 goroutine。
package bus

import (
	"reflect"
	"sync"
)

// Event 任意事件类型。
type Event any

// Handler 事件处理函数。
type Handler func(Event)

var (
	mu       sync.RWMutex
	handlers = make(map[reflect.Type][]Handler)
)

// Subscribe 订阅指定类型的事件。
func Subscribe(eventProto Event, h Handler) {
	t := reflect.TypeOf(eventProto)
	mu.Lock()
	handlers[t] = append(handlers[t], h)
	mu.Unlock()
}

// Publish 同步派发事件给所有订阅者。
func Publish(e Event) {
	t := reflect.TypeOf(e)
	mu.RLock()
	hs := handlers[t]
	mu.RUnlock()
	for _, h := range hs {
		h(e)
	}
}

// HandlerCount 当前订阅数（测试用）。
func HandlerCount(eventProto Event) int {
	t := reflect.TypeOf(eventProto)
	mu.RLock()
	defer mu.RUnlock()
	return len(handlers[t])
}

// Reset 清空所有订阅（测试用）。
func Reset() {
	mu.Lock()
	handlers = make(map[reflect.Type][]Handler)
	mu.Unlock()
}
