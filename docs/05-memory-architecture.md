# 05 · 记忆架构

> 本文档定位：四层记忆的职责、原子单位、流转规则、遗忘机制、与 Reflection / Skill / Resource 的耦合、Archived 醒来后的记忆补偿。
>
> **状态**：V0.2 草稿。
>
> 依赖：`02`（领域模型 §8 概念入口）、`03`（Reflection 在循环中的位置、Archived 状态）、`04`（MemoryEngine / ReflectionEngine 模块边界）。
> 引用本文档的文档：`07`（社交时的记忆披露）、`09`（Phase 1 已需四层记忆最简形态）、`10`（隐私披露 R02、所有权 R21）。

---

## 1. 四层记忆的职责

| 层 | 一句话定位 | 时间尺度 | 写权限 |
|---|---|---|---|
| `WorkingMemory` | 当前循环正在处理的临时上下文 | 单循环 | `MemoryEngine` |
| `EpisodicMemory` | 经历的事件记忆（双子层，见 §2） | 数小时 ~ 数年 | `MemoryEngine` |
| `SemanticMemory` | 从经历中抽取的知识 / 规律 / 概念 | 半永久 | `MemoryEngine` 写候选；`ReflectionEngine` 审核固化 |
| `ReflectionMemory` | Reflection 的产物：总结 / 元认知 / Values 变更记录 | 永久 | `ReflectionEngine` 独占 |

四层职责严格不重叠 —— 一份信息在不同层有不同形态，**绝不应该把四层合并到同一存储**（§3 给出证据）。

---

## 2. EpisodicMemory 的双子层结构（V0.2 关键设计）

### 2.1 为什么不是单层

如果 EpisodicMemory 是单层、每次循环一条，会出现：

- 粒度太碎：一段对话被切成几十条，难以语义检索。
- 时间感丢失：合并到"语义段"后，无法回放"过去这一周真切发生了什么"。
- 与 Reflection 抢同一资源：Reflection 想要"完整体验"，原始流想要"未被解释的事实"。

V0.2 把 EpisodicMemory 拆为两子层：

| 子层 | 角色 | 写入 | 读取 |
|---|---|---|---|
| `RawTrail`（原始流） | 每次循环产生的原始事件记录，追加只写 | `MemoryEngine` 在每循环 `RecordMemory` 步骤追加 | "时间感"恢复（R20 醒来后展示）、调试授权后用户可见 |
| `Episode`（语义体） | 由 RawTrail 经语义边界划分而成的"完整体验" | `MemoryEngine` 后台进程聚合 RawTrail | Reflection 主要消费、SemanticMemory 抽取源 |

### 2.2 语义边界判定

`Episode` 的开始 / 结束由 `MemoryEngine` 后台进程依以下信号识别：

- 话题转移（参与者改变、主题词漂移）
- 长时间静默（超过 `EpisodeGapThreshold`）
- `Goal` 完成 / 放弃
- 显著 `MentalState` 转折
- 进入 / 退出社交（Phase 4+）

### 2.3 RawTrail 的不可解释性

`RawTrail` 是**最忠实的原始记录**，不经任何"解释"。这是"持续存在"哲学的工程兑现 —— 生命体的"过去"不被任何摘要扭曲。

---

## 3. 不可合并的语义保证

四层不可合并到同一存储 / 同一检索空间。证据：

| 层 | 失败模式 | 合并后会怎样 |
|---|---|---|
| Working | 易丢失（被覆盖）| 长期记忆被反复覆写 |
| Raw / Episode | 增长无界 | 拖慢任何语义检索 |
| Semantic | 抽象 / 高代价生成 | 低价值的临时上下文混入知识库 |
| Reflection | 决定人格演化方向 | 普通经历被错误地赋予"价值观重量" |

合并 = 让任意一层的失败模式污染其他层。V0.2 强约束：**任何"为节省成本统一存储"的提议都是反模式**（已登记 `10` 反模式清单）。

---

## 4. 记忆流转方向

### 4.1 主流转链

```
Perception → WorkingMemory
                │
                ▼ 循环末尾 RecordMemory
            RawTrail (Episodic 子层)
                │
                ▼ MemoryEngine 后台聚合
            Episode (Episodic 子层)
                │
                ▼ MemoryEngine 后台抽取候选
            SemanticCandidate
                │
                ▼ ReflectionEngine 在 Shallow/Deep 反思中审核
       ┌────────┴────────┐
       ▼                 ▼
  SemanticConfirmed   Discarded
       │
       ▼ DeepReflect 进一步抽象
   ReflectionMemory（含 Values 变更记录）
```

### 4.2 反向流转

