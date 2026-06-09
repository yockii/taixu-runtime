package storage

import (
	"path/filepath"
	"testing"
)

// TestRetrievalLogStats 验 C5：检索精度记录落表 + 聚合（filtered miss = recall 缺口信号）。
func TestRetrievalLogStats(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "m.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-c5"

	rows := []*RetrievalLog{
		// 全列（filtered=false）：注入5用2命中2，miss 不计入 recall 缺口。
		{LifeID: life, GoalID: 1, ReadyTotal: 5, Injected: 5, Filtered: false, Used: 2, Hit: 2, Miss: 0, Success: true, CreatedAt: 100},
		// 检索过滤（filtered=true）：注入8用3命中2 miss1 —— 漏了1个真用到的技能 = recall 缺口。
		{LifeID: life, GoalID: 2, ReadyTotal: 20, Injected: 8, Filtered: true, Used: 3, Hit: 2, Miss: 1, Success: false, CreatedAt: 200},
		// 检索过滤命中满（无缺口）。
		{LifeID: life, GoalID: 3, ReadyTotal: 15, Injected: 8, Filtered: true, Used: 2, Hit: 2, Miss: 0, Success: true, CreatedAt: 300},
		// 别的生命体的记录——不应混入。
		{LifeID: "other", GoalID: 9, ReadyTotal: 30, Injected: 8, Filtered: true, Used: 5, Hit: 1, Miss: 4, Success: false, CreatedAt: 400},
	}
	for _, r := range rows {
		if err := InsertRetrievalLog(r); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	st, err := RetrievalStatsSince(life, 0)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if st.Goals != 3 {
		t.Errorf("Goals 应3(隔离别的生命), 得 %d", st.Goals)
	}
	if st.FilteredObs != 2 {
		t.Errorf("FilteredObs 应2, 得 %d", st.FilteredObs)
	}
	if st.InjectedSum != 21 { // 5+8+8
		t.Errorf("InjectedSum 应21, 得 %d", st.InjectedSum)
	}
	if st.HitSum != 6 { // 2+2+2
		t.Errorf("HitSum 应6, 得 %d", st.HitSum)
	}
	// recall 缺口只算 filtered 子集：goal2 的 miss=1（全列 goal1 的 miss 不计入）。
	if st.FilteredMiss != 1 {
		t.Errorf("FilteredMiss 应1, 得 %d", st.FilteredMiss)
	}

	// 时间窗过滤：只取 created_at>=250 → 仅 goal3。
	st2, err := RetrievalStatsSince(life, 250)
	if err != nil {
		t.Fatalf("stats since: %v", err)
	}
	if st2.Goals != 1 || st2.FilteredMiss != 0 {
		t.Errorf("时间窗250后应1目标0缺口, 得 Goals=%d FilteredMiss=%d", st2.Goals, st2.FilteredMiss)
	}
}
