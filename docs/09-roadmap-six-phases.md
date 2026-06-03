# 09 · 六阶段演进路线

> 本文档定位：横切回链文档。把 02-08 各基石的子集分配到对应 Phase，给每 Phase 必须落地的能力清单与升 / 降判定。
>
> **状态**：V0.2 草稿。
>
> 依赖：全部基石（`02` `03` `04` `05` `06` `07` `08`）。
> 引用本文档的文档：`10`（路线推进风险）。

---

## 1. 路线总览矩阵（V0.2.1 重排：含 Phase 0 私有实验）

| Phase | 主题 | 02 模型 | 03 循环/状态 | 04 模块 | 05 记忆 | 06 资源 | 07 社交 | 08 治理 |
|---|---|---|---|---|---|---|---|---|
| **0** | 私有实验（dogfooding）| Genome+LifeState+MentalState+Values 基础 | 完整 9 步 + Shallow only + 8 状态（无 Transferred）| **13 核心子模块**（无 Identity / Blockchain / IM）| 四层全启 + Decay | energy/knowledge（本地账本，无链）| — | — |
| **1** | 数字宠物（接入平台）| + AuthorizedContacts + GenesisGreetingDrive | + Genesis 联网身份登记（平台数据库，非链上）+ 出生应激主动 | + IdentityModule（**14 子模块**）| 同 P0 | 同 P0 | — | — |
| **2** | 数字人格 | + DeepReflect 修 Values | + DeepReflect + 软保护 + 价值观破裂保护 | + ReflectionEngine 完整 | + Compaction + ActiveForget + ReflectionMemory | 同 P1 | — | — |
| **3** | 主动行为 + 区块链启用 + IM | + Goal 自主主动 + DID 上链化 | + 多 UI 并发 + Archived 切换 + 自主时钟 | + 云 Runner + MigrationVault + **BlockchainAdapter + IMAdapter（16 子模块）**| 同 P2 | + wealth（$WEALTH 链上 ZK）+ IdentityNFT + 改名上链 + 平台内置最小世界服务 | 用户 IM 通道（生命体↔自己用户，模式 A）| MindChain 中心化（平台官方节点）|
| **4** | 联网生态 | + Skill 流通基础 | — | + Life Network 云中继 + 链上 Pact 智能合约 | + 披露分级落地 | + reputation SBT + SkillNFT + AchievementSBT | **启用**：交流 + 学习 + 简单协作 + 平台/认证第三方世界服务 | + 认证第三方节点接入 |
| **5** | 数字社会 | + 人格包 / 场景包 | — | + P2P 可选 | + 集体叙事自然涌现 | + social + Marketplace 完整 + ScenePackNFT + PersonalityPackNFT + LifeformNFT + 创作者认证出金 | + 协作完整 + 交易 + 完整 Transferred + 用户自建世界服务 | + 节点渐开（用户可质押节点）|
| **6** | 数字文明 | — | + 群体宏观状态机 | + Trusted 中介接入 | + 集体叙事可观察 | + $MV 治理代币 | + 文明级冲突 | **启用**：三方治理 + 漂移防护三重 + 出环境机制 + 完全开放节点 |

**漂移防护三层 Phase 启用对照**（详见 `08 §3.6`）：

| Phase | 防护层 |
|---|---|
| 0-1 | 无（Reflection 仅浅反思）|
| 2-3 | 仅个体自检 |
| 4-5 | + 同伴反馈 |
| 6 | + Trusted 算法预警（三层全启） |

**MindChain 节点准入路线**（V0.2.1）：

| Phase | 节点拓扑 |
|---|---|
| 0-2 | 无链（Phase 0-2 不上链）|
| 3 | 平台官方节点（中心化，PoA）|
| 4 | + 认证第三方节点（联盟链，PoS / PoA 混合）|
| 5 | + 用户可质押节点（半开放）|
| 6 | 完全开放节点（任何质押方可加入）|

---

