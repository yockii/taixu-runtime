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
	"fmt"
	"os"
	"path/filepath"
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
func ImportBundle(b *SkillBundle, trustDiscount float64) (*storage.SkillInstance, error) {
	if b == nil || strings.TrimSpace(b.SkillMd) == "" {
		return nil, fmt.Errorf("import: empty bundle")
	}
	if trustDiscount <= 0 {
		trustDiscount = DefaultTrustDiscount
	}
	inst, err := LoadFrom(b.SkillMd, Origin{})
	if err != nil {
		return nil, fmt.Errorf("import load: %w", err)
	}
	// 可执行入口落盘（复用 C4 的文件名映射）。
	if strings.TrimSpace(b.EntrypointCode) != "" {
		if fn := entrypointFilename(b.EntrypointLang); fn != "" {
			_ = os.WriteFile(filepath.Join(inst.InstallPath, fn), []byte(b.EntrypointCode), 0o644)
		}
	}
	// 折扣先验：继承发布者验证 mastery × discount，clamp [0,1]。剩下靠导入方 C2 自校准。
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
	from := "import"
	if b.PublisherDID != "" {
		from = "import:" + b.PublisherDID
	}
	if err := storage.SetSkillAuthoredFrom(inst.ID, from); err != nil {
		return nil, fmt.Errorf("import set provenance: %w", err)
	}
	return storage.GetSkillInstance(inst.ID)
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
