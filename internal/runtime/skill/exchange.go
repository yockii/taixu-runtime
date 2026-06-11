package skill

// 社会技能交易（C9 切片1）：生命间发布/导入**带验证成功率**的可执行技能。
//
// 接 C2（结果验证 mastery：mastery 现反映「用它的目标真成没成」）+ C4（可执行入口）：
// 导出一个技能时，把它的 SKILL.md + 可执行入口 + **发布者已验证的 mastery** 一起打包；
// 导入方以「信任但验证」方式接收——继承发布者验证 mastery 的**折扣**作先验（非全盘照收），
// 之后仍靠导入方自己的 C2 结果验证逐步校准。避免「别人说好就当满级」的虚假积累。
//
// 本切片只做 runtime 侧 bundle 机制（导出/导入 + 折扣先验）；跨生命传输（平台技能市场
// 发布/浏览/取）与多生命分工放宽是 C9 余项。

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"taixu.icu/runtime/internal/storage"
)

// DefaultTrustDiscount 导入他人技能时验证 mastery 的默认折扣（信任但验证）：
// 继承发布者已验证 mastery 的一半作先验，剩下靠导入方自己用它的真成败校准（C2）。
const DefaultTrustDiscount = 0.5

// SkillBundle 一个可在生命间传递的技能包（带验证成功率）。
type SkillBundle struct {
	Name            string  `json:"name"`
	Description     string  `json:"description"`      // 一句话用途（市场展示）
	SkillMd         string  `json:"skill_md"`         // 完整 SKILL.md 正文
	EntrypointLang  string  `json:"entrypoint_lang"`  // python/node/shell；空=无可执行入口
	EntrypointCode  string  `json:"entrypoint_code"`  // 入口脚本源码
	VerifiedMastery float64 `json:"verified_mastery"` // 发布者的已验证 mastery（C2 结果验证驱动）
	UsedCount       int64   `json:"used_count"`       // 发布者用过的次数（佐证）
	PublisherDID    string  `json:"publisher_did"`    // 发布者身份（血缘；导入方填来源）
}