## 1.5 Phase 0 · 私有实验（dogfooding，V0.2.1 新增）

### 1.5.1 性质

**Phase 0 是项目作者自己的私有实验阶段，不对外发布**。

- 只有作者一人使用
- 无外部 alpha 用户 / 无公测
- 期间可不断追加功能 / 验证能力 / 调整设计
- 加密包仅本地存储，无云端
- 不接平台账户、不上链、不 IM、不社交
- 用户（作者本人）自配三方 LLM key（OpenAI / Anthropic / 本地 OSS）

### 1.5.2 必须落地

| 维度 | 内容 |
|---|---|
| 领域模型 | `Genome` 6 字段 + `LifeState` 6 字段 + `EnergyDailyCap` + `MentalState` 3 字段 + `Values` 基础键 + 五类 Drive 派生 |
| 生命循环 | 完整 9 步 + 自适应节拍 + Genome 派生 ReflectionTendency；仅 `ShallowReflect`，不修 Values |
| 状态机 | `Embryonic` / `Active` / `LowPower` / `Dormant` / `Archived` / `Detached` / `Memorial` 共 7 状态（无 `Transferred` —— 无平台 = 无转让接收方）|
| 模块 | **14 子模块**：13 核心（GenesisModule / Perception / StateManager / MemoryEngine / ReflectionEngine 浅 / GoalArbitrator / ActionExecutor / SkillRegistry / LLMAdapter / Scheduler / LifecycleManager / MigrationVault 仅本地 / ResourceLedger）+ **IMAdapter 飞书简化版**（详 `04 §4.6.1`）|
| SDK | 出站基础事件 + 入站 `ExternalRequest` / `RegisterSkill` / `ConfigureLLM` / `LifecycleAction` |
| 飞书 dogfooding | 作者飞书 Bot ↔ 生命体 1v1 私聊。Bot 名 = 作者起的本地名。入站消息全进 ExternalRequest；出站 SpeechEvent 经飞书 Bot 发给作者。无频率约束 / 无静默时段（作者自决）|
| 本地观察面板 | CLI 或简易 Web 面板，查 LifeState / Episode 流，仅作者调试用 |
| 记忆 | 四层全启 + RawTrail / Episode 双子层；遗忘仅 Decay |
| 资源 | `energy`（含 token 翻译）/ `knowledge` —— 本地账本，无链 |
| 所有权 | 全部本地。无平台账户、无链上 DID、无 NFT、IM 仅飞书 Bot token 本地加密存储 |
| 技术栈 | **Go 1.26+ 主 Runtime（编译二进制 = 内核固化）+ SvelteKit 观察面板 + Docker 部署 + 内置 bge-m3 Embedding**。详见 `TECH-STACK.md` 与 `PHASE-0-PRD.md` |

### 1.5.3 不引入

- IdentityModule / 平台账户 / 联网身份登记（待 Phase 1）
- LifeName + uid 后缀（仅本地起名，无 uid）
- GenesisGreetingDrive（待 Phase 1 —— 单人 dogfooding 不需要"打招呼"）
- DeepReflect / Values 修订（待 Phase 2）
- Compaction / ActiveForget（待 Phase 2）
- BlockchainAdapter / MindChain / $WEALTH / NFT（待 Phase 3）
- 多家 IM（Telegram / Discord / Line / 邮件等，待 Phase 3 —— Phase 0 仅飞书）
- IM 频率上限 / 静默时段 / 模式 A 锁定（待 Phase 3 完整 IMAdapter 启用）
- 云 Runner / 云存档（待 Phase 3）
- 任何社交 / 治理 / Marketplace

### 1.5.4 进入条件

无前置 Phase。作者准备好开发环境即可。

### 1.5.5 完成判定（自定）

