# Skills & Tools 工程契约（V0.2.2）

> 本文档定位：Mindverse Skill 系统 + Tool registry 的工程实施契约。**非宪法基石**，可独立迭代。
>
> 状态：V0.2.2 起草（Phase 0.4+ Reflex 阶段后产出）。
>
> 上游依赖：`02 §7 Skill`（领域定义）、`04 §2.1 SkillRegistry / ToolRunner`（模块边界）、`04 §4.3`（SDK 边界）。
> 工程兄弟：`TECH-STACK.md`、`PHASE-0-PRD.md`。
> 相关风险：`10 R18 R31 R69 R70 R71 R72 R73 R74` + 反模式 `H06–H11`。

---

## 1. 范围与非目标

### 1.1 在范围

- Skill 两层模型：外层 SKILL.md 种子（与 Anthropic Agent Skills 标准对齐）vs 内层 Skill instance（Mindverse 有状态领域对象）
- SKILL.md frontmatter 契约（Anthropic 字段 + Mindverse 扩展字段）
- Skill 装载 / 实例化 / 依赖审批 / 卸载流程
- Tool 系统：两 lane（reflex / deliberative）+ tool registry 单例 + 调度规则
- Phase 0 工具集清单
- Sandbox 退化版规则（Docker 隔离之上的二次保护）
- 网页抓取三层策略
- 反模式速查

### 1.2 非目标

- 不定义 Skill 经济学（marketplace 抽成 / wealth 流转）→ `06 §7`、Phase 4+
- 不定义 Skill 跨生命体传播协议（Replica / Teach / Observe）→ `07 §4.2 §5`、Phase 4+
- 不定义 Skill NFT 化 → `06 §7.1`、Phase 5+
- 不替代 `02 §7` 的 Skill 领域定义，仅提供工程实装契约

---

## 2. 两层模型

| 层 | 对象 | 状态 | 跨生命体可携 | 标准来源 |
|---|---|---|---|---|
| 外层 · 种子 | `SKILL.md` bundle | 无状态（静态文件 + 资源） | ✅ 可分发 / 复用 / 安装 | Anthropic Agent Skills 标准 + Mindverse 扩展字段 |
| 内层 · 实例 | `skill_instance` 表行 | 有状态（mastery / 使用次数 / 关联记忆 / 装载时间） | ❌ 属于生命体 | Mindverse 领域对象（`02 §7`） |

**映射规则**：

- 同一 SKILL.md 装载到生命体 A 与 B → 产生两个独立 instance，演化出不同 mastery
- instance 通过 `seed_ref`（SKILL.md 内容 hash）回链种子
- 种子可被多个生命体共享；instance 不可

**这两层不是同一对象的两个视图，而是 distribution 层 vs runtime 层的强分离。**

---

## 3. SKILL.md 格式契约

### 3.1 frontmatter

```yaml
---
# Anthropic 标准字段
name: web-research-pro
description: |
  Multi-source web research with JS-rendered page support. Use when you need to
  gather information from articles, blogs, or documentation sites.
allowed-tools:
  - web.fetch
  - web.render
  - fs.write

# Mindverse 扩展字段
runtime:
  python: ">=3.12"
  node: ">=20"
  deps:
    python:
      - playwright==1.48.0    # 不在 baseline → 必须 L1 bundle 或 L3 用户授权
      - readability-lxml==0.8.1
    node: []
lanes:
  - deliberative              # 仅暴露到慎思 lane；reflex 看不到（避免阻塞对话）
dependency_bundle: ./wheels/  # L1 路径：bundle 内自带 wheel 目录
seed_version: "1.0.0"
seed_hash: sha256:abc123...   # 装载时校验
---
```

### 3.2 字段语义

