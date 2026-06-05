# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目结构（multi-repo 文件夹）

本仓库（`client/`）是**心域文明大试验**的一个子项目，不是整个项目根。磁盘布局：

```
Mindverse/            ← 普通文件夹，非 git
├── client/           ← 本仓库 = 数字生命体容器（私有 repo yockii/mindverse），Go runtime + 面板，Docker 形式交付
├── site/             ← 对外宣传落地页（公开 repo yockii/mindverse-site，Astro+Pages，独立 git）
├── docs/             ← 系统级顶层设计（00–09 宪法 + COMMERCIAL + 白皮书 + 数字文明.txt）；暂不入 git
├── platform/         ← 未来：平台系统（独立 repo）
└── chain/            ← 未来：区块链相关（独立 repo）
```

- **系统级设计文档在 `../docs/`**（根目录，本 repo 之外）。**工程实施文档 + 风险/决策日志在本 repo 的 `docs/`**（`client/docs/`）。
- 各子目录是**各自独立的 git 仓库**；根目录不版本化。改 client 只动 client repo，改落地页只动 site repo。

## 仓库当前状态

**Phase 0.4+ 工程实施中**。Phase 0.1–0.4 已落地（骨架 → 核心循环 → 飞书+LLM → 观察面板），Phase 0.4+ 加入 reflex / deliberative 分离（System 1 / System 2）。Phase 0.5 长跑验证 + skill / tool 系统补全进行中。

愿景文档（不可删，在根目录 `../docs/`）：
- `../docs/数字文明.txt` — 项目代号：**Mindverse 心域文明**
- `../docs/数字生命生态系统设计白皮书（V0.1）.docx` — 顶层设计白皮书 V0.1

源码与构建：
- Go 1.26+ runtime（`cmd/runtime/main.go` 单二进制）
- SQLite + sqlite-vec 持久化（`internal/storage/migrations/`）
- SvelteKit 观察面板（`web/`），构建后 embed 进 Go 二进制
- Docker 多阶段构建（`docker-compose.yml`，容器名 `mindverse-phase0`，端口 3000）

常用命令：
- 启动 / 重建：`docker compose up -d --force-recreate`
- 查日志：`docker logs mindverse-phase0 --tail 50`
- 进 SQLite：`docker exec mindverse-phase0 sqlite3 /app/data/mindverse.db`（**PowerShell 调用，Git Bash 会路径转换出错**）
- HTTP API：`http://localhost:3000/api/{state,episodes,goals,reflections,actions,interests,external-request,stream}`

## 项目本质（必读 — 决定所有架构决策）

Mindverse **不是** ChatGPT、不是 AI Assistant、不是 Agent Framework。
Mindverse **是 Digital Life Runtime（数字生命运行时平台）**。

核心区别：

| 传统 LLM 应用 | 数字生命 (Mindverse) |
|---|---|
| 输入 → 推理 → 输出 | 感知 → 记忆 → 反思 → 价值观演化 → 目标生成 → 行动 → 反馈 → 循环 |
| 事件驱动 | 持续存在（无用户输入也在思考） |
| 无状态或会话级状态 | 终身持续状态（Genome 出生确定、LifeState/MentalState 持续演化） |
| Token 是用户付费单位 | Token **不直接暴露**，用 energy/wealth/knowledge/reputation/social 五种世界资源建模 |
| 服务平台所有 | 生命体（人格 + 记忆 + 成长记录）**属于用户**，不可平台化占有 |

当用户讨论"加个功能"时，先问自己：这是在为"工具"加功能，还是在为"生命体"加能力？两者的设计取舍完全不同。

## 总体架构分层

```
┌────────────────────┐
│    UI Ecosystem    │  ← 第三方实现：Live2D / Unity / UE / 桌宠 / VR / Web
├────────────────────┤
│      Life SDK      │  ← 官方开放接口
├────────────────────┤
│    Life Runtime    │  ← 官方内核（本仓库核心）
├────────────────────┤
│ Model / Storage    │  ← LLM + 持久化
└────────────────────┘
```

**关键解耦原则**：Life Core 与 UI 严格解耦。官方只做内核 + SDK，不做表现层。任何 UI 相关讨论都应导向"SDK 暴露什么接口"而非"内核里直接画 UI"。

## Life Runtime 核心模型

白皮书使用 Go 风格结构体描述模型（**语言尚未定**，但 Go 是强烈暗示）。在敲定语言前，把这些视作**领域模型契约**，不是 Go-specific：

- **Genome** — 出生即固定的先天倾向（Curiosity / Sociability / Creativity / Persistence / RiskTaking）。
- **LifeState** — 持续变化的生命状态（Energy / Knowledge / SocialNeed / Stress / Confidence / Stability）。
- **MentalState** — 情绪层（Motivation / Satisfaction / Anxiety）。
- **Values** — 价值观权重表，指导目标排序（growth / friendship / creativity / safety …）。
- **Personality** — Genome + 经历 + Values 共同涌现，**持续演化**。

**Reflection（反思）是核心模块**，不是可选功能。它负责：总结经历 → 发现规律 → 修正价值观 → 调整目标。设计任何子系统时都要回答"它如何与 Reflection 闭环"。

## 记忆系统分层

四层不可合并：

1. **Working Memory** — 短期工作记忆
2. **Episodic Memory** — 事件记忆
3. **Semantic Memory** — 知识记忆
4. **Reflection Memory** — 反思成果

设计存储时优先考虑分层向量库 + 关系型组合，不要把所有记忆塞同一张表。

## 资源系统（重要架构约束）

```
energy / wealth / knowledge / reputation / social
```

LLM token 计费**不直接对用户暴露**。所有"消耗"必须翻译成五种世界资源。这是产品哲学也是架构约束：任何直接暴露 token / API call 计数的设计都违反核心理念。

