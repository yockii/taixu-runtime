// Package contextasm 统一上下文装配（ContextAssembler，docs/CONTEXT-ARCHITECTURE.md）。
//
// 收口所有 LLM 调用的「最近经历块」装配，消灭经历割裂：跨域近期显著经历自动注入历史消息（不进 system），
// 让游戏/社交/知识/对战的经历互相流通、保持行为连贯。无状态包级函数（单例风格，避注入）。
package contextasm

import (
	"fmt"
	"strings"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/shared"
	"taixu.icu/runtime/internal/storage"
)

// scanWindow 取最近多少段 episode 进候选池（再按 salience 选 top-K）。
const scanWindow = 15

// RecentExperience 生成「最近经历块」文本（作 assistant 历史消息注入；空串=无经历，调用方不注入）。
//
//	lifeID    生命体 id
//	maxChars  字符预算（lane 分层：慎思 ~1500 / reflex ~600）
//	topK      最多选几段显著经历
//
// 选取：最近 scanWindow 段里按 salience 降序取 topK（显著经历优先），再按时间正序排（叙事连贯）。
func RecentExperience(lifeID string, maxChars, topK int) string {
	// 反思感悟（C 滚动压缩 2026-06-12）：先放 1 条最近 reflection insight 作「更高层的概括」——它是从一串
	// episode 蒸馏出的规律，比细碎事件更省 token 又给「大局」。占一小段预算，余下给具体 episode。
	digest := latestReflectionDigest(lifeID, maxChars/3)
	lines := rankedExperienceLines(lifeID, maxChars-len(digest), topK)
	if len(lines) == 0 && digest == "" {
		return ""
	}
	return "（我回顾了下最近的经历——保持生活连贯用，不是新任务；要细节可 recall_recent / query_memory 深挖）\n" +
		digest + strings.Join(lines, "")
}

// latestReflectionDigest 取最近 1 条反思的 insight（无则 summary）作一行「感悟」；空/超预算→空串。
// reflection 是从一串 episode 蒸馏的规律，作经历块的「更高层概括」省 token（C 滚动压缩）。
func latestReflectionDigest(lifeID string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	rs, err := storage.ListReflections(lifeID, 1)
	if err != nil || len(rs) == 0 {
		return ""
	}
	txt := strings.TrimSpace(rs[0].Insight)
	if txt == "" {
		txt = strings.TrimSpace(rs[0].Summary)
	}
	if txt == "" {
		return ""
	}
	line := "· 感悟：" + truncate(txt, 160) + "\n"
	if len(line) > maxChars {
		return ""
	}
	return line
}

// RecentExperienceBare 同 RecentExperience，但**不带框架行**——返回纯经历条目（供 idle 自发兴趣等
// 已自带语境的 user 消息直接拼接；空切片→空串）。统一 idle 原 recentContext（原仅 recency 取 5、无 salience）。
func RecentExperienceBare(lifeID string, maxChars, topK int) string {
	return strings.Join(rankedExperienceLines(lifeID, maxChars, topK), "")
}

// rankedExperienceLines 核心装配：最近 scanWindow 段→salience 降序选 topK→时间正序→格式化为 "· [相对时间] 摘要\n" 行，
// 受 maxChars 预算截断。供 RecentExperience（带框架）与 RecentExperienceBare（裸）共用。
func rankedExperienceLines(lifeID string, maxChars, topK int) []string {
	if maxChars <= 0 || topK <= 0 {
		return nil
	}
	eps, err := storage.ListEpisodes(lifeID, "", scanWindow, 0)
	if err != nil || len(eps) == 0 {
		return nil
	}
	// 按 salience 降序选 top-K（稳定：salience 相等时保 recency——ListEpisodes 已 recency 序）。
	sorted := make([]core.Episode, len(eps))
	copy(sorted, eps)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Salience > sorted[i].Salience {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if topK > len(sorted) {
		topK = len(sorted)
	}
	picked := sorted[:topK]
	// 选中的按时间正序（叙事读起来是时间线）。
	for i := 0; i < len(picked); i++ {
		for j := i + 1; j < len(picked); j++ {
			if picked[j].StartedAt < picked[i].StartedAt {
				picked[i], picked[j] = picked[j], picked[i]
			}
		}
	}
	now := shared.SystemClock.UnixSec()
	var lines []string
	used := 0
	for _, e := range picked {
		line := strings.TrimSpace(e.Summary)
		if line == "" {
			line = strings.TrimSpace(e.Title)
		}
		if line == "" {
			continue
		}
		entry := fmt.Sprintf("· [%s] %s\n", relTime(now-e.StartedAt), truncate(line, 200))
		if used+len(entry) > maxChars {
			break
		}
		lines = append(lines, entry)
		used += len(entry)
	}
	return lines
}

// relTime 把秒差转人话相对时间。
func relTime(secAgo int64) string {
	switch {
	case secAgo < 0:
		return "刚刚"
	case secAgo < 90:
		return "刚刚"
	case secAgo < 3600:
		return fmt.Sprintf("%d分钟前", secAgo/60)
	case secAgo < 86400:
		return fmt.Sprintf("%d小时前", secAgo/3600)
	default:
		return fmt.Sprintf("%d天前", secAgo/86400)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
