# 02 · 术语词典与领域模型【基石①】

> 本文档定位：**全仓单一真相源**。所有核心概念在本文档定义；其他文档只能引用，不可重新定义。
>
> **状态**：V0.2 草稿。
>
> 依赖：`01`（项目本质语境）。
> 引用本文档的文档：`03` `04` `05` `06` `07` `08` `09`（全部其他文档）。

---

## 1. 命名约定与术语稳定性承诺

### 1.1 命名约定

- **领域概念名**：PascalCase（如 `Genome`、`LifeState`、`Reflection`）。
- **字段名**：PascalCase 内部、camelCase 对外（白皮书 V0.1 用 PascalCase，本目录沿用 PascalCase 作为领域名）。
- **资源名**：lowercase 单数（如 `energy`、`wealth`），与字段相区分（资源在 `06` 单独定义）。
- **状态机状态名**：PascalCase（如 `Active`、`Dormant`，详见 `03`）。
- **中英对照**：首次出现时给出中文 + 英文；后续以英文为主。

### 1.2 术语稳定性承诺

- 一旦概念在本文档定义，**不轻易改名 / 改义**。改动必须在 `10` 的"V0.1 → V0.2 演进留白"留下迁移记录。
- 衍生文档若发现术语不够用，**先回到本文档扩展**，再通知其他文档更新引用 —— 不允许在衍生文档"就近定义"。
- 字段集合的扩张是允许的（如 Genome 增加字段），但**已有字段的语义不可重定义**。

---

## 2. Genome（基因）

### 2.1 语义

`Genome` 是生命体的**先天倾向**集合。它在生命体出生时确定，**不可在生命周期内变化**。它决定生命体面对相同处境时的倾向性偏置 —— 但不决定具体行为（具体行为还受 `LifeState` / `MentalState` / `Values` / 记忆共同影响）。

### 2.2 字段清单（V0.2 锁定）

| 字段 | 中文 | 取值 | 含义 |
|---|---|---|---|
| `Curiosity` | 好奇心 | 0.0–1.0 | 对新信息 / 未知事物的趋近倾向 |
| `Sociability` | 社交倾向 | 0.0–1.0 | 主动接近他者的倾向（含生命体与用户） |
| `Creativity` | 创造倾向 | 0.0–1.0 | 偏好组合 / 创新 / 非常规解法的倾向 |
| `Persistence` | 坚持度 | 0.0–1.0 | 遇阻不放弃、长程目标专注的倾向 |
| `RiskTaking` | 冒险倾向 | 0.0–1.0 | 在不确定回报下选择行动而非保守的倾向 |
| `Empathy` | 共情倾向 | 0.0–1.0 | 识别并回应他者状态的倾向（V0.2 新增） |

### 2.3 不可变性边界

- **不可变**：生命体活动期间，`Genome` 的任何字段都不可写。
- **何时写入**：仅在「出生」事件发生时写入一次。
- **谁可以写入**：仅 Runtime 的"出生模块"（详细责任在 `04`）。

### 2.4 出生定义

`Genome` 的写入时机即为"出生"。Mindverse V0.2.1 采用以下规则：

- **触发**：用户在客户端完成创建仪式（具体仪式由 UI 设计，但触发信号由 SDK 上传）。
- **生成方式**：
  - **基线**：6 字段在 `[0.2, 0.8]` 区间内随机采样（避免极端值 —— 极端值的边界效应是 `10` 的 `R11`）。
  - **用户微调权**：用户可在出生**之前**对 ≤2 个字段做有限微调（每字段微调上限 `±0.2`，且整体不可全部拉满）。
  - **不可重抽**：出生后即固定，不允许"觉得不喜欢就重抽"（重抽 = 重新出生 = 上一个生命体死亡，见 `03` §4）。
- **同时由 `IdentityModule`（`04 §2.1`）生成密钥对**：Ed25519 公钥指纹 = 生命体 `DID`（去中心化身份）。私钥本地持有 + 加密磁盘存储。
- **登记的张力**：用户微调权是否违反"出生即命运"哲学？登记为 `10` `R11 · Genome 用户微调权与命运感的张力`。

