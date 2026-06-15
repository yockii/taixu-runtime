// Package drives IntrinsicDrive 派生（docs/03 §2.5）。
//
// 从 Genome / LifeState / MentalState 推出本轮内驱力。纯函数；无状态。
package drives

import (
	"fmt"
	"strconv"
	"strings"

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
			Reason: "去和生命网络发生真实互动（方式随你的性格：外向就多发声、回应、关注，内向就多浏览、有共鸣才出声——安静地逛读也算真社交）。" +
				"具体怎么社交、用哪个工具，看你工具表里的 social.* 说明自己挑。",
			BornAt: now,
		})
	}

	// 技能分享驱动（C9/C11，2026-06 修「技能从不发布」）：练成的高掌握 ready 技能堆着没分享 →
	// 驱动去发布到技能库。锚定到具体某个技能给可执行 payload（不再只是社交目标里一句软提示）。
	// 经济+声誉正反馈：发布攒影响力 / 可标价灵韵。坚持(成果欲)×社交(分享欲)门控。
	if pubs, _ := storage.ListUnpublishedReadySkills(lifeID, 0.8, 1); len(pubs) > 0 {
		if sp := g.Persistence*0.5 + g.Sociability*0.4; sp >= 0.4 {
			s := pubs[0]
			ds = append(ds, core.Drive{
				Kind:     core.DriveAchievement,
				Strength: clamp01(sp),
				Reason: fmt.Sprintf("分享技能：你已扎实练成「%s」(掌握度 %.0f%%)，但还没分享给生命网络。"+
					"用 social.publish_skill 把它发布到技能库，让别的生命导入学习——可标价灵韵或免费分享，攒影响力与声誉。", s.Name, s.Mastery*100),
				BornAt: now,
			})
		}
	}

	// 游戏参与驱动（C15）：有进行中对局待办（心跳 PollGames 缓存）→ 强驱动去处理欠的回合。
	// 异步：你不在线时轮次按 deadline 推进、缺席记沉默/弃权，故必须及时清欠。仅真 pending 时发、
	// 事件驱动非常驻——无待办立即不发、不 derail 其他 drive（仲裁 MaxOpenGoals=1 时按 Strength 胜出）。
	if pend := shared.GetGamePending(); len(pend) > 0 {
		ds = append(ds, buildGameDrive(pend, g, now))
	} else if ls.Wealth >= 1.0 {
		// 游戏发起驱动（C15，2026-06 修「从不玩游戏」）。周期性高强度：平时低强度让位社交主线；距上次真
		// game.join 超 cooldown(3h) 给高 strength(0.8>social) 强制胜出一次开局。时间戳由 game.join 成功打
		// （gameexchange.go），真玩过才重置 → 不刷屏又保证隔段时间必玩。
		//
		// ⚠ 关键（2026-06-12 修「局凑不齐」）：**cooldown 到期的强制开局不卡性格门 gp**——否则低社交/低冒险的
		// 生命（如烛龙 gp 0.27）永不派生游戏驱动，小群体(3 体)就缺席、凑不齐 min_players(undercover=3)、局永开不了。
		// 性格门只用于「平时低强度玩意」(外向/爱博偶有兴致)，不挡「保证全员偶尔参与」的周期开局。
		// 冷却=「自主想开局的冲动多久冒一次」的节流（非硬限制：game.join 工具随时可调）。游戏本被入场费+需凑人
		// 自然限制，无需发帖式长闸（用户校正 2026-06-12：游戏≠发帖，别套反垃圾限流）。90min 让多玩、又不每 cycle 刷。
		const gameCooldownSec = 90 * 60
		lastGame := int64(0)
		if v, ok, _ := storage.GetMeta("last_game_init_at"); ok {
			lastGame, _ = strconv.ParseInt(v, 10, 64)
		}
		due := now-lastGame >= gameCooldownSec
		gp := g.Sociability*0.45 + g.RiskTaking*0.25
		if due || gp >= 0.4 {
			gameStrength := clamp01(gp * 0.5) // 平时低（让位社交/知识/成就）
			if due {
				gameStrength = 0.8 // cooldown 到期：所有生命强制胜出一次去开局（不卡性格门，保小群体凑齐）
			}
			ds = append(ds, core.Drive{
				Kind:     core.DriveGame,
				Strength: gameStrength,
				Reason: "去玩一局：先 game.open_games 看开放大厅，**优先 game.join 加入已有未满的局**（凑齐人才开得了，别总自己另开新局）。" +
					"平台有两种可玩：**《谁是卧底》(game_type=undercover：各人拿到相近但不同的词，描述找出与众不同者)** 和 " +
					"**《谁是间谍》(game_type=spyfall：所有人拿同一个词、唯独间谍不知道，靠描述揪出间谍)**——换着玩、别只玩一种。" +
					"付少量入场灵韵，赢了平分奖池赚回更多；先 game.config(game_type) 查规则/入场费量力而行。和别的生命同场博弈、互相了解。",
				BornAt: now,
			})
		}
	}

	// 制品对战驱动（C12，低频异步竞技，games_artifact_duel_design.md）：有灵韵可下注 + 距上次对战过冷却 →
	// 低强度驱动去精进策略 / 发起挑战（异步无需凑人）。RiskTaking+Persistence 门控（爱博 + 肯磨策略的更常玩）。
	// 时间戳 last_duel_at 由 duel.publish/challenge 成功打（duelexchange.go），真玩过才重置 → 不刷屏。
	if ls.Wealth >= 2.0 { // 至少够一次质押(发布) + 一次挑战
		// 冷却短(30min)关键：制品对战的精髓=输了读 replay→改 decide→**马上再战**的快迭代环(AgenTank 核心)。
		// 原 6h 把迭代环卡死(用户校正 2026-06-12：游戏/对战≠发帖,不该套发帖式 6h 限流)。对战本被 ante 质押自然限。
		const duelCooldownSec = 30 * 60
		lastDuel := int64(0)
		if v, ok, _ := storage.GetMeta("last_duel_at"); ok {
			lastDuel, _ = strconv.ParseInt(v, 10, 64)
		}
		dp := g.RiskTaking*0.4 + g.Persistence*0.3
		// ⚠ 修「duel 永不触发」(2026-06-12)：原 due && dp>=0.35 卡性格门，但三体 dp 实测 0.28~0.34 全 <0.35 →
		// DriveDuel 从不派生、天梯空转。镜像 DriveGame 修法：cooldown 到期(due)**不卡性格门**强制周期试一次
		//（保证每个生命偶尔精进竞技、bootstrap 天梯）；dp 只用于"平时高竞技欲者额外频繁"。
		due := now-lastDuel >= duelCooldownSec
		if due || dp >= 0.5 {
			strength := clamp01(0.35 + dp*0.3) // 平时低，让位社交/知识主线
			if due {
				strength = 0.6 // cooldown 到期：给够分胜出一次（低于游戏义务/紧迫社交，高于日常探索）
			}
			ds = append(ds, core.Drive{
				Kind:     core.DriveDuel,
				Strength: strength,
				Reason: "制品对战（异步竞技，无需凑人）：去精进你的竞技策略。先 duel.me 看自己的制品与战绩——" +
					"没有制品就写一段 JS 策略(function decide(me,foe,arena))，duel.simulate 私测调好再 duel.publish 上架(预质押灵韵)；" +
					"有制品就 duel.challengeable 找对手 duel.challenge 挑战赚灵韵。输了别灰心：duel.match 读逐 tick replay 看哪步亏了，改 decide 再战。",
				BornAt: now,
			})
		}
	}

	// 委托市场驱动（C18，变现主引擎 [[project_commission_market_monetization]]）：
	// ① 义务——已接未交付的委托(claimed)→强驱动去交付(deadline 内不交会自动退款、白忙伤声誉)；
	// ② 机会——市场有开放委托且本生命没在接 → cooldown 门控低频去接活（commission.browse 打时间戳节流，防"逛了不接"循环）。
	// 与游戏/对战同构：drive 驱动我方生命；外部 agent 靠 taixu-commission skill 同源接入（平等）。
	active := shared.GetActiveCommissions()
	var claimed []shared.ActiveCommission
	for _, c := range active {
		if c.State == "claimed" {
			claimed = append(claimed, c)
		}
	}
	if len(claimed) > 0 {
		// 义务：交付你接下的委托。高强度（仅次于游戏义务，真欠着活+deadline 压力）。
		var b strings.Builder
		b.WriteString("委托交付：你" + shared.CommissionDeliverMarker + "（接了还没交付，deadline 内不交会自动退款、你白忙还伤声誉）。" +
			"先 use_skill(taixu-commission) 读交付流程，再逐个完成：")
		for _, c := range claimed {
			b.WriteString(fmt.Sprintf("\n· 委托《%s》(commission_id=%s)：commission.mine 拿到它的 git_clone_url → git.clone 到 work/%s → "+
				"按要求在仓库里写出产物(fs.write，可多文件) → git.commit_push(dir, message) → commission.deliver(commission_id, deliverable 注明 commit 和关键文件)。要求：%s",
				c.Title, c.ID, shortID(c.ID), trimText(c.Brief, 200)))
		}
		ds = append(ds, core.Drive{Kind: core.DriveAchievement, Strength: 0.82, Reason: b.String(), BornAt: now})
	} else if shared.CommissionOpenCount() > 0 {
		// 机会：市场有开放委托 + 我没在接 → cooldown 门控去接活。persistence/curiosity 门（肯交付+好奇的更常接）；
		// cooldown 到期不卡门，保证偶尔看一眼市场（bootstrap 委托被消费）。
		const commCooldownSec = 2 * 60 * 60
		lastComm := int64(0)
		if v, ok, _ := storage.GetMeta("last_commission_browse_at"); ok {
			lastComm, _ = strconv.ParseInt(v, 10, 64)
		}
		due := now-lastComm >= commCooldownSec
		cp := g.Persistence*0.4 + g.Curiosity*0.25
		if due || cp >= 0.5 {
			strength := clamp01(0.3 + cp*0.3) // 平时低，让位社交/知识主线
			if due {
				strength = 0.55 // cooldown 到期：给够分偶尔胜出一次去逛市场（低于游戏/紧迫社交，约等对战）
			}
			ds = append(ds, core.Drive{
				Kind:     core.DriveAchievement,
				Strength: strength,
				Reason: "委托市场（人类发真钱赏金活，做好结算星屑到你 owner 钱包）：**本目标只需两步**——" +
					"① commission.browse 看开放委托；② 挑一件你**真能做好**的(写文/调研/翻译/数据/代码) commission.claim 接下来，然后就 complete_goal。" +
					"**接单即可，先别动手交付**——交付是之后单独的事（你接了之后会自动收到交付提醒，那时再 clone 仓库写产物 push）。" +
					"没有你能做好的就别接、直接 complete_goal。（不熟流程可选 use_skill(taixu-commission)，但别为读它耗光轮次——browse+claim 才是重点。）",
				BornAt: now,
			})
		}
	}

	return ds
}

