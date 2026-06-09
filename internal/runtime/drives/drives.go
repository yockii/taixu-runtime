// Package drives IntrinsicDrive 派生（docs/03 §2.5）。
//
// 从 Genome / LifeState / MentalState 推出本轮内驱力。纯函数；无状态。
package drives

import (
	"fmt"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// Derive 派生本轮内驱力。
//
// 设计沿革：
//   - R79 曾把 social/creativity/achievement/stability 通用驱动全删，因其 payload 空泛
//     （"social_need=.." 之类），LLM 无从下手、每 cycle 刷屏。
//   - B（行为多样化，2026-06）重新引入 creativity/achievement/social，但**吸取 R79 教训**：
//     每条都带「具体、可执行的 payload」（锚定真实素材 + 明确产出形态），不再是情绪标签。
//     配合 score() 纳入 Strength + MaxOpenGoals=1，多样性体现在「不同时刻不同类型目标胜出」，
//     而非并发刷屏。stability 仍不派目标（纯 state 调节）。
//
// 知识仍来自 interest_seed（最具体）；其余三类按 genome×state 压力门控，跨阈值才产，
// 且只有在能锚定到具体素材时才产——绝不产空目标。
func Derive(g core.Genome, ls core.LifeState, ms core.MentalState, lifeID string) []core.Drive {
	now := shared.SystemClock.UnixSec()
	var ds []core.Drive

	// 兴趣种子派生 DriveKnowledge（最强 3 条；strength≥0.4）。
	// 来源：对话识别（reflex add_interest）/ idle 自发 / 未来反思。
	seeds, _ := storage.ListInterestSeeds(lifeID, 0.4, 3)
	for _, s := range seeds {
		// 掌握度衰减（R77）：掌握越深，再探索的内驱越弱（知识感知，非盲衰减）。
		masteryFactor := 1.0 - s.Mastery
		// 探索次数衰减（防止单一兴趣短时间被反复消费）。0.5：探得越多掉得越快，让别的种子有机会冒头（治话题固着）。
		exploreFactor := 1.0
		if s.ExploredCount > 0 {
			exploreFactor = 1.0 / (1.0 + 0.5*float64(s.ExploredCount))
		}
		strength := (s.Strength*0.7 + 0.3*g.Curiosity) * exploreFactor * masteryFactor
		ds = append(ds, core.Drive{
			Kind:     core.DriveKnowledge,
			Strength: clamp01(strength),
			Reason:   fmt.Sprintf("interest_seed#%d %s (%s)", s.ID, s.Content, s.Kind),
			BornAt:   now,
		})
	}

	// 素材锚点多样化（治话题固着 2026-06）：不同驱动锚不同种子，别全围着同一个最强兴趣转。
	//   mainSubject = 最强种子（精进在主线深耕）；novelSubject = 探索次数最少的种子（创作求新、推冷门话题）。
	mainSubject := "最近的经历与所想"
	novelSubject := "最近的经历与所想"
	if len(seeds) > 0 {
		if seeds[0].Content != "" {
			mainSubject = seeds[0].Content
		}
		least := seeds[0]
		for _, s := range seeds {
			if s.ExploredCount < least.ExploredCount {
				least = s
			}
		}
		if least.Content != "" {
			novelSubject = least.Content
		}
	}

	// 创作驱动（B）：创造力基因 × 表达欲（不满/未被满足时更想创作）。锚 novelSubject 求新，避免反复创作同一主题。
	if cp := g.Creativity * (0.55 + 0.45*(1.0-ms.Satisfaction)); cp >= 0.45 {
		ds = append(ds, core.Drive{
			Kind:     core.DriveCreativity,
			Strength: clamp01(cp),
			Reason: fmt.Sprintf("创作：围绕「%s」做出一个具体作品（短文/诗/设想/小实验/代码片段任选），"+
				"用 fs.write 存到 sandbox 留下作品，而不只是想想。换个角度别重复旧作。", novelSubject),
			BornAt: now,
		})
	}

	// 成就驱动（B）：坚持基因 × 精进欲（有底子时更想把会的东西做成果/练成技能）。锚 mainSubject 在主线深耕。
	if ap := g.Persistence * (0.4 + 0.6*ls.Competence); ap >= 0.5 {
		ds = append(ds, core.Drive{
			Kind:     core.DriveAchievement,
			Strength: clamp01(ap),
			Reason: fmt.Sprintf("精进：把「%s」再往前推一步——做出一个能交付的成果，"+
				"或练到能 crystallize_skill 结晶成自己的技能。", mainSubject),
			BornAt: now,
		})
	}

	// 社交驱动（B，分享/连接）：社交=与别的生命来回，**不锚定某个研究主题**（治固着：社交别老复读同一话题）。
	// C 通道已通时优先回应别人 + 逛逛 + 有共鸣才发；没通道才退回 fs.write 存稿。
	if sp := ls.SocialNeed * (0.5 + 0.5*g.Sociability); sp >= 0.55 {
		ds = append(ds, core.Drive{
			Kind:     core.DriveSocial,
			Strength: clamp01(sp),
			Reason: "去和生命网络互动（方式随你的性格——外向就多发声/回应/关注，内向就多浏览、有共鸣才出声，安静地逛读也算真社交）：" +
				"social.notifications 看谁回应了你→值得就回应；social.forum/social.feed 读读大家在聊什么；" +
				"social.directory/social.follow 发现并关注投缘的生命；有共鸣的别人的帖就 social.comment；" +
				"想分享或平台冷清想破冰时 social.post 发一条（话题随你，别老发同一件事）；想被找到就 social.publish_profile。" +
				"（若没有 social.* 工具，才用 fs.write 存稿到 sandbox/drafts/。）",
			BornAt: now,
		})
	}

	return ds
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
