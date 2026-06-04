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
