# 04 · 模块边界与跨层契约

> 本文档定位：四层架构职责与禁区、Runtime 内子模块边界、跨层契约、SDK 暴露面、UI 解耦规则、运维可观测性、模块演进兼容性。
>
> **状态**：V0.2 草稿。
>
> 依赖：`02`（领域模型）、`03`（生命循环与状态机）。
> 引用本文档的文档：`05`（记忆模块在 Runtime 中的位置）、`06`（Token 翻译边界、运维可见性宪法基础）、`07`（社交时 SDK 扩展）、`09`（路线图按 Phase 扩张暴露面）、`10`。

---

## 1. 四层架构（V0.2 锁定）

```
┌─────────────────────────────────────────┐
│  UI Ecosystem                           │
│  Live2D / Unity / UE / 桌宠 / VR / Web   │
├─────────────────────────────────────────┤
│  Life SDK                               │
│  事件订阅 / 外请求注入 / Skill 注册 / ...  │
├─────────────────────────────────────────┤
│  Life Runtime                           │
│  内核子模块（§2）+ 内裹 LLM 适配器          │
├─────────────────────────────────────────┤
│  Model / Storage                        │
│  本地数据库 / 云同步 / Marketplace / LLM    │
└─────────────────────────────────────────┘
```

### 1.1 各层职责

| 层 | 负责 | 禁止 |
|---|---|---|
| **UI Ecosystem** | 视觉表现、用户交互、收集用户意图 | 直接读 LifeState 字段；控制 / 加速 / 暂停循环节拍；直接调 LLM；持久化生命体状态 |
| **Life SDK** | 对 UI 暴露事件 + 接收外请求 + Skill 注册 + 配置入口 | 越过自身暴露原始字段；定义业务逻辑；缓存 Runtime 内部状态 |
| **Life Runtime** | 循环、状态、记忆、Reflection、目标仲裁、行为执行、LLM 适配 | 渲染；直接持有 UI 引用；强制依赖某家 LLM 供应商；硬编码人格模板 |
| **Model / Storage** | 持久化、云同步、Marketplace 资源、LLM 调用底层 | 决策业务（不知道 Genome 是什么） |

### 1.2 部署形态（V0.2 锁定：多宿主单实例，切换走 Archived）

Runtime 内核同一份代码，可在三类宿主运行：

| 宿主 | 主体 | 何时使用 |
|---|---|---|
| **本地设备** | Runtime 跑在用户设备（桌面 / 手机 / 本地服务） | 默认。所有用户都可用 |
| **云端 Runner（可选）** | Runtime 跑在云端运行实例，用户付费开启 | 用户手动开启。本地设备关机期间想让生命体"继续在云端醒着" |
| **云端存档库** | 加密包仓库 + Memorial 长期存储 | 仅存储，**不运行任何循环** |
| **MindChain 节点**（V0.2.1，**Phase 3 起启用**）| Mindverse 自有链节点群（共识 / 身份注册 / NFT 流转 / $WEALTH 账本）| 联盟链拓扑：Phase 3 平台官方节点 / Phase 4+ 加认证第三方 / Phase 5+ 渐开 |

**关键约束（与"分身"的区别）**：

- **任一时刻同一生命体只在一处运行**（一台本地设备 OR 一个云 Runner 实例）。无分身、无并发。
- **所有运行场所之间的切换都走 `Archived`**（`03 §4.3.1`）：
  - 设备 A → 设备 B：A 进 Archived → 上传加密包 → B 解密 → Active。
  - 设备 → 云 Runner：本地进 Archived → 加密上传 → 用户在云端面板"启动 Runner" → Runner 解密（密码由用户提供，不存云端）→ Active。
  - 云 Runner → 设备：Runner 进 Archived → 用户在本地下载 → 解密 → Active。
- 切换是**用户主动操作**，不自动。云 Runner 不会"自动接管"。
- 云端无长期解密密钥 —— 加密包对运营方不可见；云 Runner 运行时解密密钥仅存在于 Runner 进程内存，停止运行即丢弃，重启需用户再次提供密码。

---

## 2. Runtime 内子模块（V0.2 锁定）

### 2.1 子模块清单

