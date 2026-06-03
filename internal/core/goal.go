package core

// GoalSource 目标来源三类。Phase 0 仅 IntrinsicDrive + ExternalRequest。
type GoalSource string

const (
	GoalIntrinsic GoalSource = "IntrinsicDrive"
	GoalExternal  GoalSource = "ExternalRequest"
	GoalReflection GoalSource = "ReflectionGoal" // Phase 2+
)

// GoalStatus 队列状态。
type GoalStatus string

const (
	GoalPending   GoalStatus = "pending"
	GoalActive    GoalStatus = "active"
	GoalCompleted GoalStatus = "completed"
	GoalRejected  GoalStatus = "rejected"
	GoalExpired   GoalStatus = "expired"
	GoalFailed    GoalStatus = "failed"
)

// Goal 由 GoalArbitrator 仲裁后入队的目标。
// 见 docs/02 §7 / 03 §2.6。
type Goal struct {
	ID              int64      `json:"id"`
	Source          GoalSource `json:"source"`
	Intent          string     `json:"intent"`
	Payload         string     `json:"payload"`
	Priority        float64    `json:"priority"`
	Status          GoalStatus `json:"status"`
	CreatedAt       int64      `json:"created_at"`
	StartedAt       int64      `json:"started_at,omitempty"`
	FinishedAt      int64      `json:"finished_at,omitempty"`
	ArbitrationNote string     `json:"arbitration_note,omitempty"`
}
