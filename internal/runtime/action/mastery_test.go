package action

import (
	"testing"

	"taixu.icu/runtime/internal/core"
)

// TestMasteryDeltaValidatedDiscount 验 C2：未真收尾（!validated）的探索掌握增量大打折，
// 防"凭工具调用次数刷满结晶门槛"的浅探。
func TestMasteryDeltaValidatedDiscount(t *testing.T) {
	saved := genome
	defer func() { genome = saved }()
	genome = core.Genome{Persistence: 0.5}

	// 同样深探索（substantive≥3），validated 应显著高于 unvalidated。
	v := masteryDelta(3, true)
	u := masteryDelta(3, false)
	if u >= v {
		t.Fatalf("未验证(%.3f)应低于已验证(%.3f)", u, v)
	}
	// 折扣约 0.4 倍。
	if got, want := u, v*0.4; got < want-1e-6 || got > want+1e-6 {
		t.Fatalf("未验证应=已验证×0.4: got=%.4f want=%.4f", got, want)
	}

	// 空转（substantive==0）仍给地板，且未验证再打折。
	if masteryDelta(0, false) >= masteryDelta(0, true) {
		t.Fatal("空转未验证应低于空转已验证")
	}
	// 已验证深探索应为正且不超 1。
	if v <= 0 || v > 1 {
		t.Fatalf("已验证增量越界: %.3f", v)
	}
}