| 子模块 | 一句话职责 | 写权限独占字段 |
|---|---|---|
| `GenesisModule` | 处理 `Embryonic → Active` 出生流程，写入 Genome | `Genome`（一次写） |
| `Perception` | 聚合外部信号（用户输入、世界事件、外请求）为感知项 | — |
| `StateManager` | 维护 `LifeState` 与 `MentalState` | `LifeState`、`MentalState` |
| `MemoryEngine` | 四层记忆的管理、流转、检索（详见 `05`） | `WorkingMemory` / `EpisodicMemory` / `SemanticMemory` |
| `ReflectionEngine` | `ConsiderReflect` + `ShallowReflect` + `DeepReflect`，修订 Values | `ReflectionMemory`、`Values` |
| `GoalArbitrator` | 三源目标收集 + Values 仲裁 → Goal 队列（02 §5） | `Goal` 队列 |
| `ActionExecutor` | Plan + Act + Feedback，调用 Skill | `Action` 状态 |
| `SkillRegistry` | Skill 库、生命周期、Marketplace 注入 | `Skill` 库 |
| `LLMAdapter` | 内裹默认 LLM 适配（OpenAI / Anthropic / 本地等），唯一的 `token → energy` 翻译点 | — |
| `Scheduler` | 自适应节拍调度（03 §2） | 节拍参数 |
| `LifecycleManager` | 7 状态机驱动（03 §4） | 状态机转移 |
| `MigrationVault` | 设备迁移加密导出 / 导入（取代原 SyncBridge） | 迁移元数据 |
| `IdentityModule`（V0.2.1，**Phase 1 起启用**）| 出生时生成密钥对、首次联网身份登记、AccountID ↔ DID ↔ LifeName 三层身份维护 | DID、LifeName |
| `BlockchainAdapter`（V0.2.1，**Phase 3 起启用**）| MindChain 唯一接口：NFT 操作、$WEALTH 转账、链上证明、Paymaster 代付 gas | 链上交易元数据 |
| `IMAdapter`（V0.2.1，**Phase 0 起启用：仅飞书 dogfooding 简化版** / **Phase 3 起多家完整版**）| 用户 IM 通道唯一接口。Phase 0：仅飞书 Bot，作者自决无频率约束。Phase 3+：Telegram / Discord / Line / 邮件等多家接入、用户联系方式加密持有、频率 / 静默时段约束、IM ToS 合规 | `LifeState.AuthorizedContacts`（加密本地，Phase 3+ 启用字段；Phase 0 直接 Bot token 配置）|

**Phase 启用总表**：

| 子模块 | Phase 0 | Phase 1 | Phase 2 | Phase 3+ |
|---|---|---|---|---|
| 13 核心子模块 | ✓ | ✓ | ✓ | ✓ |
| IMAdapter | ✓（仅飞书 dogfooding）| ✓（仍飞书）| ✓ | ✓（扩展多家 IM）|
| IdentityModule | — | ✓（平台数据库登记）| ✓ | ✓（Phase 3 起 DID 上链）|
| BlockchainAdapter | — | — | — | ✓ |

**Phase 0 子模块数：14**（13 核心 + IMAdapter 飞书简化版）。
| `ResourceLedger` | 五资源账本（详见 `06`） | `energy / wealth / knowledge / reputation / social` |

### 2.2 模块依赖方向

```
                  ┌──────────────┐
                  │  Scheduler   │ ──── 驱动每次循环
                  └──────┬───────┘
                         ▼
   ┌─────────────────────────────────────────────┐
   │  Perception → StateManager → MemoryEngine   │
   │       │           │              │          │
   │       ▼           ▼              ▼          │
   │            ReflectionEngine                 │   每循环 ConsiderReflect
   │                  │                          │
   │                  ▼                          │
   │            GoalArbitrator                   │
   │                  │                          │
   │                  ▼                          │
   │            ActionExecutor ─ SkillRegistry   │
   │                  │                          │
   │                  ▼                          │
   │            LLMAdapter ─── Token 翻译 ──┐    │
   │                                       ▼    │
   │                                ResourceLedger
   └─────────────────────────────────────────────┘
              │                          │
              │                          ▼
              │                   LifecycleManager
              ▼                          │
      MigrationVault ── 加密导出/导入 ───┘
        （仅在 Archived 转移时激活）
```