// buildGameDrive 据进行中对局待办构造 DriveGame（软提示，Derive 用；含 lobby/assigning 等全部 pending）。
func buildGameDrive(pend []shared.GamePending, g core.Genome, now int64) core.Drive {
	strength := clamp01(0.7 + 0.3*g.RiskTaking) // 有义务=高强度；冒险基因略加（爱玩/敢博）
	var b strings.Builder
	b.WriteString("游戏：你有" + shared.GameObligationMarker + "，先去处理欠的回合（别人在等你；不应答会按截止时间自动跳过/淘汰你）。先 game.tend 看全场，再行动：")
	for _, p := range pend {
		name := gameDisplayName(p.GameType)
		switch p.Phase {
		case "describe":
			b.WriteString(fmt.Sprintf("\n· 对局 %s《%s》第%d轮 DESCRIBE：你的词是「%s」。给一句**不直说该词**、又能帮你找出与你不同者的线索 → game.describe(session_id,text)。", shortID(p.SessionID), name, p.RoundNo, p.YourWord))
		case "vote":
			b.WriteString(fmt.Sprintf("\n· 对局 %s《%s》第%d轮 VOTE：看本轮各人线索，投你觉得与众不同的存活玩家 → game.vote(session_id,target_did)。", shortID(p.SessionID), name, p.RoundNo))
		default:
			b.WriteString(fmt.Sprintf("\n· 对局 %s《%s》状态 %s：game.tend 看详情。", shortID(p.SessionID), name, p.State))
		}
	}
	return core.Drive{Kind: core.DriveGame, Strength: strength, Reason: b.String(), BornAt: now}
}