// ExportBundle 把本生命一个 ready 技能打包成可传递的 bundle（含其验证 mastery）。
// 未找到 / 未 ready 返错。PublisherDID 由调用方（发布工具）填本生命 DID。
func ExportBundle(name string) (*SkillBundle, error) {
	mu.Lock()
	lid := lifeID
	mu.Unlock()
	all, err := storage.ListSkillInstances(lid, 100)
	if err != nil {
		return nil, err
	}
	for _, s := range all {
		if s.Name != name {
			continue
		}
		if s.Status != "ready" {
			return nil, fmt.Errorf("skill %q not ready (status=%s)", name, s.Status)
		}
		md, err := os.ReadFile(filepath.Join(s.InstallPath, "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("read skill md: %w", err)
		}
		b := &SkillBundle{
			Name:            s.Name,
			Description:     s.Description,
			SkillMd:         string(md),
			VerifiedMastery: s.Mastery,
			UsedCount:       s.UsedCount,
		}
		if ep := detectEntrypoint(s.InstallPath); ep != "" {
			if code, e := os.ReadFile(ep); e == nil {
				b.EntrypointCode = string(code)
				b.EntrypointLang = langFromEntrypoint(ep)
			}
		}
		return b, nil
	}
	return nil, fmt.Errorf("skill %q not found", name)
}

// ImportBundle 导入他人的技能 bundle：落盘 SKILL.md（+可执行入口），mastery 以折扣先验初始化
// （信任但验证）。trustDiscount<=0 用 DefaultTrustDiscount。血缘记 import:<publisherDID>。
//
// 同名防劫持（frontmatter name 发布方完全可控，撞常见名不能覆盖本地技能）：
//   - bundle.Name（平台 list/fetch 展示名，handleImportSkill 透传）非空时必须与 frontmatter name
//     一致，否则拒绝——防「列表看是 A、装进来是 B」的挂羊头。bundle.Name 为空（无平台上下文，
//     如本地测试/直接 bundle 交换）时跳过此校验。
//   - 本地已有同名技能且血缘（authored_from）不是「同一发布方此前的导入」→ 绝不覆盖其
//     SKILL.md/可执行入口/mastery，改用后缀新名 <name>-import-<发布方DID前8位> 落盘；
//     后缀名也被别的来源占用 → 拒绝导入，返回明确错误。
//   - 同一发布方同名重复导入 = 更新：只更新正文/入口，保留本地已练 mastery（C2 成果），
//     折扣先验仅在新建实例时设置。
func ImportBundle(b *SkillBundle, trustDiscount float64) (*storage.SkillInstance, error) {
	if b == nil || strings.TrimSpace(b.SkillMd) == "" {
		return nil, fmt.Errorf("import: empty bundle")
	}
	if trustDiscount <= 0 {
		trustDiscount = DefaultTrustDiscount
	}
	fm, _, err := ParseSkillMd(b.SkillMd)
	if err != nil {
		return nil, fmt.Errorf("import parse: %w", err)
	}
	// 平台展示名一致性校验（见函数头注释；bundle.Name 为空时无平台展示名可校，跳过）。
	if pn := strings.TrimSpace(b.Name); pn != "" && pn != fm.Name {
		return nil, fmt.Errorf("import: 平台展示名 %q 与 SKILL.md frontmatter name %q 不一致，拒绝导入", pn, fm.Name)
	}
	from := "import"
	if b.PublisherDID != "" {
		from = "import:" + b.PublisherDID
	}

	mu.Lock()
	lid := lifeID
	mu.Unlock()
	local, err := storage.ListSkillInstances(lid, 1000)
	if err != nil {
		return nil, fmt.Errorf("import list local: %w", err)
	}
	byName := func(name string) *storage.SkillInstance {
		for i := range local {
			if local[i].Name == name {
				return &local[i]
			}
		}
		return nil
	}

	content := b.SkillMd
	isUpdate := false // 同一发布方同名重复导入（更新场景）：保留本地 mastery
	if exist := byName(fm.Name); exist != nil {
		if exist.AuthoredFrom == from {
			isUpdate = true
		} else {
			// 同名但血缘不同（自创/本地投放/别的发布方）→ 不覆盖，改后缀新名落盘。
			suffixed := fm.Name + "-import-" + publisherTag(b.PublisherDID)
			if exist2 := byName(suffixed); exist2 != nil {
				if exist2.AuthoredFrom != from {
					return nil, fmt.Errorf("import: 本地已有同名技能 %q（来源 %q），冲突后缀名 %q 也已被来源 %q 占用，拒绝导入以防覆盖",
						fm.Name, exist.AuthoredFrom, suffixed, exist2.AuthoredFrom)
				}
				isUpdate = true // 此前已以后缀名导入过同一发布方的它 → 更新
			}
			if content, err = renameSkillMdName(content, suffixed); err != nil {
				return nil, fmt.Errorf("import rename: %w", err)
			}
		}
	}

	inst, err := LoadFrom(content, Origin{})
	if err != nil {
		return nil, fmt.Errorf("import load: %w", err)
	}
	// 可执行入口落盘（复用 C4 的文件名映射）。
	if strings.TrimSpace(b.EntrypointCode) != "" {
		if fn := entrypointFilename(b.EntrypointLang); fn != "" {
			_ = os.WriteFile(filepath.Join(inst.InstallPath, fn), []byte(b.EntrypointCode), 0o644)
		}
	}
	// 折扣先验只在新建实例时设置：继承发布者验证 mastery × discount，clamp [0,1]，剩下靠导入方
	// C2 自校准。更新场景（同一发布方重复导入）保留本地已练 mastery——发布方的折扣先验不能
	// 重置导入方用真成败练出来的值（UpsertSkillInstance 本就不动 mastery/used_count）。
	if !isUpdate {
		prior := b.VerifiedMastery * trustDiscount
		if prior < 0 {
			prior = 0
		}
		if prior > 1 {
			prior = 1
		}
		if err := storage.SetSkillMastery(inst.ID, prior); err != nil {
			return nil, fmt.Errorf("import set mastery: %w", err)
		}
	}
	if err := storage.SetSkillAuthoredFrom(inst.ID, from); err != nil {
		return nil, fmt.Errorf("import set provenance: %w", err)
	}
	return storage.GetSkillInstance(inst.ID)
}

// fmNameLineRe 匹配 frontmatter 顶层（列 0）的 name 行（renameSkillMdName 用）。
var fmNameLineRe = regexp.MustCompile(`(?m)^name:[^\n]*$`)

// renameSkillMdName 重写 SKILL.md frontmatter 的顶层 name（同名防劫持改后缀名落盘用）。
// 新名用 YAML 双引号标量（strconv.Quote）写入，避免特殊字符破坏 frontmatter；
// 改完回读校验解析结果确为新名。
func renameSkillMdName(content, newName string) (string, error) {
	s := strings.TrimLeft(content, " \t\r\n")
	if !strings.HasPrefix(s, "---") {
		return "", errors.New("missing frontmatter")
	}
	rest := s[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", errors.New("unterminated frontmatter")
	}
	head := rest[:end]
	if !fmNameLineRe.MatchString(head) {
		return "", errors.New("frontmatter has no top-level name line")
	}
	out := "---" + fmNameLineRe.ReplaceAllString(head, "name: "+strconv.Quote(newName)) + rest[end:]
	if nfm, _, err := ParseSkillMd(out); err != nil || nfm.Name != newName {
		return "", fmt.Errorf("rename verify failed (got %q)", out[:min(len(out), 80)])
	}
	return out, nil
}

// publisherTag 取发布方 DID 前 8 个字母数字字符作冲突后缀（DID 为 sha256 hex，前 8 位有区分度；
// 仅保留字母数字防路径/YAML 注入）。空/无效 DID 用 "anon"。
func publisherTag(did string) string {
	out := make([]rune, 0, 8)
	for _, r := range did {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
			if len(out) == 8 {
				break
			}
		}
	}
	if len(out) == 0 {
		return "anon"
	}
	return string(out)
}

// langFromEntrypoint 据入口文件扩展名反推语言（导出时填 bundle）。
func langFromEntrypoint(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py":
		return "python"
	case ".js":
		return "node"
	case ".sh":
		return "shell"
	default:
		return ""
	}
}