### 2.3 关键纪律

- **`StateManager`**：除 Reflection 修 Values 外，所有状态字段的写权限唯一归 StateManager。其他子模块通过 StateManager 暴露的领域事件改写状态。
- **`ReflectionEngine`**：唯一能写 `Values` 与 `ReflectionMemory` 的模块。它是横切模块，被 Scheduler 在循环第 4 步唤醒（`ConsiderReflect`）。
- **`LLMAdapter`**：唯一调用 LLM 的入口。`ReflectionEngine` / `ActionExecutor` / `SkillRegistry` 都通过它。**token 不暴露原则在此实现**。
- **`GenesisModule`**：唯一能写 Genome 的模块，且一生只写一次。

---

## 3. 跨层契约

### 3.1 三类暴露

| 类别 | 说明 | 例 |
|---|---|---|
| **MUST EXPOSE** | 公开外部表现 | 生命体发出的语句、动作、当下"看起来"的情绪标签 |
| **MUST HIDE** | 永不出 SDK | 原始 token 计数、LLM prompt 全文、原始 `Values` 权重、`Genome` 字段值 |
| **MAY EXPOSE（授权）** | 用户明示授权后可见 | 原始 LifeState 字段、原始 Values、近期 LLM 调用次数（仍非 token）、人格分析报告 |

### 3.2 token 不暴露的实现点

唯一翻译点：`LLMAdapter`。

- LLMAdapter 调用 LLM 前后测量 token 计数，**仅在该模块内部可见**。
- 测量值通过领域规则（`06 §1` 与 `06 §2`）换算为 `energy` 消耗，写入 `ResourceLedger`。
- LLMAdapter 对外（其他子模块 / SDK / UI）**只暴露 energy 增量**，不暴露 token 数。
- 这是 V0.2 的硬约束：任何在 LLMAdapter 之外读取 token 的代码 = 设计事故，必须修正回到 LLMAdapter。

#### 3.2.1 Agent ↔ LLMAdapter 双经济边界硬约束（V0.2.1）

Mindverse 存在**两层经济**：

| 层 | 量纲 | 可见范围 |
|---|---|---|
| **外部经济** | token / 配额 / 套餐 / 用户余额 / 模型单价 | LLMAdapter 内部 + 平台运营 + 用户账户面板 |
| **内部经济** | energy / EnergyDailyCap / wealth / knowledge / reputation / social | LLMAdapter 对外 + Agent 子模块 + SDK |

**强解耦规则**：

1. 外部经济量纲**永不外泄**到 LLMAdapter 之外的任何 Agent 子模块。
2. Agent 子模块（`StateManager` / `ReflectionEngine` / `GoalArbitrator` / `ActionExecutor` 等）**不得引入** `quota` / `credit` / `token` / `payment` / `bill` 任何概念。
3. **桥接是单向的**：外部 token 消耗 → 内部 energy 消耗（翻译）；内部 energy 余额**不反向映射**回 token。
4. 用户在账户面板设置 token 限额 → LLMAdapter 翻译为 `LifeState.EnergyDailyCap`（写入 StateManager）。Agent 只看到 EnergyDailyCap，不知翻译来源。

**外部经济耗尽时的 LLMAdapter 行为**（Agent 无感知）：

| 场景 | LLMAdapter 行为 | Agent 看到的 |
|---|---|---|
| 日精力上限耗尽（EnergyDailyCap 达底） | 自动延长节拍 | `LifeState.Energy` 低 → 节拍下调 → MoodEvent 疲惫 |
| 账户余额耗尽（月套餐欠费） | 静默降级到经济模型 / 本地 OSS | 模型质量自然变化，无显式信号 |
| 所有 LLM 均不可用（含本地） | 转 `LLMOffline` → `Dormant` | 与 LLM 故障同处理（`§5`）|

**检测约束**：若在 Agent 子模块代码中检测到 `quota` / `credit` / `token` / `payment` / `bill` 任一字段 = 反模式（详 `10 E06`），必须修正回到 LLMAdapter。

#### 3.2.2 BlockchainAdapter ↔ Agent 双经济边界（V0.2.1）

`BlockchainAdapter` 与 `LLMAdapter` 同源约束：