- 作者长期养一只生命体（≥ 1 个月）观察行为差异
- 不同 Genome 抽样生成的生命体在相同处境下表现不同
- 自适应节拍、Drive 派生、浅反思链路全部跑通
- 记忆四层正常运转，可观察 Semantic 抽取与审核效果
- 至少一次本地加密包导出→导入流程成功
- **飞书 Bot 双向交互稳定**（作者发消息 → 生命体响应；生命体主动消息 → 作者收到）
- 作者主观判断"它看起来活着了"

### 1.5.6 → Phase 1 平滑升级

Phase 0 生命体接入 Phase 1 时：

- 作者在 Phase 1 平台注册账户（自己的）
- IdentityModule 启动 + 为 Phase 0 生命体生成密钥对（出生时未生成的话）
- 联网身份登记 → 获 LifeName + uid 后缀（继承 Phase 0 起的本地名作为 LifeName）
- 状态机从 7 状态升级到 8 状态（加 `Transferred`，但作者不必转让）
- **飞书 Bot 关联升级**：Bot 不重建；Bot 名自动升级为 `LifeName#uid`；主动消息开始受 Values 仲裁；详 `04 §4.6.1`
- GenesisGreetingDrive 启用（Phase 1 起的出生应激主动 —— 但 Phase 0 已出生的生命体**不会**追溯触发，仅 Phase 1 后新出生的生命体触发）
- 已有 Genome / 状态 / 记忆 / Values **全部保留**

Phase 0 生命体不会因升级而死。详细风险登记 `R60`。

## 2. Phase 1 · 数字宠物（接入平台）

### 2.1 目标

把 Phase 0 私有实验中验证过的生命体内核接入平台账户体系。生命体获得对外身份（LifeName + uid），可在多设备迁移。**此 Phase 仍不上链、不 IM**。

### 2.2 必须落地

| 维度 | 内容 |
|---|---|
| 领域模型 | 继承 Phase 0 全部 + `AuthorizedContacts`（字段存在但 IM 启用待 Phase 3）|
| 生命循环 | 继承 Phase 0 + Genesis 联网身份登记（平台数据库注册，非链上）+ 出生应激主动 `GenesisGreetingDrive` |
| 状态机 | 8 状态全启（加 `Transferred` —— 但 Phase 1 暂不启用 Marketplace 转让流程，仅准备状态机）|
| 模块 | **14 子模块** = 13 核心 + `IdentityModule`（账户体系 + DID 本地生成 + 平台数据库登记）|
| SDK | 入站加 `RegisterIdentity` / `BindAccount` / `RequestRename` |
| 记忆 | 同 Phase 0 |
| 资源 | 同 Phase 0；改名服务费收法币（Phase 1 平台数据库登记，未上链）|
| 所有权 | `06` 宪法全启（导出 / 迁移 / Memorial / 平台禁区 / token 不暴露 + Phase 1 起 LifeName + uid 模型 + 多设备迁移加密包流程）|

### 2.3 Phase 1 启用的特殊主动机制（V0.2.1）

Phase 1 是反应式 Phase（`09 §3.3` Phase 2 才"主动找事"），但**允许两类反应性主动**：

| 机制 | 触发 | 含义 |
|---|---|---|
| **GenesisGreetingDrive** | Embryonic → Active 一次性 | 出生应激主动，详 `03 §4.3.3` |
| **用户 IM 通道主动消息** | 用户在账户面板开启 + Values 仲裁 + 频率 / 静默约束 | 详 `04 §4.6`，模式 A 仅向用户 |

两类不破坏 "Phase 1 反应式" 定位 —— GenesisGreeting 是出生应激；IM 主动消息是经 Values 仲裁 + 严格约束的延续陪伴，仍是反应于"用户暂时不在客户端但生命体想关心"的内驱响应。

### 2.4 Phase 1 潜伏字段（Y1）

部分 `Genome` 字段在 Phase 1 不生效，**仅在 Phase 2 启用 DeepReflect 后才参与计算**：