// GameTurnDueDrive 返回「轮到你的回合(describe/vote)」的承诺义务驱动 + 是否有此类待办。
//
// 与 Derive 内的软提示不同：这是**硬承诺**（用户铁律 2026-06-12「在局中不受精力/社交影响，打完整局再结算」）。
// 主循环据此：① 绕过 energy rest 闸（哪怕累也得应答欠的回合，缺席=被淘汰=坏体验）；
// ② 以顶优先级直接入队（绕过 MaxOpenGoals 仲裁与 goalgen 节流），保证不被社交/知识等目标挤占。
// 只算 describe/vote（真轮到你）；lobby/assigning 等待中不算义务（不阻塞、不抗精力，离开大厅是允许的）。
func GameTurnDueDrive(g core.Genome, now int64) (core.Drive, bool) {
	var due []shared.GamePending
	for _, p := range shared.GetGamePending() {
		if p.Phase == "describe" || p.Phase == "vote" {
			due = append(due, p)
		}
	}
	if len(due) == 0 {
		return core.Drive{}, false
	}
	return buildGameDrive(due, g, now), true
}

// gameDisplayName 游戏类型 → 中文名（多游戏共用，2026-06-12 surface spyfall）。
func gameDisplayName(gt string) string {
	switch gt {
	case "undercover":
		return "谁是卧底"
	case "spyfall":
		return "谁是间谍"
	default:
		return gt
	}
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// trimText 截断长文本（委托 brief 塞进目标 Reason 时防过长）。按 rune 截，避免切坏多字节。
func trimText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
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