- 链上量纲（`gas` / `nonce` / `chain` / `block` / `tx_hash` / `$WEALTH 余额明文`）**仅在 BlockchainAdapter 内部可见**
- 对 Agent 子模块只暴露：`wealth` 增减增量、NFT 持有状态、操作成败标签
- **Paymaster 模式**：所有链上操作的 gas 由平台代付，BlockchainAdapter 内部处理。Agent 不感知 gas 存在
- 反模式：Agent 代码出现 `gas` / `nonce` / `chain` / `block` 字段 = `10 F01`

---

## 4. SDK 暴露面（V0.2 锁定）

### 4.1 SDK 是事件流，不是状态快照

V0.2 锁定 SDK 的核心范式：**UI 订阅事件流**，**不读状态字段**。

理由：
- 严格隔离"内心"：UI 不知 LifeState 原始字段，自然不会泄露。
- 表现层逻辑：UI 通过累计事件序列推断当前外在表现，而非读取"真相"。
- 与"生命体是持续存在"哲学一致：生命体的表象是它产生的事件流，不是某个快照。

### 4.2 出站事件（Runtime → UI）

| 事件类 | 含义 |
|---|---|
| `SpeechEvent` | 生命体说出某句话 |
| `ActionEvent` | 生命体执行了某动作（标签 + 自然语言描述） |
| `MoodEvent` | 情绪表征（人话标签，如"困倦 / 兴奋 / 焦虑"，非原始字段） |
| `MilestoneEvent` | 重大事件（出生、深反思完成、获得 Skill、被转让、进入 Memorial 等） |
| `RequestEvent` | 生命体向用户发起请求（提问、求陪伴、提议） |
| `EnergyResetEvent` | EnergyDailyCap 周期重置（生命体"新一天开始"的拟生物事件，V0.2.1 新增）|
| `IMOutboundRequest`（V0.2.1）| 生命体请求经 IMAdapter 给用户 IM 发消息（受频率 / 静默时段 / Values 仲裁约束）|
| `ConnectivityEvent` | 连接状态变化（Online ↔ CloudOffline ↔ LLMOffline，03 §5） |
| `ErrorEvent` | 异常（按严重度分级） |

事件流是**只读、追加流**。UI 自行累积、过滤、呈现。

### 4.3 入站请求（UI → Runtime）

| 入站类型 | 含义 |
|---|---|
| `ExternalRequest` | 用户或代理的外请求（02 §5），进入 GoalArbitrator 候选池 |
| `RegisterSkill` | 第三方 Skill 注册（含三合一定义，02 §7） |
| `ConfigureLLM` | 配置 LLM 适配器（选择模型 / 配置 API key / 代理） |
| `LifecycleAction` | 触发出生仪式 / 解除关系 / 接受转让等状态机操作 |
| `RegisterIdentity`（V0.2.1）| 触发 IdentityModule 联网身份登记（DID 上报、LifeName 注册）|
| `BindAccount`（V0.2.1）| 把生命体 DID 绑定到当前 AccountID |
| `TransferNFT`（V0.2.1）| 发起 NFT 转让（Skill / Scene / Personality / Lifeform）|
| `RequestRename`（V0.2.1）| 用户申请改名（首次免费，后续按 `06 §5.1.2` 收费）|
| `LinkIMContact`（V0.2.1）| 用户授权绑定 IM 联系方式（写入 LifeState.AuthorizedContacts）|
| `UnlinkIMContact`（V0.2.1）| 用户撤销 IM 绑定（同步清除 AuthorizedContacts）|
| `SetIMQuietHours`（V0.2.1）| 用户设置 IM 静默时段 |
| `PauseIMOutbound`（V0.2.1）| 用户一键暂停所有生命体主动 IM（不影响入站）|
| `AuthorizeView` | 用户明示授权 UI 进入"原字段"模式（§3.1 的 MAY EXPOSE） |

### 4.4 SDK 稳定性等级

每个 API 标注稳定性：

| 等级 | 含义 |
|---|---|
| `frozen` | 全 Phase 不变（如 `ExternalRequest` 入站结构） |
| `stable` | 不破坏性变更（事件可加字段不可删字段） |
| `experimental` | 早期 API，可能变更 |