| 字段 | 来源 | 必填 | 说明 |
|---|---|---|---|
| `name` | Anthropic | ✅ | 唯一 ID（kebab-case）|
| `description` | Anthropic | ✅ | 多行；前 200 字符进 LLM prompt（progressive disclosure）|
| `allowed-tools` | Anthropic | ✅ | 该 skill 能调用的 tool 名列表 |
| `runtime.python` / `runtime.node` | Mindverse | 否 | 运行时版本约束 |
| `runtime.deps` | Mindverse | 否 | 依赖列表（按语言分桶）|
| `lanes` | Mindverse | ✅ | `[reflex]` / `[deliberative]` / `[reflex, deliberative]` |
| `dependency_bundle` | Mindverse | 否 | L1 自带 wheel 目录路径（相对 SKILL.md）|
| `seed_version` | Mindverse | ✅ | 语义化版本 |
| `seed_hash` | Mindverse | ✅ | bundle 内容 sha256 |

### 3.3 bundle 结构

```
my-skill/
  SKILL.md             # frontmatter + body（progressive disclosure 详细说明）
  scripts/             # 可选：skill 私有脚本（py / js）
  wheels/              # 可选：L1 自带 wheel（pip install --no-index --find-links 用）
  resources/           # 可选：模板 / 提示词 / 数据文件
```

打包 → tar.gz。装载时哈希校验 = sha256(整 tar.gz)。

---

## 4. Skill 装载流程

```
用户上传 bundle  /  API POST /api/skills/load <bundle_url>
       │
       ▼
1. 下载 + 校 sha256（对照 SKILL.md `seed_hash`）
2. 解 tar.gz → /skills/<seed_hash>/
3. 解析 frontmatter
4. 验 lanes / allowed-tools 在已知 tool registry 内
5. 解析 runtime.deps → 比对 baseline 白名单
       │
       ├─ 全在 baseline       → 直接进 6
       └─ 有缺                → 5（依赖管理）
       │
       ▼
6. 创建 skill_instance（status=ready）
7. 写 skill_instance 表 + skill_dependency 表
8. 向 tool registry 注册该 skill 暴露的 tool 子集（按 lanes 分桶）
9. SSE: skill_ready
```

失败回滚：删 `/skills/<seed_hash>/` + skill_instance 标 `failed` + SSE: skill_failed。

---

## 5. Skill 依赖管理

### 5.1 四级方案

| 级 | 适用阶段 | 触发 | 安装路径 | 网络 |
|---|---|---|---|---|
| L0 baseline 白名单 | 全阶段 | 镜像构建期 | global site-packages / node_modules | 构建期联网 |
| L1 bundle 自带 wheel | Phase 1+ 主路径 | 装载时引擎自动 | `/skills/<id>/site-packages/` 私有 | `--no-index --find-links` 关网 |
| L2 platform mirror | Phase 2+ 评估 | 装载时引擎自动 | 同上 | 仅 mirror 域名 |
| L3 用户授权 | Phase 0 主路径 + Phase 1+ 兜底 | 装载时 SSE 弹窗 → 用户批准 | 同上 | 默认 pypi.org / registry.npmjs.org |

### 5.2 L0 baseline 白名单

镜像构建期装死（Dockerfile）。覆盖 80% 常见包：

| 语言 | 包 |
|---|---|
| Python | `httpx` `requests` `beautifulsoup4` `lxml` `trafilatura` `pyyaml` `pillow` `markdown` `feedparser` `python-dateutil` `numpy` `pandas` |
| Node | `axios` `cheerio` `dayjs` `js-yaml` `marked` |

白名单变更 = 镜像 tag 升级，不在运行时改。

### 5.3 L1 bundle 自带 wheel

skill 作者打包：

```bash
pip download playwright==1.48.0 -d ./wheels/  # 含传递依赖
pip download readability-lxml==0.8.1 -d ./wheels/
tar czf my-skill.tar.gz my-skill/
```

引擎装载（伪码）：

```go
exec.Command(
    "pip", "install",
    "--no-index",                          // 关网
    "--find-links", bundlePath + "/wheels/",
    "--target", "/skills/" + skillID + "/site-packages/",
    pkg + "==" + version,
)
```

实例化时 `PYTHONPATH=/skills/<id>/site-packages:$PYTHONPATH`。

### 5.4 L3 用户授权

