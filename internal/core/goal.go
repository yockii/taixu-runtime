package core

// GoalSource 目标来源三类。Phase 0 仅 IntrinsicDrive + ExternalRequest。
type GoalSource string

const (
	GoalIntrinsic  GoalSource = "IntrinsicDrive"
	GoalExternal   GoalSource = "ExternalRequest"
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

	// 请求者追踪（migration 008）：仅 ExternalRequest 闭环目标填，记下"这活儿是谁托付的"。
	// 完成后慎思层据此把成果主动回送给当初发起的人（飞书 ReqFrom=open_id）。内驱目标留空。
	ReqChannel string `json:"req_channel,omitempty"`
	ReqFrom    string `json:"req_from,omitempty"`

	// 递归研究目标树（migration 009）。状态机不变量见 storage/goal.go：
	//   - ParentID：子目标指向母目标 id；根目标为 0（DB 列 NULL）。
	//   - Depth：根=0，每下一层 +1；MaxResearchDepth 护栏防无限拆解。
	//   - ResultDigest：完成（或中间拆解）时的成果/进度摘要，供母目标综合 + 知识库。
	//   - PendingChildren：未结子目标计数；>0 即「被阻塞」
	//     （status='pending' 但 NextPendingGoal 不会选中，直到子目标全完减到 0）。
	ParentID        int64  `json:"parent_id,omitempty"`
	Depth           int    `json:"depth"`
	ResultDigest    string `json:"result_digest,omitempty"`
	PendingChildren int    `json:"pending_children"`
}