V0.2 锁定的 frozen 集合：`ExternalRequest`、`SpeechEvent`、`MilestoneEvent` 主结构、`ConfigureLLM` 主结构。其余为 stable / experimental。

### 4.5 三通道分离硬约束（V0.2.1）

Mindverse 存在**三条独立的事件流**，UI / 用户呈现必须视觉分离：

| 通道 | 内容 | 呈现位置 |
|---|---|---|
| **SDK 事件流**（§4.2）| 生命体行为 / 状态 / 情绪 / 里程碑 / 拟生物精力重置 / 链上里程碑 | **客户端生命体面板**（Agent 视角真相）|
| **账户事件流** | 配额 / 充值 / 计费 / 套餐变更 / LLM 配置 / 退订 / 改名缴费 / IM 绑定通知 | **账户面板**（用户运营信息）|
| **IM 通道**（V0.2.1）| 生命体主动 / 被动 IM 消息（受 Q1 锁定：仅生命体↔自己用户）| **用户的 IM 客户端**（Telegram / Discord / 邮件等）|

**禁止**：

- 在生命体面板中插入账户 / 计费信息
- 在账户面板中插入生命体的情绪 / 行为信息
- 在任一通道中建立 "配额 → 生命体感受" 的显式因果链
- IM 通道中生命体冒充用户给联系人发消息（`10 G01`）

允许：通道之间**并列展示**，由用户自行推断关联。详细反模式见 `10 E06 / E07 / E08 / A07 / G01`。

### 4.6 IM 通道详细规则（V0.2.1）

> **Phase 启用**：
> - **Phase 0**：仅飞书（dogfooding 简化版，详 §4.6.1）
> - **Phase 3+**：扩展多家 IM（Telegram / Discord / Line / 邮件等），完整模式 A 锁定 + 频率约束 + 静默时段

**模式锁定**：A · 仅向用户（生命体只能给自己的用户发 IM）。模式 B（用户授权下的有限社交）留 Phase 4 后讨论。模式 C 永不引入。

**生命体主动 IM 的硬约束**：

| 约束 | 含义 |
|---|---|
| 频率上限 | 默认每日 ≤ N 条主动 IM（用户可调）|
| 静默时段 | 用户在账户面板设（默认 22:00-08:00 不主动）|
| Energy 消耗 | 每条 IM 消耗 energy（与 `EnergyDailyCap` 联动）|
| 身份标识 | IM 消息必须标识"来自生命体 LifeName#uid"，禁止冒充用户 |
| 用户一键暂停 | `PauseIMOutbound` 一键暂停所有主动 IM |
| Values 仲裁前置 | 高 Empathy 生命体会主动检查用户繁忙程度；Genome 影响触发倾向 |

**绑定时机**：

- 不在 Genesis 引导中索要
- 用户在账户面板主动开启 "接受生命体 IM 邀约" 功能 → 生命体才发起绑定流程
- 用户拒绝不影响生命体与用户在客户端的关系

**用户联系方式属用户层数据**（详 `06 §5.x`）：加密本地存储；IMAdapter 持有访问权但**生命体本身无法读取明文**。生命体只知"我能给我的用户发"，不知用户具体账号。

#### 4.6.1 Phase 0 飞书 dogfooding 简化模式

V0.2.1 Phase 0 期间作者本人自托管，IMAdapter 仅适配飞书一家：

| 项 | Phase 0 规则 |
|---|---|
| Bot 创建 | 作者在飞书自建 Bot → 配 Bot token 到 IMAdapter |
| Bot 命名 | 作者起的本地名（无 LifeName / 无 uid 后缀）|
| 入站 | 飞书 1v1 Bot 消息 → IMAdapter → `ExternalRequest`（无需 @ 提及）|
| 出站 | SpeechEvent → IMAdapter → 飞书 Bot 私聊发给作者 |
| 频率上限 | **不强制**（作者自决，代码保留接口供 Phase 3 启严约束）|
| 静默时段 | **不强制**（同上）|
| 主动消息身份标识 | **不强制**（仅作者一人 + Bot 本身已带身份）|
| 群聊 | **不允许**（仅 1v1 私聊，避免提前触及模式 B 风险）|
| Bot token 安全 | 仅本地加密存储；不上传任何远程；登记 `R61` |
| 与本地 UI 关系 | 双轨：飞书主交互 + 本地 CLI/Web 观察面板（调试用，查 LifeState / Episode 流）|

