package core

// LifecycleState 宏观状态机（docs/03 §4）。
// Phase 0 七态（无 Transferred）。
type LifecycleState string

const (
	StateEmbryonic LifecycleState = "Embryonic"
	StateActive    LifecycleState = "Active"
	StateLowPower  LifecycleState = "LowPower"
	StateDormant   LifecycleState = "Dormant"
	StateArchived  LifecycleState = "Archived"
	StateDetached  LifecycleState = "Detached"
	StateMemorial  LifecycleState = "Memorial"
)
