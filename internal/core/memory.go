package core

// 四层记忆类型（docs/05）。

// WorkingMemorySlot 单次循环工作记忆条目。
type WorkingMemorySlot struct {
	ID        int64  `json:"id"`
	CycleID   int64  `json:"cycle_id"`
	Slot      string `json:"slot"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
}

// RawTrailEntry 事件流水原始记录。
type RawTrailEntry struct {
	ID        int64  `json:"id"`
	CycleID   int64  `json:"cycle_id"`
	EventType string `json:"event_type"`
	Payload   string `json:"payload"`
	CreatedAt int64  `json:"created_at"`
}

// Episode 事件记忆（聚合多条 RawTrail）。
type Episode struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title,omitempty"`
	Summary      string  `json:"summary"`
	StartedAt    int64   `json:"started_at"`
	EndedAt      int64   `json:"ended_at"`
	RawStartID   int64   `json:"raw_start_id,omitempty"`
	RawEndID     int64   `json:"raw_end_id,omitempty"`
	Salience     float64 `json:"salience"`
	EmotionScore float64 `json:"emotion_score,omitempty"`
	Embedding    []byte  `json:"-"`
	CreatedAt    int64   `json:"created_at"`
	SealedAt     int64   `json:"sealed_at,omitempty"`
}

// SemanticCandidate 候选知识。
type SemanticCandidate struct {
	ID           int64   `json:"id"`
	Content      string  `json:"content"`
	SourceRef    string  `json:"source_ref,omitempty"`
	SupportCount int     `json:"support_count"`
	Confidence   float64 `json:"confidence"`
	CreatedAt    int64   `json:"created_at"`
	LastSeenAt   int64   `json:"last_seen_at"`
}

// SemanticConfirmed 固化知识。
type SemanticConfirmed struct {
	ID            int64   `json:"id"`
	Content       string  `json:"content"`
	Confidence    float64 `json:"confidence"`
	PromotedFrom  int64   `json:"promoted_from,omitempty"`
	Embedding     []byte  `json:"-"`
	ConfirmedAt   int64   `json:"confirmed_at"`
}

// ReflectionKind 反思类型。
type ReflectionKind string

const (
	ReflectShallow ReflectionKind = "Shallow"
	ReflectDeep    ReflectionKind = "Deep" // Phase 2+
)

// ReflectionMemory 反思成果。
type ReflectionMemory struct {
	ID          int64          `json:"id"`
	Kind        ReflectionKind `json:"kind"`
	Summary     string         `json:"summary"`
	Insight     string         `json:"insight,omitempty"`
	TriggeredBy string         `json:"triggered_by,omitempty"`
	Embedding   []byte         `json:"-"`
	CreatedAt   int64          `json:"created_at"`
}
