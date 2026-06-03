// Package shared 跨模块的通用工具：时钟 / ID / 随机源。
//
// 设计纪律：
//   - 不依赖 core 之外的 internal/* 包
//   - 时钟可注入，便于测试
package shared

import "time"

// Clock 可被替换以便测试。
type Clock interface {
	Now() time.Time
	UnixSec() int64
}

type sysClock struct{}

func (sysClock) Now() time.Time { return time.Now() }
func (sysClock) UnixSec() int64 { return time.Now().Unix() }

// SystemClock 真实墙钟。
var SystemClock Clock = sysClock{}
