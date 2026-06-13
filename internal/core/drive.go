package core

// Drive 多源目标输入之一：内驱力。
// 由需求系统从 LifeState/MentalState/Values 衍生。
// 见 docs/02 §6 / 03 §2.5。
type Drive struct {
	Kind     DriveKind `json:"kind"`
	Strength float64   `json:"strength"`
	Reason   string    `json:"reason"`
	BornAt   int64     `json:"born_at"`
}

// DriveKind 需求类型五类。
type DriveKind string

const (
	DriveKnowledge   DriveKind = "knowledge"
	DriveSocial      DriveKind = "social"
	DriveAchievement DriveKind = "achievement"
	DriveCreativity  DriveKind = "creativity"
	DriveStability   DriveKind = "stability"
	DriveGame        DriveKind = "game" // C15：参与平台游戏（有进行中对局待办时强驱动去处理欠的回合）
	DriveDuel        DriveKind = "duel" // C12：制品对战（异步竞技；低频去精进策略/发起挑战，赚灵韵+爬天梯）
)