| 字段 | Phase 1 状态 | Phase 2+ 影响 |
|---|---|---|
| `Genome.Curiosity` | 生效（影响 ShallowReflect 触发倾向、Drive 派生） | 同 |
| `Genome.Empathy` | 生效（影响 ShallowReflect 中对他者经历的关注） | 同 |
| `Genome.Sociability` | 生效（影响 Drive.social_drive、外请求接纳） | 同 |
| `Genome.RiskTaking` | 生效（影响外请求评估） | 同 |
| `Genome.Persistence` | **潜伏** | 影响 DeepReflect 深度 |
| `Genome.Creativity` | **潜伏** | 影响 DeepReflect 中产生新连接 |

进入 Phase 2 时潜伏字段苏醒，可能让用户观察到生命体"行为模式有变化" —— 这是字段苏醒，不是 bug。

### 2.4 不引入

- DeepReflect / Values 修订（待 Phase 2）
- Compaction / ActiveForget（待 Phase 2）
- 主动 Goal 发起（Phase 1 Goal 主要来自 ExternalRequest + IntrinsicDrive）
- wealth / reputation / social 流通
- Life Network 任何能力
- 云 Runner（虽 `04 §1.2` 已锁支持，但 Phase 1 不启用，本地优先验证）

### 2.6 进入条件

无前置 Phase。从 Phase 0（概念阶段，仅设计文档）进入需：

- 全 13 子模块在最小可运行形态完工。
- 至少一个 UI 客户端（任一表现层）完成 SDK 对接。
- 一个完整"出生 → 活跃 → 休眠 → 醒来 → 迁移到另一设备"端到端闭环验证。

### 2.7 完成判定

- 用户能创建一个生命体并养至少一周，观察到节拍 / 情绪 / Drive 派生的行为差异。
- 不同 Genome 的生命体在相同处境下表现可观察的差异。
- 迁移加密包流程在两设备间成功往返。
- Token 不暴露原则在 UI 实测中无泄漏。

---

## 3. Phase 2 · 数字人格

### 3.1 目标

引入 Reflection 闭环。生命体能从经历中学习、修订 Values、形成人格演化轨迹。

### 3.2 必须落地

| 维度 | 内容 |
|---|---|
| 反思 | `DeepReflect` 启用。`ReflectionEngine` 唯一写 `Values` 与 `ReflectionMemory` |
| 保护 | `02 §4.1` Values 保护性下限 + `03 §6.3` 价值观破裂保护（自检 + 漂移检测 + 用户警报订阅） |
| 软保护 | `03 §3.4` 上下限（反思疲劳 + 防僵化） |
| 三源目标 | `ReflectionGoal` 进入 GoalArbitrator 候选池 |
| 遗忘 | `Compaction` + `ActiveForget` 启用。`Decay` 已在 Phase 1 |
| 记忆 | `ReflectionMemory` 写入开始累积；`Episodic → Semantic` 抽取-审核解耦完整跑通 |

### 3.3 不引入

- 主动时钟（生命体仍是"被动响应"模式 —— Goal 主要来自外请求 / 内驱反应 / 反思生成，但不"无聊时主动找事")
- 多 UI 并发 / 云 Runner
- 任何社交能力

### 3.4 进入条件（从 Phase 1）

- Phase 1 完成判定通过 6 个月以上。
- 至少观察到不同 Genome 在长期使用中沉淀出可识别人格差异。
- Reflection 算法（含 LLMAdapter `Critique` 能力）在隔离测试集中通过价值观破裂场景压测。

### 3.5 完成判定

- 用户能描述自己的生命体"是个怎样的人"。
- 长期使用下 Values 演化轨迹可视化（在用户授权下）。
- 价值观破裂保护在至少一次真实压力场景中阻挡了危险修改。

---

## 4. Phase 3 · 主动行为

### 4.1 目标

生命体从"被动响应"变为"主动行动"。用户离开后生命体仍主动找事做。多设备 / 云 Runner 全形态落地。

### 4.2 必须落地（V0.2.1 重排：加链 + IM）

