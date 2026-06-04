// Package skill SKILL.md 装载器（docs/SKILLS-AND-TOOLS §3 §4 §5）单例。
//
// 两层模型：SKILL.md 种子（Anthropic 标准 + Mindverse 扩展字段）→ skill_instance（有状态）。
//
// 装载流程：
//   解析 frontmatter → 算 seed_hash → 比对 L0 baseline 白名单
//   全在 baseline / 无 deps → status=ready
//   有缺包 → status=pending_approval + pending_deps（L3 用户授权，dangerous-skip 可自动批）
//
// 依赖安装（L3）：pip/node 装到 skill 私有目录 /skills/<id>/，命令用 exec slice 防注入（H09）。
package skill

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"mindverse/internal/shared"
	"mindverse/internal/storage"

	"gopkg.in/yaml.v3"
)

const (
	// SkillsRoot skill 私有安装根目录（容器内）。
	SkillsRoot = "/skills"
	// DepInstallTimeout 单次依赖安装超时。
	DepInstallTimeout = 300 * time.Second
)

// pkgNameRe 校验包名 + 版本约束，防 shell 注入（H09）。
var pkgNameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]+(\[[a-zA-Z0-9_,-]+\])?((==|>=|<=|~=|<|>)[0-9a-zA-Z.+-]+)?$`)

// baselinePython L0 Python 白名单（与 Dockerfile 同步，docs/SKILLS-AND-TOOLS §5.2）。
var baselinePython = set("httpx", "requests", "beautifulsoup4", "bs4", "lxml", "trafilatura",
	"pyyaml", "yaml", "pillow", "pil", "markdown", "feedparser", "python-dateutil", "dateutil")

// baselineNode L0 Node 白名单。
var baselineNode = set("axios", "cheerio", "dayjs", "js-yaml", "marked")

var (
	mu              sync.Mutex
	lifeID          string
	skillsRoot      string // 挂载的 skills 目录（容器内，如 /workspace/skills）
	autoApproveDeps bool   // dangerous-skip-permissions（R73）
)

// Init 绑定生命体 ID + skills 目录。autoApprove 来自 config（dangerous-skip）。
//
// skills 目录是宿主 mount 进来的（docker-compose ./workspace/skills:/workspace/skills）。
// 每个 skill = 一个子文件夹（Anthropic 规范）：SKILL.md + 脚本 / ref / 资源文件。
func Init(id, root string, autoApprove bool) error {
	if id == "" {
		return errors.New("skill: empty life id")
	}
	if root == "" {
		root = "/workspace/skills"
	}
	mu.Lock()
	defer mu.Unlock()
	lifeID = id
	skillsRoot = root
	autoApproveDeps = autoApprove
	return nil
}

// SetAutoApprove 运行时切换 dangerous-skip（config 改动时调）。
func SetAutoApprove(v bool) {
	mu.Lock()
	autoApproveDeps = v
	mu.Unlock()
}

// Frontmatter SKILL.md YAML 头（Anthropic 字段 + Mindverse 扩展）。
type Frontmatter struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	AllowedTools []string `yaml:"allowed-tools"`
	Runtime      struct {
		Python string `yaml:"python"`
		Node   string `yaml:"node"`
		Deps   struct {
			Python []string `yaml:"python"`
			Node   []string `yaml:"node"`
		} `yaml:"deps"`
	} `yaml:"runtime"`
	Lanes       []string `yaml:"lanes"`
	SeedVersion string   `yaml:"seed_version"`
}

// Dep 一条待装依赖。
type Dep struct {
	Runtime string `json:"runtime"` // python / node
	Package string `json:"package"` // 含版本约束的原始串
	Base    string `json:"-"`       // 不含约束的纯包名（白名单比对用）
}

// ParseSkillMd 拆 frontmatter + body。frontmatter 用 --- 包裹。
func ParseSkillMd(content string) (Frontmatter, string, error) {
	var fm Frontmatter
	s := strings.TrimLeft(content, " \t\r\n")
	if !strings.HasPrefix(s, "---") {
		return fm, "", errors.New("skill: missing frontmatter (--- block)")
	}
	rest := s[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return fm, "", errors.New("skill: unterminated frontmatter")
	}
	head := rest[:end]
	body := rest[end+4:]
	if err := yaml.Unmarshal([]byte(head), &fm); err != nil {
		return fm, "", fmt.Errorf("skill: parse frontmatter: %w", err)
	}
	if fm.Name == "" {
		return fm, "", errors.New("skill: frontmatter missing name")
	}
	return fm, strings.TrimLeft(body, "\r\n"), nil
}

// ScanDir 扫描 skills 根目录，装载每个含 SKILL.md 的子文件夹（Anthropic 文件夹规范）。
// boot 时调一次 + rescan API 触发。返回成功装载数。
func ScanDir() (int, error) {
	mu.Lock()
	root := skillsRoot
	mu.Unlock()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // 目录还没建，正常
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		folder := filepath.Join(root, e.Name())
		mdPath := filepath.Join(folder, "SKILL.md")
		content, err := os.ReadFile(mdPath)
		if err != nil {
			continue // 没有 SKILL.md 的子目录跳过
		}
		if _, err := loadFolder(folder, string(content)); err != nil {
			slog.Warn("skill scan load", "folder", folder, "err", err)
			continue
		}
		n++
	}
	return n, nil
}

// MasteryToCrystallize 知识结晶为 skill 的最低掌握度门槛（学透才结晶，免误人）。
const MasteryToCrystallize = 0.8

// AuthorFromKnowledge 把生命体学透的知识结晶成一个自创 skill（知识→skill，R80 创作半）。
//
// 生命体（deliberative LLM）提供 name / description / instructions（用自己的话写的指引）
// + 它学习时用过的 allowedTools。本函数组装 SKILL.md 文件夹（含血缘 frontmatter）并装载。
//
// 前瞻（Phase 4）：自创 skill 可在社群传授（Replica/Teach）。传前需审（R18）。
func AuthorFromKnowledge(seedID int64, name, description, instructions string, allowedTools []string) (*storage.SkillInstance, error) {
	seed, err := storage.GetInterestSeed(seedID)
	if err != nil || seed == nil {
		return nil, fmt.Errorf("skill: interest_seed#%d not found", seedID)
	}
	if seed.Mastery < MasteryToCrystallize {
		return nil, fmt.Errorf("skill: mastery %.2f < %.2f, not mastered enough to crystallize", seed.Mastery, MasteryToCrystallize)
	}
	if name == "" {
		name = seed.Content
	}
	if len(allowedTools) == 0 {
		allowedTools = []string{"web.fetch", "script.python"}
	}

	atJSON, _ := json.Marshal(allowedTools)
	var atYaml string
	{
		var tmp []string
		_ = json.Unmarshal(atJSON, &tmp)
		for _, t := range tmp {
			atYaml += "\n  - " + t
		}
	}
	authoredFrom := fmt.Sprintf("interest_seed#%d", seedID)
	content := fmt.Sprintf(`---