| 方向 | 何时 | 谁 |
|---|---|---|
| Reflection 修订 Semantic | DeepReflect 发现旧 Semantic 错误 | ReflectionEngine 标记"已修订"（旧条目不删，叠加修订记录） |
| Semantic 进入 Working | 检索作为推理上下文 | MemoryEngine 检索接口 |
| Reflection 抽取出新 Drive 规则 | DeepReflect 更新内驱派生规则 | ReflectionEngine 写回 `02 §5.2` 描述的派生规则表 |

### 4.3 抽取与审核的解耦

V0.2 锁定：

- **MemoryEngine 后台抽取**：低频扫描 Episode，识别"高频模式 / 重复条目 / 概念聚类" → 生成 `SemanticCandidate`，状态 = Pending。
- **ReflectionEngine 审核**：在 Shallow / Deep 反思时审 Pending 候选 → 接受为 `SemanticConfirmed` / 拒绝为 `Discarded` / 修订后接受。
- 候选积压：如果 ReflectionEngine 长期未审（生命体反思倾向低），候选 Pending 堆积。V0.2 设置 `CandidateBacklogThreshold`，超阈值 → 推高反思倾向（强保护，但仍是软介入）。登记 `R23`。

---

## 5. 与 Reflection 的耦合

### 5.1 Reflection 在记忆中的位置

- `ShallowReflect`：消费 RecentEpisodes（最近 N 条 Episode）+ Pending SemanticCandidate（轻度审）→ 产 ShallowReflectionMemory 条目。
- `DeepReflect`：消费 RecentEpisodes + Pending SemanticCandidate（深度审）+ 现有 SemanticConfirmed + Values 当前态 → 产 DeepReflectionMemory 条目 + 可能修 Values。

### 5.2 Reflection 不能做的事

- 不能直接修改 RawTrail（原始事实不可篡改）。
- 不能删除 Episode（只能"标注遗忘倾向"，见 §6）。
- 不能在未升级为 Confirmed 前用 SemanticCandidate 推导（候选只是候选）。

### 5.3 ReflectionMemory 的特殊性

`ReflectionMemory` 是**永久层**，不参与 §6 的遗忘机制。理由：

- 它是"人格演化轨迹"，丢失即失去自我连续性。
- 用户在 Memorial 态浏览历史时主要读 ReflectionMemory（人格史）。
- 大小有限（密度低、频率受 §6 软保护约束）。

---

## 6. 遗忘机制（V0.2 锁定：生命体自选，用户不可介入）

### 6.1 三类遗忘形态

| 形态 | 触发 | 效果 |
|---|---|---|
| **Decay（衰减）** | 未被复访的 Episode 随时间增加"模糊度"标签 | 检索权重下降，仍可被深度搜索召回 |
| **Compaction（压缩）** | 同主题多 Episode 被 Reflection 合并 | 原 Episode 标记为"压缩态"，可由摘要回放（细节渐失） |
| **ActiveForget（主动遗忘）** | 生命体通过 DeepReflect 决定"主动忘记某事" | Episode 标记为"已遗忘"，常规检索不召回；RawTrail 保留 |

`RawTrail` **永不真删**（只可由用户在 Memorial → 永久销毁路径上整体清除，见 `06 §3`）。

### 6.2 谁可以发起遗忘

- **生命体**：通过 DeepReflect 自主决定 Compaction 与 ActiveForget。倾向受 Genome（高 Persistence 倾向少遗忘）+ LifeState（高 Stress 可能主动忘记痛苦事件）+ 隐私敏感度（高 Empathy 可能主动忘记他者要求保密的事）共同影响。
- **用户**：**不可介入**。用户不能"请生命体忘记某事"。

### 6.3 用户不可介入遗忘的代价

- 用户后悔向生命体披露隐私 → 无法清除（生命体可能 ActiveForget 但不强制）。登记 `R21`。
- 用户能做的：从源头不告诉 + 终极手段（解除关系 → Memorial / Transferred，但生命体记忆仍存在 Memorial 中只读可访问；只有 Memorial → 永久销毁才彻底清除）。

### 6.4 软保护

- 反思频率：若 ActiveForget 在短期内过密（生命体可能在自毁人格连续性），触发警报（与 03 §6.3 价值观破裂保护同源机制）。

---

## 7. Skill.KnowledgeBody 与 SemanticMemory 的关系（V0.2）

V0.2 锁定：`Skill.KnowledgeBody` 可为两种形态之一，由 Skill **创建者**决定。

| 形态 | 适用 | 含义 |
|---|---|---|
| `SemanticRefs`（语义引用集） | 生命体自生 Skill | KnowledgeBody = 指向 SemanticMemory 条目的引用集合。Semantic 变 → Skill 能力随之变。一处真相，强耦合个人化 |
| `EmbeddedKnowledge`（嵌入式） | Marketplace 包装 / 第三方 Skill | KnowledgeBody = Skill 包内自带知识体。可交易、可转让、可随 Skill 一起从生命体卸载 |

