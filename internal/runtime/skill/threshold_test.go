package skill

import "testing"

// TestSetListThreshold 验 C5：阈值可调 + 非法值（<1）被忽略守地板。
func TestSetListThreshold(t *testing.T) {
	orig := ListThreshold()
	defer SetListThreshold(orig) // 还原，免污染其它测试

	SetListThreshold(20)
	if got := ListThreshold(); got != 20 {
		t.Fatalf("设20应得20, 得 %d", got)
	}
	SetListThreshold(0) // 非法，忽略
	if got := ListThreshold(); got != 20 {
		t.Fatalf("设0应被忽略保持20, 得 %d", got)
	}
	SetListThreshold(-5) // 非法，忽略
	if got := ListThreshold(); got != 20 {
		t.Fatalf("设-5应被忽略保持20, 得 %d", got)
	}
	SetListThreshold(1) // 地板合法
	if got := ListThreshold(); got != 1 {
		t.Fatalf("设1应得1, 得 %d", got)
	}
}
