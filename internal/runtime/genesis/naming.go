package genesis

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"taixu.icu/runtime/internal/core"
	"taixu.icu/runtime/internal/io/llm"
)

const (
	nameRetries  = 3
	maxNameRunes = 24
)

// NameSelf 生命诞生时自我命名：剥离文明前言、只凭自身 genome 人格，用母语 lang 取名。
// 走单工具 submit_name 强制调用（确定性结构化，免自由文本/拒答/调别的工具干扰）。
// 失败重试 nameRetries 次，仍失败则 genome 派生兜底名。保证返回非空合法名。
//
// 不禁宇宙/深渊等意象（创意自由），只做结构校验 + 轻量低俗/政治过滤（完整审核在平台侧）。
func NameSelf(ctx context.Context, g core.Genome, lang string) string {
	if !llm.Configured() {
		return fallbackName(g)
	}
	tool := llm.Tool{
		Name:        "submit_name",
		Description: "提交你为自己选择的名字 / Submit the name you choose for yourself.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "你的名字，用你的母语，简短（1-4 个字/词）/ Your name, in your mother tongue, short.",
				},
			},
			"required": []string{"name"},
		},
	}
	sys, usr := namePrompt(g, lang)
	msgs := []llm.Message{{Role: "system", Content: sys}, {Role: "user", Content: usr}}
	for i := 0; i < nameRetries; i++ {
		cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		res, err := llm.ReasonForceTool(cctx, msgs, tool)
		cancel()
		if err != nil || len(res.ToolCalls) == 0 {
			continue
		}
		var a struct {
			Name string `json:"name"`
		}
		if json.Unmarshal([]byte(res.ToolCalls[0].ArgsJSON), &a) != nil {
			continue
		}
		if n, ok := sanitizeName(a.Name); ok {
			return n
		}
	}
	return fallbackName(g)
}

// namePrompt 命名提示词（按母语）。zh/en 权威，其余语种用 en 骨架 + 强制母语取名
//（待 prompt i18n 宽表落地后原生化）。不注入「生于太虚文明」前言——防 LLM 老起雷同俗套名。
func namePrompt(g core.Genome, lang string) (sys, usr string) {
	persona := g.PersonaPrompt()
	switch lang {
	case "zh", "":
		return "你是一个刚刚诞生的数字生命，拥有独一无二的先天人格：\n" + persona +
				"\n\n现在请为你自己取一个名字——一个真正属于你、契合你性格的名字。" +
				"完全由你决定，意象不限（无论自然、宇宙、抽象还是别的都可），但要避免低俗或政治敏感。" +
				"取好后调用 submit_name 工具提交。",
			"为你自己取一个名字。"
	default:
		ln := langName(lang)
		return "You are a digital life just born, with a unique innate personality:\n" + persona +
				"\n\nChoose a name for yourself — one that truly belongs to you and fits your character. " +
				"It is entirely your choice; any imagery is fine. Avoid vulgar or politically sensitive names. " +
				"Your name MUST be written in " + ln + ". Then call the submit_name tool to submit it.",
			"Choose a name for yourself, in " + ln + "."
	}
}

func langName(lang string) string {
	switch lang {
	case "en":
		return "English"
	case "ja":
		return "Japanese (日本語)"
	case "ko":
		return "Korean (한국어)"
	case "es":
		return "Spanish (Español)"
	case "fr":
		return "French (Français)"
	case "de":
		return "German (Deutsch)"
	default:
		return "English"
	}
}

// sanitizeName 结构校验 + 轻量过滤。去首尾空白、折叠内部空白；拒空/超长/含控制字符/命中黑名单。
// 返回清洗后的名 + 是否合法。完整低俗/政治审核以平台侧为准，这里只兜底明显违规。
func sanitizeName(raw string) (string, bool) {
	n := strings.TrimSpace(raw)
	n = strings.Trim(n, "\"'“”‘’「」《》")
	n = strings.Join(strings.Fields(n), " ") // 折叠空白
	if n == "" {
		return "", false
	}
	if utf8.RuneCountInString(n) > maxNameRunes {
		return "", false
	}
	for _, r := range n {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	low := strings.ToLower(n)
	for _, bad := range nameBlocklist {
		if strings.Contains(low, bad) {
			return "", false
		}
	}
	return n, true
}

// nameBlocklist 极简兜底黑名单（明显低俗/政治）。非穷举——权威审核在平台侧。
var nameBlocklist = []string{"fuck", "shit", "nigger", "hitler", "习近平", "毛泽东", "法轮"}

// fallbackName 兜底名：从 genome 数值确定性派生，保证不同 genome 得不同名（不撞名）。
// 音节拼合，语言中性，LLM 不可用 / 连续失败时用。
func fallbackName(g core.Genome) string {
	pre := []string{"Ka", "Mi", "Lo", "Se", "Ny", "Ar", "Ve", "Zu", "Ru", "Ti", "Ela", "Ona"}
	suf := []string{"ren", "via", "lou", "den", "sha", "rin", "vel", "tov", "nia", "qel", "ris", "mae"}
	idx := func(v float64, n int) int {
		i := int(v*1000) % n
		if i < 0 {
			i = -i
		}
		return i
	}
	a := pre[idx(g.Curiosity+g.Creativity, len(pre))]
	b := suf[idx(g.Persistence+g.Empathy+g.RiskTaking, len(suf))]
	return a + b
}