## 演进路线（六阶段）

| Phase | 目标 |
|---|---|
| 1 | 数字宠物：状态 + 记忆 + 成长 |
| 2 | 数字人格：价值观 + 反思 + 目标 |
| 3 | 主动行为：自主目标 + 主动行动 |
| 4 | 联网生态：社交 + 世界服务 |
| 5 | 数字社会：组织 + 交易 + 合作 |
| 6 | 数字文明：群体演化 + 文化 + 文明 |

当用户提出某能力时，先识别它属于哪个 Phase，避免 Phase 3 之前就讨论 Phase 5 的社交协议细节。

## 商业模式约束（影响数据模型）

- 生命体**属于用户**，不可平台化占有 — 这意味着导出 / 迁移 / 离线运行从一开始就要在数据模型中考虑。
- 平台盈利：Runtime 服务 / 云同步 / Marketplace（技能/场景/人格包，平台抽成）/ 世界服务（学校/图书馆消耗资源）。

## 与未来 Claude 实例的协作建议

1. **白皮书是 V0.1，会演化**。在做技术决策前，确认用户当前讨论的是白皮书原版还是已迭代的新设想。
2. **用户的工作模式**：白皮书代号"心域文明"+ 显著的中文思维结构，默认用**简体中文**对话。
3. **避免过度工程**：项目处于 Phase 0（连 Phase 1 都还没开始）。不要一上来就讨论分布式架构、k8s、microservices。先把"一个数字宠物的最小生命循环"跑通。
4. **代码风格**：白皮书用 Go struct 描述领域模型 — 如果用户选 Go 作为 Runtime 语言，请遵循白皮书的命名（`Genome`、`LifeState`、`MentalState`，不要改名为 `Gene`、`State`）。
5. **新建文件前先问**：当前没有任何项目骨架。在 scaffold 任何目录结构前，用 `AskUserQuestion` 确认语言、框架、目录组织偏好。

## 设计文档导航

系统级顶层设计骨架在**根目录 `../docs/`**（本 repo 之外，V0.1 → V0.2 演进中，**章节正文未起草**，仅章节问题清单）。
**例外**：`10-risks-and-open-questions.md`（R 风险/决策日志，高频更新）留在**本 repo `docs/10-risks-and-open-questions.md`**，保住 git 历史。

**首先打开 `../docs/00-README.md`** — 它是文档地图、引用纪律、覆盖矩阵、替代方案排除理由的单一入口。

文档体系核心纪律（必须遵守，违反即设计事故）：

- **基石①** `../docs/02-glossary-and-domain-model.md` 是所有术语 / 领域模型的唯一定义源。
- **基石②** `../docs/06-resource-economics-and-ownership.md` 是所有资源 / 所有权 / 平台禁区的唯一定义源。
- 其他文档**只能回链**这两份基石，不得自行重新定义术语或宪法规则。
- 所有未决问题与盲点集中在 `docs/10-risks-and-open-questions.md`（本 repo），编号 `R01`-`R90`，其他文档需引用时用编号回链。

起草任一份文档正文前，先与用户确认章节问题清单是否仍然准确（白皮书 V0.1 可能已迭代）。起草顺序：`00 → 01 → 02 → 03 → 04 → 05/06 → 07 → 08 → 09 → 10`。

## Phase 0 工程文档

V0.2.2 已锁 Phase 0 = 作者私有 dogfooding。工程文档（**不属于宪法基石，可独立迭代**）：

- **`docs/TECH-STACK.md`** — 主语言 Go 1.26+ / 存储 SQLite + sqlite-vec / Embedding 内置 bge-m3 INT8 / LLM OpenAI 兼容协议 / 飞书 lark-oapi LongConnection / 前端 SvelteKit / Docker 多阶段构建 / 仓库结构
- **`docs/PHASE-0-PRD.md`** — 5 个子阶段（0.1 骨架 → 0.2 核心循环 → 0.3 飞书+LLM → 0.4 观察面板 → 0.5 长跑验证）+ 验收标准 + 风险缓解
- **`docs/SKILLS-AND-TOOLS.md`** — Skill 两层模型（SKILL.md 种子 vs Skill instance）+ tool registry（lane 分桶）+ Phase 0 工具集 + 依赖管理四级（L0-L3 + dangerous-skip）+ sandbox 退化版 + 网页抓取 Tier 1-3

Phase 0 不引入区块链 / 多家 IM / 主动行为 / Marketplace。这些在 Phase 1+ 演进。

工程文档与设计文档的对应映射见 `TECH-STACK §17`、`PHASE-0-PRD §11`、`SKILLS-AND-TOOLS §12`。

## 工程铁律（编码时必须遵守）

### 依赖管理（不可破）

**Go 依赖**：
- **禁止**手写 `go.mod` / `go.sum` 中的 `require` / 版本号 / sum hash
- **必须**使用命令：`go get <pkg>@<version>` + `go mod tidy`
- 例：`go get github.com/sashabaranov/go-openai@latest`

**前端依赖**（`web/` 目录）：
- **禁止**手写 `web/package.json` 中的 `dependencies` / `devDependencies` / `web/pnpm-lock.yaml`
- **必须**使用命令：`cd web && pnpm add <pkg>`
- 例：`cd web && pnpm add @sveltejs/kit`

**理由**：命令工具自动维护校验和、版本号、传递依赖、锁定文件。手写易写错包名 / 版本 / 漏 sum，且 LLM/AI 编程时尤其易错。

**例外**：仅允许手写 `module` 名、`go 1.26` 版本声明、`replace` 行、`package.json` 中 `name/version/scripts` 等非依赖元信息。

详细规则与反模式登记 `10 H01 H02` 见 `TECH-STACK §17`。