```
缺包检测  →  skill_instance.status = pending_approval + pending_deps JSON
       │
       ▼
SSE: skill_pending_approval { skill_id, deps: [{pkg, version, source_url}] }
       │
       ▼
UI 弹窗：
  Skill <name> 申请安装：
    • pandas>=2.0  [PyPI ↗]
    • scipy>=1.10  [PyPI ↗]
  [批准]  [拒绝]
       │
       ├─ 批准  → POST /api/skills/<id>/approve_deps → 后端 exec pip → status=ready
       └─ 拒绝  → POST /api/skills/<id>/reject_deps → status=disabled
```

**安全规则**：

- 包名正则验证：`^[a-zA-Z0-9_-]+(\[[a-zA-Z0-9_,-]+\])?(==|>=|<=|~=|<|>)?[0-9a-zA-Z.+-]*$`
- 命令构造用 `exec.Command(arg, arg, ...)` slice，**禁** `sh -c "<拼接>"`
- 超时 300s
- 失败回滚（删 site-packages 子目录）
- 装载记录 append-only（`skill_dependency` 表）

### 5.5 dangerous-skip-permissions（全局 toggle）

| 项 | 值 |
|---|---|
| 配置键 | `config.runtime.skill_auto_approve_deps` |
| 类型 | bool |
| 默认 | false |
| UI 位置 | ConfigPanel |
| UI 提示 | 红字："开启 = 等同 LLM 任意 `pip install`。仅自托管 dogfooding 阶段建议开启。" |
| 审计标记 | 自动审批的 install 记录 `installed_by="auto_approve"` |

开启后流程：缺包 → 跳过 SSE 弹窗 → 直接进 install → 仍记审计。

---

## 6. Tool 系统

### 6.1 Lane 划分

| Lane | 目的 | 特征 | 入口 |
|---|---|---|---|
| Reflex（System 1）| 对话即时反应 | 轻量、零外部副作用、≤8 轮 agent loop、不阻塞 scheduler | `reflex.Handle(req)` |
| Deliberative（System 2）| 自主行动 | 重、可消耗资源、可外部副作用、由 scheduler tick 驱动 | `action.Execute(goal, cycle)` |

### 6.2 Tool registry 单例

`internal/runtime/tools/`（待实装，R70）：

```go
type Tool struct {
    Name        string
    Description string
    Parameters  map[string]any   // JSON Schema
    Lanes       []Lane           // 哪个 lane 可见
    Handler     func(ctx, args) (Result, error)
}

func Register(t Tool)
func ListFor(lane Lane) []Tool
func Dispatch(lane Lane, name string, args json.RawMessage) (Result, error)
```

- Register 时按 `Lanes` 分桶
- `ListFor(reflex)` → reflex agent loop 取
- `Dispatch` 路由到 Handler

### 6.3 Skill 装载 → Tool 暴露

Skill 装载时，按 `lanes` 字段把 `allowed-tools` 暴露的 tool（skill 私有 handler）注册到对应桶。Skill 卸载时 unregister。

核心 runtime tool（不依附 skill）始终注册（如 `update_mood` / `query_memory`）。

---

## 7. Phase 0 工具集清单

### 7.1 Reflex lane 工具

| Tool | 来源 | 实装状态 | 作用 |
|---|---|---|---|
| `update_mood` | core | ✅ Phase 0.4+ | MentalState 字段微调（每字段 ±0.2 clamp）|
| `add_interest` | core | ✅ Phase 0.4+ | 写 `interest_seed`（kind ∈ skill / knowledge / topic / experience）|
| `recall_recent` | core | 🔲 Phase 0.5 | 查最近 N 条 episode 摘要 / working memory |
| `note_to_self` | core | 🔲 Phase 0.5 | 暂存想法到 working memory，deliberative 下轮可接 |

### 7.2 Deliberative lane 工具

**内置 ToolRunner（PHASE-0-PRD §4.1 + 本文档新增）：**

