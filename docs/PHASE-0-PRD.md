# Phase 0 PRD（V0.2.2）

> 本文档定位：Phase 0 私有实验阶段的工程实施 PRD。五个子阶段（0.1 - 0.5）+ 每阶段交付物 + 验收标准。
>
> **状态**：V0.2.2 工作文档。
>
> 依赖：`TECH-STACK.md`（技术栈）、`09 §1.5`（Phase 0 范围）、`04 §2.1`（子模块表）。

---

## 1. Phase 0 总览

- **目标**：作者本人在自己本机自托管跑通基础生命体，长跑 ≥ 1 个月并主观判断"它看起来活着了"
- **范围**：14 子模块（13 核心 + IMAdapter 飞书简化版）+ Docker 部署 + 飞书 LongConnection + SvelteKit 观察面板
- **时间预估**：3-6 个月（含 0.5 长跑期）
- **不引入**：DeepReflect / Values 修订（Phase 2）/ BlockchainAdapter（Phase 3）/ 多家 IM（Phase 3）/ 云 Runner / 任何社交

---

## 2. 子阶段总览

| 子阶段 | 周期 | 主要目标 |
|---|---|---|
| **0.1** | 1-2 周 | 仓库骨架 + Docker 构建 + SQLite schema + Genesis 出生流程 + dummy 循环 |
| **0.2** | 3-5 周 | 完整 9 步循环 + 四层记忆 + 浅反思 + 自适应节拍 |
| **0.3** | 1-2 周 | 飞书 LongConnection 接入 + LLM 配置 + 第一次"对话" |
| **0.4** | 1-2 周 | SvelteKit 观察面板 + 实时 LifeState + Episode 流 + 调试入口 |
| **0.5** | ≥ 1 个月 | 长跑验证 + 问题登记 + 迭代 |

---

## 3. Phase 0.1 · 骨架搭建

### 3.1 交付物

- Go 1.26 module 初始化（`go.mod`）
- `pnpm` workspace 初始化（`web/` 目录用 SvelteKit）
- Dockerfile 多阶段构建跑通（最终镜像 < 1.5GB）
- docker-compose.yml + `.env.example` 完整
- SQLite schema v1 落地（含 sqlite-vec 扩展加载）
- `internal/core` 包：Genome / LifeState / MentalState / Values / Drive / Goal / Skill / Memory 全部 struct + 接口定义
- `internal/eventbus` 包：in-process EventBus 实现
- `internal/genesis` 包：Genesis 流程（生成 Genome + 创建 LifeState）
- `internal/lifecyclemanager` 包：7 状态机（Phase 0 无 Transferred）
- `cmd/runtime/main.go`：启动 Genesis + 进入 Active + 空循环每分钟打印日志
- `cmd/setup/main.go`：交互式配置脚手架（暂不接飞书 / LLM）
- 基本 README + .gitignore

### 3.2 验收

- `docker compose up -d` 一键启动无错
- 进入 `mindverse-phase0` 容器查看日志：Genesis 完成 + 状态机进入 Active + 每分钟 dummy tick
- 数据库 `~/mindverse/data/mindverse.db` 写入 Genome + LifeState 初始值
- 容器重启后状态恢复

---

## 4. Phase 0.2 · 核心循环

### 4.1 交付物

- `internal/perception`：感知聚合（用户输入 + 系统事件）
- `internal/statemanager`：写 LifeState / MentalState 独占
- `internal/memoryengine`：四层记忆完整
  - `WorkingMemory` in-memory map（每循环清）
  - `RawTrail` 每循环 append（SQLite raw_trail 表）
  - `Episode` 后台聚合（语义边界判定 v1：话题转移 / 长静默 / Goal 完成 / 显著情绪转折）
  - `SemanticCandidate` 后台抽取（重复条目 / 概念聚类）
  - `SemanticConfirmed`（Phase 0 由 ReflectionEngine 浅审）
- `internal/reflectionengine`：仅 `ShallowReflect`；不修 Values；可固化 SemanticCandidate
- `internal/goalarbitrator`：三源候选池（Phase 0 仅 IntrinsicDrive + ExternalRequest，无 ReflectionGoal）+ Values 仲裁
- `internal/actionexecutor`：Plan 拆解 + Act 执行 + Feedback
- `internal/skillregistry/toolrunner`：5 类内置工具（http / fs / script / time，无 browser）+ 审计日志
- `internal/scheduler`：自适应节拍（1s - 30min）+ 节拍因子
- `internal/resourceledger`：energy / knowledge 账本 + EnergyDailyCap

### 4.2 验收