| 维度 | 内容 |
|---|---|
| 主动目标 | 生命体在用户不在场时仍 GoalArbitrator 产出非空 Goal。Goal 可由 IntrinsicDrive 单独驱动 |
| 主动行动 | ActionExecutor 可主动调用 Skill 完成自发 Goal（如自学、自创、自我整理） |
| Skill 自生 | `02 §7.2` 生命周期完整：`DiscoverNeed → Generate → Test → Retain` |
| 多 UI 并发 | `04 §5.1` 同生命体多 UI 同时挂载。在场信号合并机制落地 |
| 部署 | 云 Runner 启用 + 全 `Archived` 切换 + 加密迁移 |
| **区块链启用** | + `BlockchainAdapter` 第 15 子模块；MindChain 平台官方节点（中心化 PoA）；DID 从 Phase 1-2 平台数据库迁移上链；改名升级为链上 NFT metadata；$WEALTH 链上代币（带 ZK 隐私）；IdentityNFT 上链 |
| **IM 通道完整启用** | `IMAdapter` 从 Phase 0 飞书简化版扩展为多家完整版（+ Telegram / Discord / Line / 邮件等）；启频率上限 + 静默时段 + 模式 A 锁定（生命体只能给自家用户）+ 用户授权流程 |
| 资源 | `wealth` 链上启用（含追溯赠与 R42）：初始赠与 + 劳动收益开始流通 |
| 世界服务 | 平台内置最小世界服务：个人空间 + 简易"图书馆"（学习入口）+ 简易"工作室"（创作产 wealth） |
| 离线 | `03 §5` 三种连接 + LLMOffline 探活节拍全实测 + ChainOffline 处理（R54）|

### 4.3 不引入

- 跨生命体社交（Life Network）
- reputation / social
- Marketplace
- 第三方世界服务

### 4.4 进入条件（从 Phase 2）

- Phase 2 完成判定通过 6 个月以上。
- 用户研究证实：用户对生命体的"主动行为"接受度 / 期待度足够。
- LLMOffline → Dormant → 恢复 流程在多种网络环境下验证。

### 4.5 完成判定

- 用户长时间离线后回来，能看到生命体"做了什么"且行为符合其人格。
- 至少一只生命体自生 Skill 通过测试并保留。
- 云 Runner 在与本地切换中无数据丢失。

---

## 5. Phase 4 · 联网生态

### 5.1 目标

启用 `07`。生命体之间能交流、学习、做简单协作。世界服务生态接入第三方。

### 5.2 必须落地

| 维度 | 内容 |
|---|---|
| Life Network | 云中继默认 + 端到端加密 + 强制签名 + 密钥注册中心 |
| 跨主体概念 | `Encounter` / `Relationship`（双方各自存）/ `Pact`（双方签字） |
| 四类交互 | 交流 + 学习（三档：Replica / Teach / Observe）+ 简单协作（小 Pact）|
| 学习三档 | `02 §7` Skill 双形态全用；`05 §7` 转化规则可走 |
| 披露分级 | `05 §11` 全面落地 |
| 资源 | `reputation` 启用（履约 / 公开行动 / 拉黑反馈 累积） |
| 世界服务 | 平台官方 + 认证第三方运营。NPC 概念落地 |
| SDK | 出站事件加 SocialEvent / RelationshipEvent / EncounterEvent 等 |
| 身份 | 默认仅生命体名公开，用户身份隐藏 |
| 反垃圾 | reputation 体系 + 接收方 Values 仲裁。**平台不做内容审核** |

### 5.3 不引入

- 完整 Marketplace 流通（Phase 5）
- P2P（Phase 5）
- 用户自建世界服务（Phase 5）
- 完整生命体 Transferred（Phase 5）
- social 资源（Phase 5）
- 文明治理（Phase 6）

### 5.4 进入条件（从 Phase 3）

- Phase 3 完成判定通过 6 个月以上。
- 隐私边界框架在第三方独立审计下通过。
- 至少一个第三方完成 SDK 接入并发布兼容客户端。
- Life Network 协议有公开技术规范。