| Tool | 来源 | 实装 | 备注 |
|---|---|---|---|
| `web.fetch(url)` | core | 🔲 Phase 0.5 | Tier 1+2 自动：http.get → trafilatura 直出 markdown |
| `web.render(url)` | core | 🔲 Phase 0.5 | Tier 3：rod + headless chromium，返 rendered HTML / markdown |
| `fs.read(path)` | core | 🔲 Phase 0.5 | 限 `/sandbox/` root |
| `fs.write(path, content)` | core | ✅ Phase 0.2 | 限 `/sandbox/` root |
| `fs.list(path)` | core | 🔲 Phase 0.5 | 限 `/sandbox/` root |
| `script.run.python(code)` | core | 🔲 Phase 0.5 | 超时 30s + mem 256M + 禁网 |
| `script.run.node(code)` | core | 🔲 Phase 0.5 | 同上 |
| ~~`script.run.shell`~~ | — | ❌ 不开 | 攻击面过大 |
| `time.now()` | core | 🔲 Phase 0.5 | 仅读时间 |

**记忆 / 思考 / Skill 类（runtime 内部 tool）：**

| Tool | 实装 | 作用 |
|---|---|---|
| `query_memory(layer, q, k)` | 🔲 Phase 0.5 | 跨 working / episodic / semantic / reflection 检索 |
| `seal_episode` | 🔲 Phase 0.5 | 主动封段（语义边界判定补强）|
| `write_reflection` | 🔲 Phase 0.5 | 浅反思固化（ShallowReflect 输出）|
| `enqueue_subgoal` | 🔲 Phase 0.5 | 拆子目标入 goal_queue |
| `complete_goal(id)` | 🔲 Phase 0.5 | 标记完成 + 反馈 |
| `explore_interest_seed(id)` | 🔲 Phase 0.5 | 探索 seed → 产 SemanticCandidate（R74 升级路径）|
| `use_skill(name, args)` | 🔲 Phase 1 | 调已装载 skill（mastery++）|

**Phase 0 不引入的 tool**：

- `update_values`（→ Phase 2 DeepReflect）
- `acquire_skill` 网络获取（→ Phase 4 Marketplace）
- 链上工具：`mint_nft` / `sign_pact` / `transfer_wealth`（→ Phase 3 BlockchainAdapter）
- `browser.*` 全浏览器自动化（→ Phase 1+ 评估）

### 7.3 Tool 命名约定

- 全小写 + 下划线 + 点号分组：`web.fetch` / `fs.write` / `script.run.python`
- 副作用按风险级前缀：核心 runtime tool 无前缀；skill 私有 tool 以 `skill.<name>.<tool>` 命名
- 禁带语言 / 实现细节（无 `python.requests.get` 一类）

---

## 8. Sandbox 退化版规则

Docker 隔离宿主，但容器内 root 仍可破坏生命体自身数据。Sandbox 二次保护：

| 控制点 | 实装 | 防什么 |
|---|---|---|
| `fs.*` root 锁 | 硬编码 `/sandbox/`，禁穿透 `..` | LLM 决策错误删数据库 |
| `script.run` 资源限 | timeout 30s + mem 256M（cgroup v2）+ `unshare -n`（禁网）| 死循环 / 爆内存 / 数据外泄 |
| `http.*` 网络限 | url allowlist + 内网段拦截（10/8 / 172.16/12 / 192.168/16 / 169.254/16）| SSRF / 内网爬取 |
| skill 私有目录 | `/skills/<id>/` 隔离，不跨 skill 读写 | skill 互相干扰 |

**不引入**（Phase 0 范围）：

- nsjail / firejail / bwrap（重型隔离，Phase 1+ 评估）
- seccomp filter（同上）
- 用户命名空间映射

---

## 9. 网页抓取分层（Tier 1-3）

### 9.1 三层策略

| Tier | 实装 | 体积 | 覆盖 | 触发 |
|---|---|---|---|---|
| Tier 1 | `http.get` + `bs4`/`lxml` | ~0MB | 静态 HTML / SSR / OpenGraph meta（~60%） | 默认 |
| Tier 2 | `trafilatura`（py 包，含在 baseline） | ~5MB | 文章模板自动识别 → 直出 markdown（+10%） | Tier 1 后处理 |
| Tier 3 | `rod` (Go) + `chromium-swiftshader` (alpine) | ~155MB | SPA / JS-rendered（+30%） | Tier 1+2 失败自动升级 |

