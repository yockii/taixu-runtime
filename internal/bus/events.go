package bus

// 跨模块共享事件类型。模块特有的事件在各自包内定义。

type GenesisCompleted struct {
	LifeID string
}

type LifecycleTransitioned struct {
	LifeID    string
	FromState string
	ToState   string
	Reason    string
}

type TickStarted struct {
	LifeID  string
	CycleID int64
}

type TickFinished struct {
	LifeID  string
	CycleID int64
}

// EpisodeSealed memory 包完成一段封段。
type EpisodeSealed struct {
	LifeID    string
	EpisodeID int64
	Summary   string
	Events    int64
	StartedAt int64
	EndedAt   int64
}

// ReflectionCompleted 一次反思完成。
type ReflectionCompleted struct {
	LifeID       string
	ReflectionID int64
	Kind         string
	Promoted     int
	Summary      string
}

// GoalEnqueued 一条目标入队（仲裁后）。
type GoalEnqueued struct {
	LifeID   string
	GoalID   int64
	Source   string
	Intent   string
	Priority float64
	Payload  string
}

// ActionDone Plan/Act/Feedback 三段后。
type ActionDone struct {
	LifeID    string
	ActionID  int64
	CycleID   int64
	GoalID    int64
	Action    string
	Success   bool
	StartedAt int64
}

// ToolAudited 一次工具调用审计。
type ToolAudited struct {
	LifeID     string
	AuditID    int64
	ToolName   string
	Success    bool
	DurationMs int64
}
