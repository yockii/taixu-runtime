package drives

import (
	"path/filepath"
	"strings"
	"testing"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// TestDeriveGameDrive 验 C15：有进行中对局待办（shared 缓存）→ 发 DriveGame，reason 带具体待办（R79 非空）；无则不发。
func TestDeriveGameDrive(t *testing.T) {
	if err := storage.Init(filepath.Join(t.TempDir(), "g.db")); err != nil {
		t.Fatalf("storage.Init: %v", err)
	}
	defer func() { _ = storage.Close() }() // 释放 SQLite 句柄，免 Windows TempDir 清理锁
	g := core.Genome{RiskTaking: 0.6, Sociability: 0.5}
	ls := core.LifeState{}
	ms := core.MentalState{}

	// 无 pending → 不发 DriveGame。
	shared.SetGamePending(nil)
	for _, d := range Derive(g, ls, ms, "life-x") {
		if d.Kind == core.DriveGame {
			t.Fatalf("无 pending 不应发 DriveGame")
		}
	}

	// 有 describe 待办 → 发 DriveGame，reason 含你的词 + 相位（具体可执行，非空目标）。
	shared.SetGamePending([]shared.GamePending{{
		SessionID: "abcd1234efgh", GameType: "undercover", State: "active", Phase: "describe", RoundNo: 1, YourWord: "老虎",
	}})
	defer shared.SetGamePending(nil)
	var gd *core.Drive
	for i, ds := 0, Derive(g, ls, ms, "life-x"); i < len(ds); i++ {
		if ds[i].Kind == core.DriveGame {
			gd = &ds[i]
		}
	}
	if gd == nil {
		t.Fatalf("有 pending 应发 DriveGame")
	}
	if gd.Strength <= 0 || gd.Reason == "" {
		t.Fatalf("DriveGame 应 strength>0 + reason 非空(R79)，得 strength=%.2f reason=%q", gd.Strength, gd.Reason)
	}
	if !strings.Contains(gd.Reason, "老虎") || !strings.Contains(gd.Reason, "DESCRIBE") {
		t.Fatalf("DriveGame reason 应含具体待办(你的词+相位)，得：%s", gd.Reason)
	}
}