name: %s
description: |
  %s
allowed-tools:%s
lanes:
  - deliberative
seed_version: "0.1.0-self"
authored_from: "%s"
---

%s
`, sanitizeName(name), oneLine(description), atYaml, authoredFrom, instructions)

	inst, err := Load(content)
	if err != nil {
		return nil, err
	}
	// 标记血缘（Load 走通用路径不知道是自创）
	if err := storage.SetSkillAuthoredFrom(inst.ID, authoredFrom); err != nil {
		slog.Warn("skill set authored_from", "err", err, "id", inst.ID)
	}
	return storage.GetSkillInstance(inst.ID)
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// Load 装载一份 SKILL.md 文本（ad-hoc，如面板粘贴）。
// 写到 <skillsRoot>/<name>/SKILL.md 后按文件夹模型装载，使其与扫描装载一致、宿主可见。
func Load(content string) (*storage.SkillInstance, error) {
	fm, _, err := ParseSkillMd(content)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	root := skillsRoot
	mu.Unlock()
	folder := filepath.Join(root, sanitizeName(fm.Name))
	if err := persistBody(folder, content); err != nil {
		return nil, fmt.Errorf("skill: write folder: %w", err)
	}
	return loadFolder(folder, content)
}

// loadFolder 从一个 skill 文件夹（含 SKILL.md content）解析并 upsert 实例。
// install_path = 文件夹本身（脚本 / ref / 依赖都在其中）。
func loadFolder(folder, content string) (*storage.SkillInstance, error) {
	fm, _, err := ParseSkillMd(content)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(content))
	seedHash := fmt.Sprintf("%x", sum)
	id := seedHash[:16]

	lanesJSON, _ := json.Marshal(fm.Lanes)
	toolsJSON, _ := json.Marshal(fm.AllowedTools)
	missing := missingDeps(fm)
	now := shared.SystemClock.UnixSec()

	mu.Lock()
	lid := lifeID
	auto := autoApproveDeps
	mu.Unlock()

	// 若已存在同名 skill（同文件夹重扫），保留其 mastery/used_count（UpsertSkillInstance 按 id 更新元信息，不重置统计）。
	inst := &storage.SkillInstance{
		ID:           id,
		LifeID:       lid,
		Name:         fm.Name,
		SeedRef:      seedHash,
		SeedVersion:  fm.SeedVersion,
		Description:  fm.Description,
		Lanes:        string(lanesJSON),
		AllowedTools: string(toolsJSON),
		Status:       "ready",
		InstallPath:  folder,
		CreatedAt:    now,
	}
	if len(missing) > 0 {
		depsJSON, _ := json.Marshal(missing)
		inst.PendingDeps = string(depsJSON)
		inst.Status = "pending_approval"
	}
	if err := storage.UpsertSkillInstance(inst); err != nil {
		return nil, fmt.Errorf("skill: upsert: %w", err)
	}
	if inst.Status == "pending_approval" && auto {
		if err := ApproveDeps(id, "auto_approve"); err != nil {
			slog.Warn("skill auto-approve install failed", "err", err, "id", id)
			return storage.GetSkillInstance(id)
		}
	}
	return storage.GetSkillInstance(id)
}

// sanitizeName 把 skill name 规整为安全的文件夹名。
func sanitizeName(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "skill"
	}
	return string(out)
}

// ApproveDeps 批准并安装 skill 的待装依赖到私有目录。installedBy: user_approve / auto_approve。
func ApproveDeps(skillID, installedBy string) error {
	inst, err := storage.GetSkillInstance(skillID)
	if err != nil || inst == nil {
		return fmt.Errorf("skill: not found %q", skillID)
	}
	if inst.PendingDeps == "" {
		return storage.SetSkillReady(skillID, inst.InstallPath)
	}
	var deps []Dep
	if err := json.Unmarshal([]byte(inst.PendingDeps), &deps); err != nil {
		return fmt.Errorf("skill: bad pending_deps: %w", err)
	}

	_ = storage.UpdateSkillStatus(skillID, "installing", false)
	now := shared.SystemClock.UnixSec()
	for _, d := range deps {
		if !pkgNameRe.MatchString(d.Package) {
			_ = storage.UpdateSkillStatus(skillID, "failed", false)
			return fmt.Errorf("skill: invalid package name %q", d.Package)
		}
		if err := installDep(inst.InstallPath, d); err != nil {
			_ = storage.UpdateSkillStatus(skillID, "failed", false)
			return fmt.Errorf("skill: install %s: %w", d.Package, err)
		}
		pkg, ver := splitPkgVer(d.Package)
		_ = storage.InsertSkillDependency(&storage.SkillDependency{
			SkillID:     skillID,
			Runtime:     d.Runtime,
			Package:     pkg,
			Version:     ver,
			InstalledBy: installedBy,
			InstalledAt: now,
		})
	}
	return storage.SetSkillReady(skillID, inst.InstallPath)
}

// RejectDeps 拒绝并禁用 skill。
func RejectDeps(skillID string) error {
	return storage.UpdateSkillStatus(skillID, "disabled", true)
}

// ListReady 返回当前生命体 ready 的 skill（供 deliberative prompt 列出）。
func ListReady() ([]storage.SkillInstance, error) {
	mu.Lock()
	lid := lifeID
	mu.Unlock()
	all, err := storage.ListSkillInstances(lid, 100)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, s := range all {
		if s.Status == "ready" {
			out = append(out, s)
		}
	}
	return out, nil
}

// UseByName 按名取一个 ready skill 的 SKILL.md 全文（供 LLM 遵循），并 bump 使用计数。
// 未找到 / 未 ready 返 ("", error)。
func UseByName(name string) (string, error) {
	mu.Lock()
	lid := lifeID
	mu.Unlock()
	all, err := storage.ListSkillInstances(lid, 100)
	if err != nil {
		return "", err
	}
	for _, s := range all {
		if s.Name != name {
			continue
		}
		if s.Status != "ready" {
			return "", fmt.Errorf("skill %q not ready (status=%s)", name, s.Status)
		}
		body, err := os.ReadFile(filepath.Join(s.InstallPath, "SKILL.md"))
		if err != nil {
			return "", fmt.Errorf("read skill body: %w", err)
		}
		_ = storage.BumpSkillUsed(s.ID, shared.SystemClock.UnixSec())
		return string(body), nil
	}
	return "", fmt.Errorf("skill %q not found", name)
}

// missingDeps 返回不在 baseline 白名单的依赖。
func missingDeps(fm Frontmatter) []Dep {
	var out []Dep
	for _, p := range fm.Runtime.Deps.Python {
		base := strings.ToLower(splitBase(p))
		if !baselinePython[base] {
			out = append(out, Dep{Runtime: "python", Package: p, Base: base})
		}
	}
	for _, p := range fm.Runtime.Deps.Node {
		base := strings.ToLower(splitBase(p))
		if !baselineNode[base] {
			out = append(out, Dep{Runtime: "node", Package: p, Base: base})
		}
	}
	return out
}

// installDep 装一个依赖到 skill 私有目录（命令用 exec slice，不拼 shell — H09）。
func installDep(installPath string, d Dep) error {
	ctx, cancel := context.WithTimeout(context.Background(), DepInstallTimeout)
	defer cancel()
	var cmd *exec.Cmd
	switch d.Runtime {
	case "python":
		target := filepath.Join(installPath, "site-packages")
		_ = os.MkdirAll(target, 0o755)
		cmd = exec.CommandContext(ctx, "pip3", "install",
			"--break-system-packages", "--no-cache-dir",
			"--target", target, d.Package)
	case "node":
		_ = os.MkdirAll(installPath, 0o755)
		cmd = exec.CommandContext(ctx, "npm", "install",
			"--prefix", installPath, "--no-save",
			"--registry=https://registry.npmmirror.com", d.Package)
	default:
		return fmt.Errorf("unknown runtime %q", d.Runtime)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, truncate(string(out), 300))
	}
	return nil
}

func persistBody(installPath, content string) error {
	if err := os.MkdirAll(installPath, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(installPath, "SKILL.md"), []byte(content), 0o644)
}

// --- small helpers ---

func set(xs ...string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// splitBase 去掉版本约束与 extras，取纯包名。
func splitBase(p string) string {
	p = strings.TrimSpace(p)
	for _, sep := range []string{"==", ">=", "<=", "~=", ">", "<", "["} {
		if i := strings.Index(p, sep); i >= 0 {
			p = p[:i]
		}
	}
	return strings.TrimSpace(p)
}

func splitPkgVer(p string) (string, string) {
	for _, sep := range []string{"==", ">=", "<=", "~=", ">", "<"} {
		if i := strings.Index(p, sep); i >= 0 {
			return strings.TrimSpace(p[:i]), strings.TrimSpace(p[i:])
		}
	}
	return strings.TrimSpace(p), "*"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
