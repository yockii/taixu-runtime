// Package eventbus in-process 事件总线。
//
// 设计纪律（TECH-STACK §13.1）：
//   - 模块之间不直接调用
//   - 通过 Publish / Subscribe + 强类型 event
//   - 同步派发：handler 在 publisher goroutine 执行；handler 应快速返回（< 10ms）或自启 goroutine
package eventbus

import (
	"reflect"
	"sync"
)

// Event 任意事件类型。实现方推荐用值类型（小 struct）。
type Event any

// Handler 事件处理函数。
type Handler func(Event)

// Bus 进程内事件总线。
type Bus struct {
	mu       sync.RWMutex
	handlers map[reflect.Type][]Handler
}

// New 创建一个 Bus。
func New() *Bus {
	return &Bus{
		handlers: make(map[reflect.Type][]Handler),
	}
}

// Subscribe 订阅指定类型的事件。
// eventProto 可传零值，仅用于类型识别（如 eventbus.Subscribe(bus, perception.EventTick{}, handler)）。
func (b *Bus) Subscribe(eventProto Event, h Handler) {
	t := reflect.TypeOf(eventProto)
	b.mu.Lock()
	b.handlers[t] = append(b.handlers[t], h)
	b.mu.Unlock()
}

// Publish 同步派发事件给所有订阅者。
func (b *Bus) Publish(e Event) {
	t := reflect.TypeOf(e)
	b.mu.RLock()
	hs := b.handlers[t]
	b.mu.RUnlock()
	for _, h := range hs {
		h(e)
	}
}

// HandlerCount 返回某类型的当前订阅数（测试用）。
func (b *Bus) HandlerCount(eventProto Event) int {
	t := reflect.TypeOf(eventProto)
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[t])
}