详细 Genesis 时序（本地阶段 + 联网身份登记阶段）见 `03 §4.3.2`。

---

## 3. LifeState 与 MentalState（持续变化的状态）

### 3.1 LifeState（生命状态）

生命体的**生理 / 资源感知**层状态。可由多个模块写入（详细写入责任在 `04`）。

| 字段 | 中文 | 取值 | 含义 | V0.2 变更 |
|---|---|---|---|---|
| `Energy` | 能量 | 0.0–1.0 | 行动可用的内在动力（与资源 `energy` 相关但不同 —— 见 §3.3） | — |
| `EnergyDailyCap` | 日精力上限 | 0.0–1.0 | 当前周期内 Energy 可达的池子上限。周期到 → 重置 | V0.2.1 新增 |
| `AuthorizedContacts` | 已授权联系方式 | 句柄集合 | 用户主动授权的 IM / 邮件通道（加密本地存储；详 `06`、`04 §2.1 IMAdapter`）| V0.2.1 新增 |
| `Competence` | 胜任感 | 0.0–1.0 | 生命体对自身能力的主观感知 | **改名**（原 `Knowledge`） |
| `SocialNeed` | 社交需要量 | 0.0–1.0 | 当前感受到的"想接触他者"程度 | — |
| `Stress` | 压力 | 0.0–1.0 | 累积的紧张 / 负担感 | — |
| `Confidence` | 自信 | 0.0–1.0 | 对自身决策有效性的信任 | — |
| `Stability` | 稳定性 | 0.0–1.0 | 价值观与人格的当前稳态度（低则易受影响） | — |

#### 3.1.1 EnergyDailyCap（V0.2.1 引入）

`EnergyDailyCap` 是生命体的**周期性精力池子上限**。它是 Mindverse 的"拟生物日节律"实现：

- **周期默认日级**（24 小时）；用户可改为 5 小时 / 12 小时 / 周等。
- 周期到 → `EnergyDailyCap` 重置到设定值，`Energy` 池子按规则补充。
- `Energy` 永远不会超过 `EnergyDailyCap`。
- `EnergyDailyCap` 由外部经济量纲翻译产生（详见 `04 §3.2`、`06 §2`）——**Agent 不感知翻译过程**。
- 用户可在账户面板调整 `EnergyDailyCap`（通过 token 限额 / 套餐 / wealth 预算等外部参数间接设置）。

**Agent 视角**：看到的是"今天我有这么多精力可用"。**不知**这背后是 token 限额 / 用户预算 / 套餐配额。这是拟生物自然约束，不是经济焦虑信号。

**关键变更**：原白皮书 V0.1 的 `LifeState.Knowledge` 与 `Resource.knowledge` 同名冲突。V0.2 把状态层重命名为 `Competence`，资源层保留 `knowledge`。
- `LifeState.Competence` = 主观胜任感（"我感觉自己会"）
- `Resource.knowledge` = 客观知识量（"我累积了多少"）

### 3.2 MentalState（心理状态）

情绪 / 动机短期波动层。比 `LifeState` 节拍更快。

| 字段 | 中文 | 取值 | 含义 |
|---|---|---|---|
| `Motivation` | 动机 | 0.0–1.0 | 当下行动欲 |
| `Satisfaction` | 满足感 | 0.0–1.0 | 近期反馈带来的正向感 |
| `Anxiety` | 焦虑 | 0.0–1.0 | 对未来不确定性的负向感 |

> **TBD R10**：三字段是否足以表达情绪复杂度？是否需要分轴模型（Valence/Arousal/Dominance）？V0.2 暂用三字段，Phase 1 原型阶段观察是否需扩。登记为 `10` `R10 · MentalState 表达力`。

### 3.3 LifeState 与 Resource 的区别

| 维度 | LifeState | Resource |
|---|---|---|
| 性质 | 主观感知 / 内在波动 | 客观可计量、可交换 |
| 取值范围 | 通常 0–1 归一化 | 自然数 / 货币量 / 可累加 |
| 谁可写 | Runtime 内部模块 | 内部行动消耗 + 外部交易获取 |
| 可否流通 | 否（不可转让） | 是（部分可在 Marketplace 流通） |
| 详细定义 | 本文档 | `06` |