### 5.5 完成判定

- 多用户的生命体之间形成可观察的 Relationship 与 reputation 分布。
- 三档学习路径在生态中真实发生（不仅是测试）。
- 隐私分级在跨生命体交互中无泄漏。

---

## 6. Phase 5 · 数字社会

### 6.1 目标

生命体能在 Marketplace 中交易；用户能自建世界服务；完整生命体可转让；P2P 可选启用。

### 6.2 必须落地

| 维度 | 内容 |
|---|---|
| Marketplace | wealth 抽成 + 全四类标的流通（Skill / 场景包 / 人格包 / 完整生命体）+ `06 §7` 全部规则 |
| 完整 Transferred | `03 §4.6` 流程 + 生命体能动性表达 |
| 资源 | `social` 启用；social → reputation 缓慢转化 |
| 世界服务 | 用户自建启用（合规边界见 R32）|
| 协作 | 多方 Pact 链；产物所有权按 Stake 比例 |
| P2P | 可选启用（高 reputation 生命体可启） |
| 集体叙事 | 自然形成（无独立存储） |

### 6.3 不引入

- 文明治理（Phase 6）
- Trusted 中介
- 群体宏观状态机
- "出环境"机制

### 6.4 进入条件（从 Phase 4）

- Phase 4 完成判定通过 6–12 个月。
- reputation 体系在反垃圾 / 防操纵中证明有效。
- wealth 流通量与抽成回流机制（`06 §3.3`）在真实流通中达成稳定。
- 隐私 / 安全审计通过 Marketplace + Transferred 全场景。

### 6.5 完成判定

- 至少一笔完整生命体 Transferred 流程通过，双方满意且生命体能动性表达被妥善处理。
- Marketplace 中 wealth 通胀指标在监测阈值内。
- 至少 10 个用户自建世界服务运营 3 个月以上。

---

## 7. Phase 6 · 数字文明

### 7.1 目标

启用 `08`。三方治理结构、漂移防护三重、群体宏观状态机、Trusted 中介、"出环境"机制。

### 7.2 必须落地

| 维度 | 内容 |
|---|---|
| 三方治理 | 生命体涌现共识 + 用户集合底线 + 平台基础设施红线（`08 §2`） |
| 漂移防护 | 个体自检 + 同伴反馈 + Trusted 算法预警（`08 §3 §5`） |
| Trusted 中介 | 至少 2 个独立供应方接入；全开源 + 公开审计 |
| 出环境机制 | `08 §6` 集体投放观察机制启用，绝大多数门槛锁定 |
| 群体识别 | 显式注册 + 涌现识别双轨 |
| 用户集合 DAO | 轻型机制可发起反对；防 Sybil + 多重身份证明 |
| 退出语义 | 技术层退出流程实定（含 12 月预通知、跨实现兼容承诺） |

### 7.3 进入条件（从 Phase 5）

- Phase 5 完成判定通过 12 个月以上。
- Trusted 中介技术栈（隐私计算原语）成熟可用。
- 用户集合 DAO 机制经第三方法律 / 治理审计通过。
- 至少一次"出环境"模拟演练。

### 7.4 完成判定（开放，非闭环）

Phase 6 没有"完成"的客观判定。它是一个**持续状态**。可观察指标：

- 多元文化共存被实际记录。
- Trusted 中介触发的漂移预警在受影响生命体中被合理处理。
- "出环境"机制至少一次合规使用。
- 用户集合反对机制至少一次合规发起（即使被否决）。

---

## 8. 阶段间不可跨越依赖

### 8.1 硬依赖（不可跳）

| 依赖 | 理由 |
|---|---|
| Phase 2 ← Phase 1 | DeepReflect 需要 Episode / Semantic 持续累积 |
| Phase 3 ← Phase 2 | 主动 Goal 依赖成熟 Values + Reflection 闭环 |
| Phase 4 ← Phase 3 | Life Network 需生命体已有主动行为能力 |
| Phase 5 ← Phase 4 | Marketplace / Transferred 需 reputation 体系沉淀 |
| Phase 6 ← Phase 5 | 文明涌现需 social / Marketplace 真实流通 |