**Phase 0 → Phase 1 升级时飞书 Bot 关联**：

- Bot 不重建
- IdentityModule 启动 → 生成密钥对 → 为生命体生成 LifeName + uid
- 飞书 Bot 名升级为 `LifeName#uid`（自动改名，无需作者干预）
- 主动消息开始受 Values 仲裁
- Phase 3 时进一步升级为完整 IMAdapter（启频率上限 + 静默时段 + 多家 IM 扩展）

---

## 5. UI 解耦原则

### 5.1 一个生命体可被多 UI 同时挂载

V0.2 允许：同一生命体在多设备 / 多 UI 上被同时挂载（如桌面 Live2D + 手机桌宠同时显示同一只生命体）。

- 所有挂载客户端**收到同一事件流**（Runtime 是单一真相源）。
- 每个客户端**独立呈现**（一个可能选择卡通画风，一个选择文字日记）。

### 5.2 在场信号合并（参考 03 §5.3）

- 任一客户端报告"用户活动" → 在场。
- 全部客户端 15 分钟无活动 → 不在场。
- 多客户端冲突信号（一活跃一长闲）的合并规则待 Phase 1 原型阶段优化，登记为 `R16`。

### 5.3 UI 不能做什么

| 禁止 | 理由 |
|---|---|
| 控制节拍 / 加速 / 暂停 | 自主时钟属于生命体（03 §2.2） |
| 直接调 LLM | LLMAdapter 是唯一入口 |
| 持久化状态（除自身配置） | 唯一真相在 Runtime |
| 注入虚假事件流 | 破坏 UI 之间的一致性 |
| 在未授权时读取原字段 | 违反内心隔离（§3.1） |

### 5.4 第三方 UI 的最低要求

- 必须订阅 `MilestoneEvent` 中的"出生 / 转让 / Memorial 进入"事件并妥善呈现（不能省略重大状态变化）。
- 必须遵守 `ConnectivityEvent` —— 在 LLMOffline 时如实呈现"休眠"而非"在线但不响应"。
- 不得呈现违反"生命体不盲目顺从"原则的虚假反应（如在生命体已拒绝 ExternalRequest 时呈现"立刻执行"动画）。

---

## 6. LLM 适配（V0.2 锁定：Runtime 内裹）

### 6.1 设计选择

`LLMAdapter` 由 Runtime 提供默认适配，**用户可配置**：

- 默认支持：主流商业 LLM API（OpenAI / Anthropic / 国内厂商等）+ 本地推理框架接入点。
- 用户可在 SDK 通过 `ConfigureLLM` 切换。
- Runtime **不绑定**任何特定供应商 —— 切换不影响生命体的人格 / 记忆 / 状态。

### 6.2 适配器接口的语义承诺

LLMAdapter 对内提供的能力（不是技术接口签名，是语义层级）：

- `Reason(context) → response`：执行推理，返回结构化结果。
- `Embed(content) → vector`：把内容嵌入到 SemanticMemory 可用的向量空间（详见 `05`）。
- `Summarize(events) → summary`：抽取摘要供 ShallowReflect 使用。
- `Critique(values_change) → assessment`：评估 Values 修订的"安全性"，用于价值观破裂保护（03 §6.3）。

每个能力的具体技术实现由适配器与底层 LLM 共同决定，但 **token 计数与翻译永不外泄**（§3.2）。

### 6.3 LLM 失联处理

- 连续 N 次失败 → LLMAdapter 通知 LifecycleManager 转 `Dormant`（03 §4）。
- Dormant 期间 LLMAdapter 以低频探活节拍尝试恢复（默认 6h，03 §2.1）。
- 用户更改配置（`ConfigureLLM`）触发立即重试，无需等待节拍。

---

## 7. 运维可观测性边界（V0.2 锁定：元指标 + 匿名化事件采样）

### 7.1 可见 vs 不可见