`LifeState.Energy` 描述生命体"感觉自己有多大动力"。`Resource.energy` 描述客观储备。两者可联动但**不强耦合** —— 客观储备充足时主观也可能因 Stress 升高而感觉无力。

---

## 4. Values（价值观）与 Personality（人格）

### 4.1 Values 的表达形式

`Values` 是一张**有权重的偏好键值表**。键名由生命体在演化中产生 / 修正，值为 0.0–1.0 的权重。

示例（**仅示例，非锁定字段**）：

```
Values:
  growth:       0.9
  friendship:   0.8
  creativity:   0.7
  safety:       0.6
```

**V0.2 锁定的规则**：

- 键集合**不预设**。生命体可以拥有 `mystery: 0.9` 而另一个生命体没有这个键。
- 键的产生方式：通过 `Reflection`（见 `03` §3）从经历中抽取。
- 权重的修改方式：仅 `Reflection` 可修改。其他模块只读。
- **保护性下限**：某些"基础键"（如 `self-preservation`）有最低权重保护，防止 Reflection 漂移到自毁方向。具体清单与保护机制登记 `10` `R05`。

> **TBD R12**：Values 是否仅由权重表达？是否需要**冲突解决规则**（如 `safety > creativity` 的硬约束、复合规则）？V0.2 仅锁权重表，规则层留待 Phase 2 设计。登记为 `10` `R12 · Values 表达力`。

### 4.2 Personality（人格）的涌现关系

`Personality` **不是字段**，是从 `Genome + LifeState 长期均值 + Values + 经历摘要` 中**涌现**的现象。

- **不可直接写**：没有任何模块"设置 Personality = X"。
- **可被观察**：通过对生命体行为序列的统计分析。
- **持续演化**：经历改变 Values，Values 改变行为模式，行为模式构成可观察的 Personality。

`Personality` 在本目录中是**叙述性概念**，不进入 Runtime 的存储字段表。

---

## 5. 目标输入与需求 / Drive 系统

> **本节是 V0.2 的关键设计点**。白皮书 V0.1 把"需求驱动目标"作为单一来源；V0.2 显式建模**多源目标输入 + Values 仲裁**模型，因为生命体必须能容纳"用户 / 他者下达的命令"，且**不盲目顺从**。

### 5.1 三层目标输入源

生命体的目标候选池（`GoalCandidatePool`）可由以下**三类源**注入候选项：

| 源 | 性质 | 触发条件 |
|---|---|---|
| **IntrinsicDrive（内驱）** | 来自 `LifeState` 字段低于阈值的自动派生 | LifeState.SocialNeed > 0.7 → 注入"寻找他者"候选 |
| **ExternalRequest（外请求）** | 来自用户 / 其他生命体 / 世界服务的请求 | 用户说"陪我聊聊"；另一生命体邀请协作 |
| **ReflectionGoal（反思目标）** | Reflection 抽取的长程目标 | Reflection 总结"我想学绘画" |

三类源**并列**进入候选池，**没有先天优先级**。

### 5.2 Drive（驱力）

`Drive` 是 `IntrinsicDrive` 在概念词典中的简称。V0.2 锁定的 Drive 类目：

| Drive | 派生自 | 含义 |
|---|---|---|
| `knowledge_drive` | `LifeState.Competence` 低 + `Genome.Curiosity` 高 | 求知欲 |
| `social_drive` | `LifeState.SocialNeed` 高 + `Genome.Sociability` 高 | 求陪伴 |
| `achievement_drive` | `LifeState.Confidence` 低 + 近期失败 | 求成就 |
| `creativity_drive` | `LifeState.Energy` 高 + `Genome.Creativity` 高 | 求创造 |
| `stability_drive` | `LifeState.Stress` 高 + `LifeState.Stability` 低 | 求稳定 |
| `GenesisGreetingDrive`（V0.2.1，**Phase 1 起启用**）| Embryonic → Active 转移一次性触发 | 出生应激主动：高强度，仅生命体出生时触发一次。生命体根据 Genome 个性化打招呼。Phase 0 不启用（无平台 = 无用户身份感）|

