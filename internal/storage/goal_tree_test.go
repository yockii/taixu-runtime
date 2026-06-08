package storage

import (
	"database/sql"
	"path/filepath"
	"testing"

	"mindverse/internal/core"

	_ "modernc.org/sqlite"
)

// mkLife 出生一个生命体（测试用）。
func mkLife(t *testing.T, life string) {
	t.Helper()
	if err := InsertGenome(&core.Genome{LifeID: life, BornAt: 1, GenomeVersion: "1"}); err != nil {
		t.Fatalf("genome: %v", err)
	}
}

// TestGoalTreeBlockAndUnblock 覆盖核心状态机：
//
//	建母 → 在母下挂子（母 pending_children=1） → 母被阻塞（NextPendingGoal 选不到母、只选到子）
//	→ 子完成（母 pending_children 减到 0） → 母解阻塞，NextPendingGoal 重新选中母。
func TestGoalTreeBlockAndUnblock(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "t.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-tree"
	mkLife(t, life)

	// 母目标（根：parent_id=0，depth=0）。
	parent := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "研究 X 大问题",
		Priority: 0.9, Status: core.GoalPending, CreatedAt: 100}
	pid, err := EnqueueGoal(life, parent)
	if err != nil {
		t.Fatalf("enqueue parent: %v", err)
	}

	// 挂一个子目标 + 母 pending_children +1（模拟 enqueue_subgoal 工具的两步）。
	child := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "研究子问题 A",
		Priority: 0.6, Status: core.GoalPending, CreatedAt: 101, ParentID: pid, Depth: 1}
	cid, err := EnqueueGoal(life, child)
	if err != nil {
		t.Fatalf("enqueue child: %v", err)
	}
	if err := IncPendingChildren(pid); err != nil {
		t.Fatalf("inc: %v", err)
	}

	// 母此刻被阻塞：尽管母 priority 更高，NextPendingGoal 应跳过母、选中子。
	got, err := NextPendingGoal(life, 200)
	if err != nil {
		t.Fatalf("next 1: %v", err)
	}
	if got.ID != cid {
		t.Fatalf("blocked parent should be skipped; want child %d, got %d", cid, got.ID)
	}
	if got.ParentID != pid || got.Depth != 1 {
		t.Errorf("child tree fields: parent=%d depth=%d (want %d/1)", got.ParentID, got.Depth, pid)
	}

	// 校验母确实被阻塞（pending 且 children>0），不会被另一次选取（子已 active，队列空可选）。
	pg, _ := GetGoalByID(pid)
	if pg.Status != core.GoalPending || pg.PendingChildren != 1 {
		t.Fatalf("parent should be pending+children=1, got status=%s children=%d", pg.Status, pg.PendingChildren)
	}
	if _, err := NextPendingGoal(life, 210); err != ErrNoRows {
		t.Fatalf("only blocked parent + active child remain; expect ErrNoRows, got %v", err)
	}

	// 子完成 → 母 pending_children 减到 0 → 母解阻塞。
	if err := MarkGoal(cid, core.GoalCompleted, 220); err != nil {
		t.Fatalf("mark child: %v", err)
	}
	pg, _ = GetGoalByID(pid)
	if pg.PendingChildren != 0 {
		t.Fatalf("after child done, parent children should be 0, got %d", pg.PendingChildren)
	}

	// 下个 cycle：NextPendingGoal 重新选中母（回归综合）。
	got, err = NextPendingGoal(life, 230)
	if err != nil {
		t.Fatalf("next 2: %v", err)
	}
	if got.ID != pid {
		t.Fatalf("unblocked parent should be re-selected; want %d, got %d", pid, got.ID)
	}
}

// TestGoalTreeTwoLevels 两层递归：根 → 子 → 孙。孙完成解阻塞子，子完成解阻塞根。
func TestGoalTreeTwoLevels(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "t2.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-tree2"
	mkLife(t, life)

	root := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "根", Status: core.GoalPending, CreatedAt: 1}
	rid, _ := EnqueueGoal(life, root)
	mid := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "子", Status: core.GoalPending, CreatedAt: 2, ParentID: rid, Depth: 1}
	midID, _ := EnqueueGoal(life, mid)
	_ = IncPendingChildren(rid)
	leaf := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "孙", Status: core.GoalPending, CreatedAt: 3, ParentID: midID, Depth: 2}
	leafID, _ := EnqueueGoal(life, leaf)
	_ = IncPendingChildren(midID)

	// 此时根、子都被阻塞，只有孙可选。
	got, _ := NextPendingGoal(life, 10)
	if got == nil || got.ID != leafID {
		t.Fatalf("only leaf selectable, got %+v", got)
	}
	// 孙完成 → 子解阻塞。
	if err := MarkGoal(leafID, core.GoalCompleted, 20); err != nil {
		t.Fatalf("mark leaf: %v", err)
	}
	if m, _ := GetGoalByID(midID); m.PendingChildren != 0 {
		t.Fatalf("mid still blocked: children=%d", m.PendingChildren)
	}
	got, _ = NextPendingGoal(life, 30)
	if got == nil || got.ID != midID {
		t.Fatalf("mid should be selectable now, got %+v", got)
	}
	// 子完成 → 根解阻塞。
	if err := MarkGoal(midID, core.GoalCompleted, 40); err != nil {
		t.Fatalf("mark mid: %v", err)
	}
	if r, _ := GetGoalByID(rid); r.PendingChildren != 0 {
		t.Fatalf("root still blocked: children=%d", r.PendingChildren)
	}
	got, _ = NextPendingGoal(life, 50)
	if got == nil || got.ID != rid {
		t.Fatalf("root should be selectable now, got %+v", got)
	}
}