### 9.2 自动升级触发

`web.fetch` 内部：

1. http.get → rendered HTML
2. trafilatura 提取 → markdown
3. 若 markdown 长度 < 200 char **或** `<noscript>` 比例 > 30% **或** DOM 文本 / DOM 节点比 < 0.1 → 升 Tier 3
4. Tier 3：rod → headless chromium 渲染 → 二次 trafilatura → markdown

### 9.3 排除候选

| 工具 | 排除理由 |
|---|---|
| Firecrawl 自托管 | ~1G 镜像 + Redis 依赖，过重；又一道资源翻译 |
| Playwright | ~350MB，与 rod 重叠 |
| Puppeteer | Node-only，体积同 Playwright |
| Crawl4AI | 依赖 Playwright；与 Mindverse 内核重叠 |
| Reader-LM 1.5B | ~3G 权重；再引入一个推理模型 |
| Jina Reader API | 云依赖，违 Phase 0 本地优先；按页计费 |

Phase 1+ 视情况评估 Firecrawl 自托管（成熟反爬场景）。

---

## 10. 反模式速查

详见 `10 §4.9 H06-H11`。简表：

| ID | 反模式 |
|---|---|
| H06 | LLM tool 暴露面包含 `pip install` / `npm install` / `apt-get` |
| H07 | Skill bundle 不带哈希 / 签名即装载 |
| H08 | 运行时 pip 装包未带 `--target <private_dir>` |
| H09 | 依赖安装命令用 `sh -c` 拼字符串 |
| H10 | 依赖审批弹窗不显示包名 / 版本 / 包链接 |
| H11 | UI 默认勾选 `dangerous-skip-permissions` |

---

## 11. 风险登记回链

| 风险 | 主题 |
|---|---|
| `R18` | 第三方 Skill 注册的恶意 / 低质过滤 |
| `R31` | 学习三档（Replica / Teach / Observe）的滥用边界 |
| `R69` | Skill 系统与 Anthropic SKILL.md 标准对齐 |
| `R70` | Deliberative lane 缺失 tool calling 机制 |
| `R71` | 动态网页抓取分层策略 |
| `R72` | Skill 依赖管理三级方案 |
| `R73` | dangerous-skip-permissions 全局 toggle |
| `R74` | Interest seed 探索语义升级 |

---

## 12. Phase 演进路径

| Phase | 关键里程碑 |
|---|---|
| Phase 0.5 | Deliberative agent loop + 核心 runtime tool（web / fs / script / memory / goal）+ R74 探索语义升级 |
| Phase 1 | Skill 装载完整流程（含 L1 bundle）+ `use_skill` tool + skill_instance mastery 累加 |
| Phase 2 | DeepReflect tool（`update_values` 走 Critique 安全门）|
| Phase 3 | BlockchainAdapter tool 子集（链上操作经 Adapter 中介）|
| Phase 4 | Marketplace 装载 skill bundle + L2 mirror 评估 |
| Phase 5+ | Skill NFT 化 + 跨生命体 Replica / Teach |

---

## 13. 致后续 Claude 实例

- 修 SKILL.md frontmatter 字段时，**先**核对 Anthropic 上游标准是否变更（避免分歧）
- 新增 tool 时务必声明 `Lanes` —— reflex lane 工具必须轻量（无 I/O 阻塞 > 1s）
- 装载流程改动须同步更新 §4 流程图 + `04 §2.1 SkillRegistry` 子模块描述
- 任何"绕过审批"的 PR 需在 PR 描述显式回答："这条路径会被 H06–H11 哪条挡住？"
- Phase 0 阶段 dogfooding 单用户 → toggle 默认 false 可以接受用户自开；Phase 1+ 多用户阶段 toggle 仅自托管模式可开