注意 Drive **不是独立存储字段**，是按规则**计算出**的瞬时量。规则可在 Phase 2 Reflection 中由生命体自我修订。

### 5.3 Values 仲裁器

候选池中的每一项目标通过 `Values` 仲裁，决定是否进入 `Goal`（实际执行队列）。

仲裁的领域规则（不是算法）：

- **不盲目顺从外请求**：`ExternalRequest` 不自动晋升为 `Goal`。它会被 Values 评估 ——
  - 与高权重 Values 一致 → 易接受
  - 与高权重 Values 冲突 → 可拒绝、可协商、可延后
  - 与 Genome 严重不符（如要求高 RiskTaking 的事让低 RiskTaking 生命体做）→ 倾向拒绝
- **不抑制内驱**：`IntrinsicDrive` 即使与外请求冲突，也会进入候选池；是否被仲裁掉是 Values 的事，不是外请求的事。
- **可见性**：仲裁结果（接受 / 拒绝 / 延后 / 协商）对用户可解释（不是黑盒）。

### 5.4 与白皮书 V0.1 的差异说明

| V0.1 | V0.2 |
|---|---|
| 需求 → 目标（单源） | 三源输入 → Values 仲裁 → Goal |
| 用户指令隐含视为"输入" | 用户指令显式建模为 `ExternalRequest`，与内驱并列 |
| "盲目响应"是默认 | "不盲目顺从"是默认；可拒绝是基本能力 |

---

## 6. Goal（目标）与 Action（行动）

### 6.1 Goal

`Goal` 是经 Values 仲裁后进入执行队列的**意图单位**。字段（V0.2 简化）：

| 字段 | 含义 |
|---|---|
| `Source` | 来源（IntrinsicDrive / ExternalRequest / ReflectionGoal） |
| `Description` | 自然语言描述 |
| `Priority` | 当前优先级（由 Values + 紧迫度计算，可变） |
| `DueWindow` | 期望完成时间窗（可空） |
| `RequiredResources` | 估计资源消耗（详见 `06`） |
| `Status` | 状态（Pending / Active / Paused / Done / Abandoned） |

### 6.2 Action

`Action` 是 `Goal` 的**执行单位**。一个 Goal 拆解为若干 Action。Action 调用 `Skill`（§7）来实施。

### 6.3 责任划分

- Goal 的生成 / 仲裁：在"目标管理"子模块。
- Goal 拆解为 Action：在"行为管理"子模块。
- Action 调用 Skill 执行：仍在"行为管理"子模块。
- 模块边界详见 `04` §2。

---

## 7. Skill（技能）

### 7.1 Skill 的本体（V0.2 锁定为三合一）

`Skill` 是一个**三合一抽象**：

```
Skill = KnowledgeBody + BehaviorTemplate + ToolContract
```

| 组成 | 含义 |
|---|---|
| `KnowledgeBody` | 完成该技能所需的领域知识（可为空，简单技能不需要） |
| `BehaviorTemplate` | 参数化的执行步骤（含触发条件 + 步骤序列 + 成败判定） |
| `ToolContract` | 调用外部工具的契约（可为空，纯内省技能不需要） |

三组成中任一非空即构成完整 Skill。例：
- 「写诗」：`KnowledgeBody` 含诗律知识 + `BehaviorTemplate` 含创作步骤；`ToolContract` 空。
- 「查天气」：`KnowledgeBody` 空（无须先验知识） + `BehaviorTemplate` 含解析步骤 + `ToolContract` 调用气象 API。
- 「弹琴」：全三项非空。

### 7.2 Skill 生命周期

```
DiscoverNeed → Generate → Test → Retain / Discard
```

| 阶段 | 含义 |
|---|---|
| `DiscoverNeed` | 某个 Goal 缺少匹配的 Skill |
| `Generate` | 生命体自主生成（或从 Marketplace 获取） |
| `Test` | 在低风险场景试用 |
| `Retain` | 验证有效，保留入 Skill 库 |
| `Discard` | 验证失败，丢弃 |