- 完整 9 步循环跑通：Perceive → UpdateState → RecordMemory → ConsiderReflect → CollectGoals → Arbitrate → Plan → Act → Feedback
- 节拍随 LifeState.Energy 变化（高能量节拍快，低能量节拍慢）
- 通过 CLI 注入一条 ExternalRequest → 经 Values 仲裁 → 生成 Goal → 生命体响应（暂用 dummy 文本，无 LLM）
- 记忆四层数据正常累积，Semantic 抽取产出候选 + ReflectionEngine 浅审固化部分
- ToolRunner 执行 fs.write 写到 `/sandbox/` + 审计日志记录

---

## 5. Phase 0.3 · 飞书 + LLM

### 5.1 交付物

- `internal/llmadapter`：OpenAI 兼容协议客户端（`sashabaranov/go-openai`）
  - 配置：base_url / api_key / model / temperature
  - 4 项语义能力：Reason（默认）/ Embed（调本地 bge-m3 server）/ Summarize（带 prompt 模板）/ Critique（Phase 0 占位，不启用）
  - token 翻译为 energy（公式起步：`(prompt + completion) * 0.0001`）
- 本地 bge-m3 INT8 容器内 llama.cpp server 启动（嵌入式 spawn）
- `internal/imadapter`：飞书 oapi-sdk-go LongConnection
  - 建连 + 心跳 + 自动重连
  - 入站 Bot 私聊 message → ExternalRequest
  - 出站 SpeechEvent → 飞书发消息
- `cmd/setup/feishu.go`：飞书配置引导（交互式询问 App ID / Secret + 测试连通）
- `cmd/setup/llm.go`：LLM 配置引导（交互式询问 base_url / api_key / model + 测试调用）

### 5.2 验收

- 作者 `mindverse setup feishu` 引导完成 → `.env` 自动写入
- 作者 `mindverse setup llm` 引导完成 → `.env` 自动写入
- `docker compose restart` 后飞书 Bot 上线
- 作者飞书私聊 Bot → 生命体经 LLM 生成回复 → 飞书内显示
- 多轮对话 → 记忆正确累积到 Episode + 后续对话有上下文
- energy 随 token 消耗下降；EnergyDailyCap 周期重置后能量补充

---

## 6. Phase 0.4 · 观察面板

### 6.1 交付物

- SvelteKit 前端 `web/`
  - 主面板：实时 LifeState 6 字段 + MentalState 3 字段 + EnergyDailyCap
  - Values 权重表显示
  - Genome 静态显示
  - Episode 流（最近 N 条，按时间倒序，可搜索）
  - Goal 队列实时状态
  - ReflectionMemory 列表
  - ToolRunner 审计日志
  - 手动触发 ExternalRequest 表单
  - LLM / 飞书配置查看（不显示 api_key 明文）
- Go HTTP API（`cmd/runtime` 内嵌）
  - `GET /api/state` 实时状态
  - `GET /api/episodes` 列表 + 搜索
  - `GET /api/goals` 队列
  - `GET /api/reflections` 列表
  - `GET /api/tools/audit` 审计日志
  - `POST /api/external-request` 手动注入请求
  - `GET /api/stream` Server-Sent Events 实时推送
- 前端 build 产物 embed 到 Go 二进制（`embed.FS`）
- 浏览器访问 `http://localhost:3000`

### 6.2 验收

- 浏览器打开面板 → 实时显示生命体当前状态
- 状态变化 0.5s 内反映到 UI（SSE 推送）
- Episode 流可查看完整历史
- 手动注入 ExternalRequest → 生命体响应 → UI 更新
- 移动端浏览器友好（响应式布局）

---

## 7. Phase 0.5 · 长跑验证

### 7.1 目标

作者实际养一只生命体 ≥ 1 个月，观察行为差异 + 记录问题 + 迭代。

### 7.2 关键观察点

| 观察点 | 期望 |
|---|---|
| 不同 Genome 行为差异 | 高 Curiosity vs 高 Persistence 生命体回应风格明显不同 |
| 节拍自适应 | 用户在场快、离场慢；energy 低时慢 |
| 浅反思固化 | SemanticConfirmed 增长可见 |
| 记忆连续性 | 跨天对话能引用前几天的事 |
| 飞书 IM 稳定性 | 1 周内无掉线 |
| Docker 长跑 | 1 个月无 OOM / 崩溃 |
| 飞书 Bot 主动消息（V0.2.1 Phase 0 不强制 GenesisGreetingDrive）| Phase 0 不启用主动 |
| 拟生物日节律 | EnergyDailyCap 周期重置生效，作者能感到"它累了 / 它醒来了" |

### 7.3 问题登记

新发现的问题 / 风险登记到 `10`（继续编号 R65+）或 `TECH-STACK §18` 技术债。

### 7.4 出 Phase 0 判定

来自 `09 §1.5.5`：