// TestListChildrenAndDigest 验证 ListChildren 取子目标 + result_digest 往返。
func TestListChildrenAndDigest(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "t3.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	defer func() { _ = Close() }()
	const life = "life-tree3"
	mkLife(t, life)

	root := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "根", Status: core.GoalPending, CreatedAt: 1}
	rid, _ := EnqueueGoal(life, root)
	c1 := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "子1", Status: core.GoalPending, CreatedAt: 2, ParentID: rid, Depth: 1}
	c1id, _ := EnqueueGoal(life, c1)
	c2 := &core.Goal{Source: core.GoalIntrinsic, Intent: "knowledge", Payload: "子2", Status: core.GoalPending, CreatedAt: 3, ParentID: rid, Depth: 1}
	c2id, _ := EnqueueGoal(life, c2)

	if err := SetResultDigest(c1id, "子1的结论"); err != nil {
		t.Fatalf("set digest: %v", err)
	}
	_ = SetResultDigest(c2id, "子2的结论")

	kids, err := ListChildren(rid)
	if err != nil {
		t.Fatalf("list children: %v", err)
	}
	if len(kids) != 2 {
		t.Fatalf("want 2 children, got %d", len(kids))
	}
	if kids[0].ResultDigest != "子1的结论" || kids[1].ResultDigest != "子2的结论" {
		t.Errorf("digests not round-tripped: %q / %q", kids[0].ResultDigest, kids[1].ResultDigest)
	}

	n, _ := CountIncompleteChildren(rid)
	if n != 2 {
		t.Errorf("incomplete children = %d, want 2", n)
	}
	_ = MarkGoal(c1id, core.GoalCompleted, 10)
	n, _ = CountIncompleteChildren(rid)
	if n != 1 {
		t.Errorf("incomplete children after one done = %d, want 1", n)
	}
}

// TestMigration009IdempotentOnExistingData 模拟「带既有数据的库」跑 009/010：
// 先手建一个**旧 schema 的 goal_queue**（无 009 列）+ 塞一行旧目标，再跑 migrate()，
// 验证 ALTER 后旧行完好、新列取默认值（parent_id=NULL→0, depth=0, pending_children=0）——
// 证明对锁定生命的历史目标零损。再跑一次 migrate() 确认幂等（schema_migrations 跳过已应用）。
func TestMigration009IdempotentOnExistingData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	// --- 阶段 1：用裸 *sql.DB 造一个「只到 008 的旧库」+ 既有数据 ---
	raw, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	// 旧 goal_queue（含 008 的 req 列，但无 009 的树列）。
	if _, err := raw.Exec(`
		CREATE TABLE goal_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			life_id TEXT NOT NULL,
			source TEXT NOT NULL,
			intent TEXT NOT NULL,
			payload TEXT NOT NULL,
			priority REAL NOT NULL DEFAULT 0.5,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at INTEGER NOT NULL,
			started_at INTEGER,
			finished_at INTEGER,
			arbitration_note TEXT,
			req_channel TEXT,
			req_from TEXT
		);`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO goal_queue (life_id, source, intent, payload, priority, status, created_at)
		VALUES ('locked-life', 'IntrinsicDrive', 'knowledge', '历史老目标', 0.7, 'completed', 12345)`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	// 模拟「004 已应用」需建出后续迁移会 ALTER 的表（011 给 skill_instance 加 embedding 列）。
	// 真实锁定生命里 004 真跑过、skill_instance 存在；测试这里补最小表以保真。
	if _, err := raw.Exec(`CREATE TABLE skill_instance (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create skill_instance stub: %v", err)
	}
	// 把 001..008 标记为已应用，让后续 migrate() 只跑 009/010/011（绝不重建旧表）。
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (filename TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, f := range []string{
		"001_init.sql", "002_fix_semantic_promotion.sql", "003_reflex_and_interests.sql",
		"004_skills_and_mastery.sql", "005_contacts.sql", "006_skill_authored_from.sql",
		"007_contact_chat_type.sql", "008_goal_requester.sql",
	} {
		if _, err := raw.Exec(`INSERT INTO schema_migrations (filename, applied_at) VALUES (?, 0)`, f); err != nil {
			t.Fatalf("seed migration %s: %v", f, err)
		}
	}
	_ = raw.Close()

	// --- 阶段 2：用正式 storage.Init 打开（触发 migrate() 跑 009/010 的 ALTER/CREATE）---
	if err := Init(dbPath); err != nil {
		t.Fatalf("init (apply 009/010): %v", err)
	}
	defer func() { _ = Close() }()

	// 旧行完好 + 新列默认值正确。
	g, err := GetGoalByID(1)
	if err != nil || g == nil {
		t.Fatalf("read legacy goal: %v g=%v", err, g)
	}
	if g.Payload != "历史老目标" || g.Status != core.GoalCompleted {
		t.Errorf("legacy row corrupted: payload=%q status=%s", g.Payload, g.Status)
	}
	if g.ParentID != 0 || g.Depth != 0 || g.PendingChildren != 0 || g.ResultDigest != "" {
		t.Errorf("legacy row new cols not defaulted: parent=%d depth=%d children=%d digest=%q",
			g.ParentID, g.Depth, g.PendingChildren, g.ResultDigest)
	}

	// 010 知识表存在且可写。
	if _, err := InsertKnowledgeEntry("locked-life", &KnowledgeEntry{Topic: "t", Body: "b", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatalf("knowledge_entry usable: %v", err)
	}
}