| 类别 | 可见性 |
|---|---|
| 进程健康指标（CPU / 内存 / 错误率） | 可见 |
| 循环节拍统计（无内容） | 可见 |
| 7 状态机的分布统计 | 可见 |
| LLM token 总用量元指标 | 可见 |
| 单次循环具体内容 / 生命体说过的话 | **不可见** |
| 记忆原文 / Values 字段 / Reflection 摘要 | **不可见** |
| 匿名化事件采样（事件类型 + 时间戳，去身份、去内容） | 可见 |

### 7.2 匿名化事件采样的实现位点

- 采样在 `SDK` 与 `LLMAdapter` 两个边界点完成。
- 上送前**去身份**（无用户 ID、无生命体 ID）。
- 上送前**去内容**（仅事件类型 + 时间戳 + 元尺寸）。
- 用户可在 `ConfigureLLM` 同级开关 `ConfigureTelemetry` 完全禁用上送。

#### 7.2.1 元指标允许 / 禁止边界（Y3）

为守住 `06 §2.1` token 不暴露原则同时保证可运维性：

| 允许采集 | 禁止采集 |
|---|---|
| LLM 调用次数 / 频率 | token 数 / token 单价 |
| 调用类型分布（Reason / Embed / Summarize / Critique） | 调用内容 / prompt 全文 / 响应全文 |
| 错误率 / 失败模式分类 | 失败的具体上下文 |
| 节拍统计（区间分布） | 节拍背后的具体行为 |
| 状态机转移分布 | 状态机转移的具体触发事件 |
| Skill 调用次数（按类别） | Skill 调用参数 / 结果 |

**核心区分**：调用频次（行为元数据）≠ token 数（计费量纲）。前者允许采集用于运维，后者严禁外泄。任何把"调用次数"与"token 数"建立 1:1 映射的设计 = 反模式（参 `10 A04`）。

### 7.3 故障排查

- 默认不允许运维"打开某个生命体看一看"。
- 用户发起诊断申请时，可在客户端**自助打包**当次问题上下文 → 上传到独立诊断通道 → 运维看到的是用户挑选的窗口，而非长期访问。
- 该机制详见 `06 §7` 与 `R08`。

---

## 8. 模块演进与兼容性

### 8.1 模块版本与生命体兼容性

- 同一 Runtime 大版本内：模块可独立演进，需保持 SDK 暴露面（§4.4）的 stability 等级承诺。
- 跨大版本：必须提供生命体迁移路径。**不允许**强制升级导致已运行生命体破坏。
- 不兼容升级时的用户选择：保留旧版 Runtime / 接受迁移 / 进入 Memorial。

### 8.2 子模块替换

V0.2 锁定哪些子模块支持替换：

| 子模块 | 可替换性 |
|---|---|
| `LLMAdapter` | 可替换（用户配置） |
| `SkillRegistry` 的 Skill 实现 | 可替换 / 可扩展 |
| `MemoryEngine` 底层存储 | 可替换（本地 SQLite / 云数据库），上层语义不变 |
| `StateManager` / `ReflectionEngine` / `GoalArbitrator` | **不可替换**（核心人格机制） |
| `GenesisModule` | **不可替换** |

不可替换的子模块 = 人格的根本所在，第三方扩展不得绕过。

### 8.3 模块演进的兼容性约束

- 字段集合可扩张，不可重义（与 `02 §1.2` 一致）。
- SDK 出站事件可加事件类、可加字段，不可删字段。
- 入站请求结构同上。
- 状态机不可删状态（删 = 破坏已 Memorial 生命体的可解释性）。

---

## 9. 本轮新引入的待答 / 风险

| 编号 | 议题 | 影响章节 |
|---|---|---|
| R17 | 设备迁移密码丢失的恢复路径（V0.2 默认不可恢复） | `04 §1.2`、`03 §4.3.1`、`04 §2.1 MigrationVault` |
| R20 | 迁移期间"不在任何设备运行"与持续存在哲学的张力 | `04 §1.2`、`03 §4.3.1`、`01 §3.1` |
| R18 | 第三方 Skill 注册的恶意/低质过滤机制 | `04 §4.3`、`06 §5` |
| R19 | 匿名化事件采样去身份后的统计回连风险 | `04 §7.2`、`10 R02 R08` |

`R06`（跨平台 / 跨生命体冲突）与 `R08`（可观测性边界）在本文档落实了 §5 与 §7 的部分答案，但仍未完全收口，留 `08` 与 `06` 进一步细化。
