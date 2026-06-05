package reflex

import (
	"path/filepath"
	"testing"

	"mindverse/internal/storage"
)

// hourUTC 返回某 UTC 小时（当天）对应的一个 unix 秒（用于喂 inQuietHours）。
func hourUTC(h int) int64 { return int64(h) * 3600 }

func TestInQuietHours(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "q.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = storage.Close() }()

	// 未启用 → 恒 false
	if inQuietHours(hourUTC(2)) {
		t.Fatal("disabled should never be quiet")
	}

	_ = storage.SetConfigBool(cfgQuietEnabled, true)

	// 跨午夜窗口 23→8（UTC）
	_ = storage.SetConfigInt(cfgQuietStart, 23)
	_ = storage.SetConfigInt(cfgQuietEnd, 8)
	_ = storage.SetConfigInt(cfgTZOffsetMin, 0)
	cases := map[int]bool{2: true, 23: true, 7: true, 8: false, 12: false, 22: false}
	for h, want := range cases {
		if got := inQuietHours(hourUTC(h)); got != want {
			t.Errorf("overnight window: hour %d UTC → got %v want %v", h, got, want)
		}
	}

	// 同日窗口 9→17
	_ = storage.SetConfigInt(cfgQuietStart, 9)
	_ = storage.SetConfigInt(cfgQuietEnd, 17)
	if !inQuietHours(hourUTC(12)) {
		t.Error("12 UTC should be inside 9-17")
	}
	if inQuietHours(hourUTC(8)) || inQuietHours(hourUTC(17)) {
		t.Error("8 and 17 should be outside 9-17 (end exclusive)")
	}

	// 时区偏移：窗口 23→8 本地，偏移 +480（UTC+8）。UTC 16:00 = 本地 0:00 → 应静默。
	_ = storage.SetConfigInt(cfgQuietStart, 23)
	_ = storage.SetConfigInt(cfgQuietEnd, 8)
	_ = storage.SetConfigInt(cfgTZOffsetMin, 480)
	if !inQuietHours(hourUTC(16)) { // 本地 0 点
		t.Error("UTC16 (local 0:00 at +480) should be quiet")
	}
	if inQuietHours(hourUTC(6)) { // 本地 14 点
		t.Error("UTC6 (local 14:00 at +480) should NOT be quiet")
	}

	// 空窗 start==end → false
	_ = storage.SetConfigInt(cfgQuietStart, 10)
	_ = storage.SetConfigInt(cfgQuietEnd, 10)
	_ = storage.SetConfigInt(cfgTZOffsetMin, 0)
	if inQuietHours(hourUTC(10)) {
		t.Error("empty window (start==end) should be false")
	}
}