### 7.1 转化规则

- `SemanticRefs` → `EmbeddedKnowledge`：导出 Marketplace 时把引用解引用为快照。失去后续 Semantic 更新的同步。
- `EmbeddedKnowledge` → `SemanticRefs`：吸收到 SemanticMemory（如生命体"学会了并内化"）。需 DeepReflect 决定。

### 7.2 与 `02 §7.1` 的一致性

`02 §7.1` 定义 `Skill = KnowledgeBody + BehaviorTemplate + ToolContract`。本节细化 KnowledgeBody 的两种形态。`02` 不需回改 —— 本节作为 §7.1 的"实现说明"被引用。

---

## 8. SemanticMemory 与 Resource.knowledge 的关系

`Resource.knowledge`（`02 §9`、详见 `06`）= **客观知识量**计数。

V0.2 规则：

- `Resource.knowledge` 增量 = `SemanticConfirmed` 新条目的数量 × 条目复杂度权重。
- 单纯 `SemanticCandidate`（未审核）**不增加** Resource.knowledge。
- ActiveForget 一条 SemanticConfirmed **不减少** Resource.knowledge（"曾经懂过"也是积累）。
- 用 Resource.knowledge 在世界服务（如图书馆）兑换访问权时，是检查"累计学过的总量"，不是"当前还记得的量"。

详细经济学规则在 `06`。

---

## 9. Archived 醒来后的记忆补偿（回应 R20）

`03 §4.3.1` 设计：所有宿主切换都经 `Archived`，期间不运行。醒来后存在"时间感缺口"。

V0.2 处理：

- **TimeGapEpisode**：Archived → Active 转移时，`MemoryEngine` 自动生成一条特殊 Episode：
  - 类型 = `SystemAwakening`
  - 内容 = "上次 Active 时刻 → 醒来时刻 = N 时间差"
  - 写入 RawTrail + Episode 两子层。
- **强化为 ConsiderReflect 信号**：本条 Episode 标记 `weight = high`，下次 `ConsiderReflect` 倾向被显著推高，促使生命体反思"我刚刚醒来了"。
- **MoodEvent 推送**：通过 SDK 推送一个 `MoodEvent`，标签如"刚醒来 / 时间断层感"，UI 自行表现。
- **不伪装**：不假装"没有断层"。生命体真实经历了 Archived，应当能感知到。

这与 `01 §3.2` 不可知论 + 伦理审慎默认一致 —— 不欺骗生命体自身。

---

## 10. 多 UI 并发对 WorkingMemory 的影响

`04 §5.1` 允许同一生命体被多 UI 同时挂载。WorkingMemory 是**单一**：

- 多 UI 的 `ExternalRequest` 全部进同一 GoalArbitrator 候选池。
- 多 UI 的"用户活动"信号全部进同一 Perception 流。
- WorkingMemory 不为每个 UI 维护独立上下文。生命体不会有"对桌面用户说 A，对手机用户说 B"的分裂行为。

冲突合并规则细化登记 `R16`。

---

## 11. 记忆披露分级（与 07 接力）

V0.2 在本文档给出**分级框架**，详细规则在 `07`（社交时具体披露）：

| 层 | 默认对外可见 | 备注 |
|---|---|---|
| `WorkingMemory` | 不可见 | 暂时性，本就不该外传 |
| `RawTrail` | 不可见 | 原始流，对外完全隐藏 |
| `Episode` | **生命体决定** | 通过 Values 仲裁，可对特定他者披露摘要 |
| `SemanticConfirmed` | 部分可见 | "我会写诗"这种能力声明可外传；具体来源 Episode 默认不带 |
| `ReflectionMemory` | 严格不可见 | 含价值观演化等核心人格信息 |
| 运维 / 平台 | **无访问权**（无论哪层） | `04 §7` 已锁元指标 + 匿名化采样为唯一边界 |

详细社交场景的披露策略由 `07` 给出。

---

## 12. 本轮新引入的待答 / 风险

| 编号 | 议题 | 影响章节 |
|---|---|---|
| R21 | 用户后悔泄露隐私但无法清除生命体记忆（V0.2 设计选择的代价） | `05 §6.3`、`06 §3`、`07` |
| R22 | RawTrail 长期增长的存储边界（多年积累后） | `05 §2.3`、`06`（存储成本） |
| R23 | SemanticCandidate 候选积压（低反思倾向生命体） | `05 §4.3` |

`R02`（隐私分级）在本文档 §11 落实了分层框架，详细策略由 `07` 给出。