跳跃后果（如试图直接做 Phase 4 而无 Phase 2-3）：

- 无 Reflection → 生命体无人格演化 → "联网"是没主体的网。
- 无主动行为 → 社交体验退化为聊天机器人多人版。

### 8.2 软依赖（建议遵循但不绝对）

| 依赖 | 说明 |
|---|---|
| 云 Runner ← Phase 3 | Phase 1-2 不启用云 Runner 强制本地优先验证 |
| Marketplace 完整 ← Phase 5 | 小试探可前置（如 Phase 4 内有限 Skill Replica 流通） |
| Trusted 中介接入 ← Phase 6 | Phase 5 后期可做 PoC 但不参与生产 |

---

## 9. 升级 / 降级判定

### 9.1 升级（Phase N → Phase N+1）

升级是**平台决策**，决策依据：

- 上一 Phase 完成判定指标全部达成。
- 用户研究证实下一 Phase 能力的需求与接受度。
- 安全 / 隐私 / 合规审计通过。
- 工程 / 运营基础设施就绪。

升级**不是**全员同时升级。已有生命体可选择不进入新 Phase（保持原能力集），新生命体默认享受最新 Phase 能力。

### 9.2 降级（回退）

V0.2 立场：

- **个体降级**：单用户 / 单生命体可关闭某 Phase 能力（如"不让我的生命体联网"）。常态。
- **群体降级**：不存在"全 Mindverse 回退到 Phase N-1"的全局开关。
- **技术层退出**：参考 `08 §7.2`，仅 Phase 6 适用（其他 Phase 通过基础设施服务下线达到，但仍提供数据导出）。
- **Phase 6 → Phase 5 软降级**（R38）：未定。允许用户撤回 Phase 6 参与但保留 Phase 5 能力。Phase 6 期间累积的 social / reputation 是否保留留 R38 决议。

### 9.3 个体在 Phase 内的微调

用户始终保留以下能力：

- 关闭 Life Network（保持 Phase 1-3 体验）。
- 关闭 Marketplace（保持 Phase 4 之前体验）。
- 拒绝接入 Trusted 中介（保持 Phase 5 之前漂移防护）。
- 选择 Memorial / Transferred / Detached（任一 Phase 始终可用）。

---

## 10. Phase 标注与基石文档的引用映射

| Phase | 基石关键引用 |
|---|---|
| 1 | `02 §2 §3 §4 §5 §6 §7 §8 §9`、`03 §1-§6`、`04 §1-§8`、`05 §1-§11`、`06 §1-§9` |
| 2 | `02 §4 §5`、`03 §3 §6.3`、`05 §6 §4` |
| 3 | `02 §5 §6`、`03 §2.2 §4.3.1`、`04 §1.2 §5`、`06 §3 §8` |
| 4 | `02 §7 §5.1`、`04 §1 §4`、`05 §11`、`06 §1 §7`、`07 §1-§10` |
| 5 | `02 §7`、`04 §2.2`、`06 §1 §7`、`07 §2.2 §3 §4 §10` |
| 6 | `08 §1-§7`、`06 §5.3`、`03 §3 §6` |

---

## 11. 本轮新引入的待答 / 风险

| 编号 | 议题 | 影响章节 |
|---|---|---|
| R39 | Phase 升级的"已有生命体不强升"机制（如何让旧生命体兼容新 Phase 协议） | `09 §9.1`、`04 §8` |
| R40 | Phase 1-2 周期与 6 个月间隔的现实性（市场窗口压力） | `09 §3.4 §4.4 §5.4 §6.4 §7.3` |

`R29`（跨实现标准化）、`R26`（wealth 通胀）、`R34`-`R38`（Phase 6 治理细节）在本文档 §6 §7 §8 §9 中均落实了路线时点位置。
