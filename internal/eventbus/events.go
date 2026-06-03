package eventbus

// 跨模块共享事件类型集合。模块特有的事件可在自己包内定义。
//
// 命名约定：动词过去式 + 名词，如 "GenesisCompleted" / "TickStarted"。

// GenesisCompleted Genesis 流程完成。
type GenesisCompleted struct {
	LifeID string
}

// LifecycleTransitioned 状态机迁移。
type LifecycleTransitioned struct {
	LifeID    string
	FromState string
	ToState   string
	Reason    string
}

// TickStarted Scheduler 触发新循环。
type TickStarted struct {
	LifeID  string
	CycleID int64
}

// TickFinished 循环结束。
type TickFinished struct {
	LifeID  string
	CycleID int64
}
