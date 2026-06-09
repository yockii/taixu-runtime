package storage

// 技能检索精度指标（C5）：把「检索注入了什么 vs 目标实际用了什么 vs 目标成败」落表，
// 供 data-driven 调 SkillListThreshold（替固定 >8 启发）。见 migration 012。

// RetrievalLog 一次终态目标的检索精度记录（append-only）。
type RetrievalLog struct {
	LifeID     string
	GoalID     int64
	ReadyTotal int
	Injected   int
	Filtered   bool
	Used       int
	Hit        int
	Miss       int
	Success    bool
	CreatedAt  int64
}

// InsertRetrievalLog 追加一行检索精度记录（action.finalize 在目标终态调）。
func InsertRetrievalLog(r *RetrievalLog) error {
	_, err := db.Exec(`
		INSERT INTO skill_retrieval_log
			(life_id, goal_id, ready_total, injected, filtered, used, hit, miss, success, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		r.LifeID, r.GoalID, r.ReadyTotal, r.Injected, boolToInt(r.Filtered),
		r.Used, r.Hit, r.Miss, boolToInt(r.Success), r.CreatedAt)
	return err
}

// RetrievalStats 一段时间窗内的检索精度聚合（阈值推荐用）。
type RetrievalStats struct {
	Goals        int // 有检索记录的目标数
	FilteredObs  int // 其中真走了语义过滤的目标数（filtered=1）
	InjectedSum  int // 注入技能总数
	UsedSum      int // 用到技能总数
	HitSum       int // 用到且在注入集内
	MissSum      int // 用到却未注入（recall 缺口，仅 filtered）
	FilteredMiss int // filtered 子集里的 miss（判定阈值是否过低的关键）
}

// RetrievalStatsSince 汇总自 sinceTs 起本生命的检索精度。sinceTs<=0 取全量。
func RetrievalStatsSince(lifeID string, sinceTs int64) (RetrievalStats, error) {
	var s RetrievalStats
	err := db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(filtered),0),
			COALESCE(SUM(injected),0),
			COALESCE(SUM(used),0),
			COALESCE(SUM(hit),0),
			COALESCE(SUM(miss),0),
			COALESCE(SUM(CASE WHEN filtered=1 THEN miss ELSE 0 END),0)
		FROM skill_retrieval_log
		WHERE life_id = ? AND created_at >= ?`,
		lifeID, sinceTs).Scan(&s.Goals, &s.FilteredObs, &s.InjectedSum,
		&s.UsedSum, &s.HitSum, &s.MissSum, &s.FilteredMiss)
	return s, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