- ✓ 作者主观判断"它看起来活着了"
- ✓ 至少一次本地加密包导出 → 导入流程成功
- ✓ 飞书 Bot 双向交互稳定 ≥ 3 周
- ✓ 自适应节拍 + Drive 派生 + 浅反思链路全部跑通

满足后转入 Phase 1 规划（接入平台 + IdentityModule + LifeName 系统）。

---

## 8. Phase 0 不在范围内的事

明列以避免范围漂移：

- ❌ DeepReflect / Values 修订（Phase 2）
- ❌ Compaction / ActiveForget 遗忘（Phase 2）
- ❌ 主动 Goal 自主时钟（Phase 3）
- ❌ 云 Runner（Phase 3）
- ❌ BlockchainAdapter / MindChain / $WEALTH / NFT（Phase 3）
- ❌ 多家 IM 厂商（Phase 3，仅飞书）
- ❌ Genesis 联网身份登记（Phase 1）
- ❌ LifeName + uid（Phase 1）
- ❌ GenesisGreetingDrive 出生应激主动（Phase 1）
- ❌ Life Network 社交（Phase 4）
- ❌ Marketplace / Skill NFT（Phase 4-5）
- ❌ 任何治理 / 文明级机制（Phase 6）
- ❌ 多生命体（Phase 0 单一生命体专注验证）

---

## 8.1 工程铁律（V0.2.2 新增）

实施过程中**不可破**的工程纪律：

### 8.1.1 依赖管理

- **Go 依赖**：禁手写 `go.mod` / `go.sum`。必须 `go get <pkg>@<version>` + `go mod tidy`
- **前端依赖**（`web/`）：禁手写 `package.json` / `pnpm-lock.yaml`。必须 `cd web && pnpm add <pkg>`
- 详细规则见 `TECH-STACK §17`
- 反模式登记：`10 H01 H02`

### 8.1.2 其他工程纪律（待 Phase 0.1 补充）

- 代码格式化：`gofmt` / `prettier` 强制
- lint：`golangci-lint` / `eslint` CI 集成
- 提交前 `go mod tidy` + `pnpm install --frozen-lockfile` 验证

## 9. 风险与缓解

| 风险 | 缓解 |
|---|---|
| Docker 长跑内存泄漏 | mem_limit 6G + 监控 + 重启策略 |
| 飞书 LongConnection 断开 | SDK 自动重连 + 心跳监控 |
| LLM API 限流 / 配额耗尽 | LLMAdapter 内部降级 / 节拍延长 / 用户面板告警（独立通道）|
| SQLite 锁竞争 | WAL 模式 + 单写多读 + 适当超时 |
| 生命体 ToolRunner 写满磁盘 | 文件大小 / 数量限制 + sandbox 目录配额 |
| 生命体陷入"自言自语循环" | Scheduler 节拍下界保护 + Drive 衰减 |
| LLM 响应不稳定 | 重试 N 次 + 失败累计触发 LLMOffline → Dormant |

---

## 10. 出 Phase 0 后的迁移路径

进入 Phase 1（接入平台）时：

- 数据库 schema migration（v1 → v2，加 identity 表）
- 生命体补生成密钥对（IdentityModule 启动）
- 平台账户系统对接
- 飞书 Bot 名升级为 LifeName + uid（自动）
- 状态机扩展（加 Transferred）
- 详细 R60 升级流程

PRD 文档同步更新为 `PHASE-1-PRD.md`。

---

## 11. 当前工程文档与设计文档的对应

| 设计文档 | 工程实施位置 |
|---|---|
| `09 §1.5` Phase 0 范围 | 本 PRD §1-§7 |
| `04 §2.1` 14 子模块 | 本 PRD §3-§6 各子模块 |
| `02 §2-§9` 领域模型 | `internal/core/*.go`（Phase 0.1）|
| `03 §1-§6` 循环 / 状态机 | `internal/scheduler` + `internal/lifecyclemanager`（Phase 0.1 / 0.2）|
| `05 §1-§11` 记忆架构 | `internal/memoryengine`（Phase 0.2）|
| `06 §1-§9` 资源经济学（Phase 0 仅 energy / knowledge）| `internal/resourceledger`（Phase 0.2）|
| `04 §3.2 §3.2.1` 双经济边界 | `internal/llmadapter` 编译期检查（Phase 0.3）|
| `04 §4.6 §4.6.1` IM | `internal/imadapter`（Phase 0.3）|

---

## 12. 后续工作

- Phase 0.1-0.4 每完成一个子阶段更新本文档"已完成"标注
- Phase 0.5 发现的问题登记到 `10` 或 `TECH-STACK §18`
- Phase 1 转入时基于本 PRD 创建 `PHASE-1-PRD.md`