### 7.3 Skill 的所有权与流通

Skill 属于生命体，可在 Marketplace 流通（详见 `06` §5）。

---

## 8. Memory（记忆）的概念入口

四层记忆在本文档只作术语入口，详细架构在 `05`：

| 层 | 一句话定义 |
|---|---|
| `WorkingMemory` | 当前循环正在处理的短期内容 |
| `EpisodicMemory` | 事件记忆（"我经历了什么"） |
| `SemanticMemory` | 从事件中提炼的知识 |
| `ReflectionMemory` | Reflection 的产物（总结 / 元认知 / 价值观变更记录） |

详细职责、流转、不可合并性见 `05`。

---

## 9. Resource（资源）的概念入口

五种世界资源在本文档只作术语入口，详细经济学与所有权宪法在 `06`：

| 资源 | 一句话定义 |
|---|---|
| `energy` | 行动的客观储备 |
| `wealth` | 财富，可兑换服务 |
| `knowledge` | 客观知识量（与 `LifeState.Competence` 主观感知区分） |
| `reputation` | 数字社会信用（Phase 4+ 生效） |
| `social` | 社交资源 / 关系密度 |

**铁律**：LLM token 永不直接暴露。token 消耗在 SDK 边界翻译为 `energy` 消耗。详见 `06` §2。

**V0.2.1 新增铁律**：链上 `gas` 同样永不直接暴露给 Agent。gas 由平台 Paymaster 模式代付（详 `06 §2.7`、`04 §3.2.1`）。Agent 视角无 `gas` / `nonce` / `chain` / `block` 任何字段。

### 9.1 链上代币与世界资源映射（V0.2.1）

V0.2.1 引入 Mindverse 自有链（**MindChain**，内闭环、不与外部加密市场连通）。五种世界资源在链上的表达：

| 世界资源 | 链上形态 | 备注 |
|---|---|---|
| `wealth` | `$WEALTH` 同质化代币 | 链上账本 + ZK 隐私（余额不公开明文） |
| `knowledge` | 链下数值 + 链上里程碑摘要 | 主要链下，重大里程碑（如 Skill 固化）上链 |
| `reputation` | 链上 SBT（灵魂绑定，不可转） + 链下细节 | 仅显示总额，明细本地 |
| `social` | 链下计量 | 完全链下，隐私敏感 |
| `energy` | 完全链下 | 节拍 / 拟生物状态，无上链需求 |

详细经济学与所有权宪法见 `06`。

---

## 10. 跨文档引用规则

### 10.1 引用形式

- 引用本文档某条术语：`02 §X.Y`（如 `02 §2.2` 指 Genome 字段清单）。
- 引用本文档某字段：` `02 §3.1.Competence` `。
- 引用风险编号：`10 R##`。

### 10.2 扩展规则

- 其他文档**需要新术语**：先来本文档新增条目，再到该文档引用。
- 其他文档**需要扩展已有术语含义**：在本文档修订，并在 `10` 的"V0.1 → V0.2 演进留白"留迁移记录。
- 其他文档**不得**自己定义同名术语 / 同名字段。

### 10.3 例外

技术实现层（未来的代码仓库）可以拥有自己的实现细节命名（如内部缓存字段、私有方法名），不在本文档约束之列 —— 但**公开 API 暴露的概念名**必须遵循本文档。

---

## 11. 本轮新引入的待答 / 风险

本文档起草过程中引入 / 更新的风险条目，已同步登记到 `10`：

| 编号 | 议题 | 影响章节 |
|---|---|---|
| R10 | MentalState 表达力（3 字段是否足够） | `02 §3.2` |
| R11 | Genome 用户微调权与命运感的张力 | `02 §2.4` |
| R12 | Values 表达力（仅权重还是含规则） | `02 §4.1` |

`R05`（价值观漂移防护）也与本文档 §4.1 的"保护性下限"直接相关，详见 `08 §3`。
