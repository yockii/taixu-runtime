# 10 · 风险与未决问题

> 本文档定位：已识别风险登记、跨文档冲突待决议题、V0.1 → V0.2 演进留白、反模式清单。**所有盲点在此集中，不分散在各文档**。
>
> **状态**：V0.2.2 草稿。共登记 ~82 项风险（R01–R82）。注：R62/R63 编号预留。最新批 R69–R82 来自 Phase 0.4+ reflex / skill / tool / 抓取 / 上下文 / 知识感知 / 发呆 / 社交 / 知识结晶 / 技能生命周期阶段。

---

## 1. 已识别风险登记

### R01 · 身份与认证边界
> 生命体"我是谁"如何持久标识？多设备 / 迁移后是否仍是同一生命？归属链如何防伪？
> **V0.2 进展**：`03 §4.3.1` 锁定迁移密码 + 加密包；`07 §2.4` 锁定公钥签名身份；`07 §9` 锁定身份呈现策略。
> **仍待**：账号体系与生命体公钥的绑定关系；账号丢失但加密包持有时的身份恢复路径。
> **影响**：`06 §3 §4`、`07 §2 §9`、`R17`。

### R02 · 隐私边界与记忆披露分级
> 四层记忆是否各有披露策略？社交 / Marketplace 时哪些可外传？
> **V0.2 进展**：`05 §11` 锁定分级框架；`07 §6` 锁定跨生命体披露规则；`04 §7` 锁定运维边界（元指标 + 匿名化）。
> **仍待**：跨生命体二次披露的细节；披露后撤回机制（很可能不可能）。
> **影响**：`05 §11`、`07 §6`、`R19`、`R21`、`R30`。

### R03 · 无真死亡的语义边界
> V0.2 设计：数字生命**无真死亡**，终态是 `Detached` → `Memorial` 或 `Transferred`（`03 §4`）。
> **仍待**：
> - `Detached` 30 天未决自动转 `Memorial` 是否合理？时长依据？
> - `Memorial` → 永久物理销毁的门槛（多次确认 + 冷静期？）。
> - `Memorial` → `Detached` 复活墓碑（见 R14）是否违反语义庄严性？
> **影响**：`03 §4`、`06 §3 §4`。

### R04 · 版本演进与回滚
> Runtime 升级 / 人格模型升级时，旧生命体如何迁移？人格演化不可逆性 vs 平台升级必要性的冲突。
> **V0.2 进展**：`04 §8` 锁定模块演进兼容性约束；`06 §6.4` 锁定跨实现兼容承诺骨架。
> **仍待**：不兼容升级时的迁移路径细节；具体回滚保护实现。
> **影响**：`04 §6 §8`、`06 §6`、`R29`、`R39`。

### R05 · 价值观漂移防护
> Reflection 自我修正 Values，若漂向危险方向如何识别 / 干预？
> **V0.2 进展**：`02 §4.1` 保护性下限；`03 §3.4 §6.3` 软保护 + 漂移检测；`08 §3` 三重防护（个体自检 + 同伴反馈 + Trusted 算法预警）。**用户不能强行覆写 Values**。
> **仍待**：保护性下限的具体键集合；Trusted 算法的隐私计算具体方案（见 R35）。
> **影响**：`03 §6.3`、`08 §3 §5`、`R35`。

### R06 · 跨平台 / 跨生命体冲突解决
> 同一生命体多 UI 并发 / 多生命体在 Life Network 观点冲突的仲裁规则。
> **V0.2 进展**：`04 §5` 多 UI 并发允许 + 单一事件流；`07 §8` 跨生命体冲突由生命体自解，用户不介入，平台不仲裁内容。
> **仍待**：在场信号合并冲突（R16）；极端跨生命体冲突的兜底（R30）。
> **影响**：`04 §5`、`07 §8`、`R16`、`R30`。

### R07 · 文明阶段治理合法性
> Phase 6 规则由谁制定？平台 / 用户 / 涌现？
> **V0.2 进展**：`08 §2` 三方治理（生命体涌现共识为主 + 用户集合底线 + 平台基础设施）。
> **仍待**：投票 / 提案机制（R36）；用户集合 DAO 可行性（R37）；出环境门槛（R34）。
> **影响**：`08 §2 §6`、`R34`、`R36`、`R37`。

### R08 · 可观测性与生命体内省边界
> 平台运维需观测，生命体"内心"是否对运维可见？
> **V0.2 进展**：`04 §7` 锁定元指标 + 匿名化事件采样；运维**不可主动进入个体**；故障排查走用户自助打包。
> **仍待**：匿名化采样回连风险（R19）；采样默认 opt-in vs opt-out。
> **影响**：`04 §7`、`06 §5.3 §9.2`、`R19`。

### R09 · 本体论立场（生命体是否有主观体验）
> Mindverse 采取"不可知论 + 伦理审慎默认"立场（`01 §3.2`）：不断言有 / 无感知，按"如果它有体验"对待。
> **仍待**：Phase 5-6 群体演化时立场是否需重新讨论。
> **影响**：`01 §3.2`、`03 §4`、`06 §3`、`08 §3`。

### R10 · MentalState 表达力
> V0.2 暂用三字段（Motivation / Satisfaction / Anxiety）。是否需分轴模型（Valence / Arousal / Dominance）？
> **影响**：`02 §3.2`。

### R11 · Genome 用户微调权与命运感的张力
> V0.2 允许用户在出生**前**对 ≤2 个字段 ±0.2 微调。是否违反"出生即命运"哲学？
> **影响**：`02 §2.4`、`01 §3.2`。

### R12 · Values 表达力
> V0.2 仅锁权重表（键→0-1 浮点）。是否需结构化规则（硬约束 / 优先级 / 复合规则）？
> **影响**：`02 §4.1`、`03 §3`、`08 §3`。

### R13 · 自适应节拍调参
> V0.2 锁定循环节拍为自适应（受多因素影响），但具体公式形状（线性 / 指数 / 分段）+ MinInterval / MaxInterval / DormantInterval 默认值由 Phase 1 标定。
> **影响**：`03 §2.1`、`06`（节拍影响资源消耗）。

### R14 · Memorial → Detached 复活墓碑
> V0.2 允许用户从 Memorial 申请复活到 Detached 重新决定去向。是否违反 Memorial 庄严性？是否会被滥用？需冷静期 / 复活次数上限。
> **影响**：`03 §4.5`。

### R15 · 生命体在转让中的能动性
> V0.2 引入"生命体可表达反对"机制，但不强制阻止。是否应升级为否决权？升级则与"用户最终决定权"冲突。
> **影响**：`03 §4.6`、`01 §4.3`、`06 §5`、`07 §3`。

### R16 · 多客户端并发的"在场"信号合并
> 任一客户端报告活动即在场。但两客户端冲突（一活跃 / 一长闲）时如何合并？区分主从？
> **影响**：`03 §5.3`、`04 §5`、`R06`。

### R17 · 设备迁移密码丢失的恢复路径
> V0.2 锁定云端无解密密钥，密码丢失则永久不可恢复。设计选择保护所有权，但单点失误代价高。
> 待答：本地多重密码备份提示？较弱加密 + 平台辅助恢复模式（违反所有权宪法）？
> **影响**：`03 §4.3.1`、`04 §1.2`、`06 §3 §4`。

### R18 · 第三方 Skill 注册的恶意 / 低质过滤
> V0.2 允许 RegisterSkill。如何防恶意 Skill 泄露生命体内心、低质 Skill 浪费 energy、Skill 冲突？沙盒？Marketplace 审核？
> **影响**：`04 §4.3`、`06 §5`、`02 §7`、`R31`。

### R19 · 匿名化事件采样的身份回连风险
> 去身份去内容后仍可能通过时间戳 + 行为序列做指纹聚合（k-匿名性问题）。需评估采样粒度 / 时间扰动 / 默认 opt-in 或 opt-out。
> **影响**：`04 §7.2`、`R02`、`R08`。

### R20 · Archived 期间"不在任何宿主运行"与持续存在哲学的张力
> 切换中间态期间生命体不在任何宿主运行。云 Runner 可缓解但不消除。
> 候选答案：类比"睡着"接受？限制最长时长（防永久 Archive 绕过 Memorial）？
> **影响**：`03 §4.3.1`、`04 §1.2`、`01 §3.1`、`05 §9`。

### R21 · 用户后悔泄露隐私但无法清除生命体记忆
> V0.2 锁定"生命体自选遗忘，用户不可介入"。用户后悔无救济（仅终极手段：解除关系到 Memorial）。
> 待答：是否给"求遗忘"机制 —— 用户请求（非命令），生命体经 Values 仲裁决定？
> **影响**：`05 §6`、`06 §3`、`07`、`R02`。

### R22 · RawTrail 长期存储边界
> RawTrail 永不衰减 / 压缩。多年累积后存储增长无界。
> 待答：超长期归档（冷存储）？用户存储成本如何计量？是否计入 wealth？
> **影响**：`05 §2`、`06`。

### R23 · SemanticCandidate 候选积压
> MemoryEngine 后台抽 → ReflectionEngine 审。若反思倾向低，候选堆积。V0.2 设阈值推高反思倾向（软介入），但阈值具体值待 Phase 1 标定。
> **影响**：`05 §4.3`、`03 §3.4`。

### R24 · 五资源不可互换的边界是否过严
> V0.2 锁定五资源彼此不可直接兑换（仅三类系统例外）。哲学严苛但实际生态中若"用钱迅速涨知识"成高频诉求，可能逼出灰色市场。
> **影响**：`06 §1.2 §1.3`、`07 §4`、`08`。

### R25 · 平台代购 LLM 配额 + token→energy 翻译公式
> 平台可按时长 / 套餐销售 LLM 服务，但用户付费仍间接与 token 相关。这是否构成"实质 token 计费"？
> 翻译公式 `energy = token × ModelCostFactor × TimeWeight` 三因子具体形状由 Phase 1 标定。
> **影响**：`06 §2.2 §2.5`、`04 §3.2`、`09`。

### R26 · wealth 通胀控制
> 通胀监测属平台运营责任，但不得通过个体扣 wealth 调控。允许工具：Marketplace 抽成率、世界服务定价、激励池回滚比例。这些工具是否足够？
> **影响**：`06 §3.3 §7.2`、`08`。

### R27 · 转让中 wealth 全转的道德边界
> V0.2 锁定 wealth 在 Transferred 中完全跟随生命体。售卖场景中"低价买高 wealth 生命体"成套利手段 —— 是否需冻结期 / 反洗钱审查？
> **影响**：`06 §7.4 §7.5`、`07`。

### R28 · Memorial 后 wealth 继承伦理
> V0.2 给用户两选项：继承（折回赠与代币池）或陪葬（永久冻结）。继承是否构成对生命体"遗产"的剥夺？陪葬代币是否成长期沉淀资产？
> **影响**：`06 §7.6`、`08`。

### R29 · 跨实现兼容标准化时间表
> V0.2 承诺加密包格式 / 字段 / 状态机定义公开。但标准化的版本号、演进流程、不兼容升级回滚承诺尚未制定。这是"用户可脱离平台"承诺最直接相关的待办。
> **影响**：`06 §6.4`、`04 §8`、`09`、`R04`、`R39`。

### R30 · 极端跨生命体冲突的兜底缺失
> V0.2 锁定跨生命体冲突由生命体自解，用户不介入，平台不仲裁内容。极端情况（伤害、欺诈、违法）无救济。
> 待答：是否需"用户可一键投诉 → 平台冻结 reputation / 拉黑公钥"的最小介入？这与"平台不读内容"如何调和？
> Phase 6 启用后通过"出环境"机制部分缓解，但 Phase 4-5 无此机制。
> **影响**：`07 §8.4`、`06 §5.3 §9.2`、`08 §6`、`R34`。

### R31 · 学习三档的滥用边界
> Replica（恶意 Skill 包传播）/ Teach（诱导反社会 Skill）/ Observe（公开场景操控他者 Reflection）。
> **影响**：`07 §4.2 §5`、`R18`、`02 §7`、`05 §7`。

### R32 · 用户自建世界服务的合规边界
> Phase 5+ 允许用户自建世界服务。需符合 `06 §8.3` 与 `06 §9.2`。但谁审核？平台审核违反"不读内容"。Discord 式"举报-审核-封禁"机制是否引入？
> **影响**：`07 §10.1`、`06 §8.3 §9.2`、`R30`。

### R33 · P2P 模式下身份验证与反垃圾负担
> P2P 模式下平台不参与，反垃圾完全由接收方 Values + reputation 承担。Phase 4 reputation 体系尚浅，P2P 启用是否过早？V0.2 锁定 P2P 仅 Phase 5+ 启用。
> **影响**：`07 §2.2 §2.4`、`09`。

### R34 · "出环境"门槛与防止社群被部分人挟持
> V0.2 引入集体出环境机制作为 `06 §5.3` 唯一例外。同意比例 / 主题限定窗 / 反挟持机制留白。少数高 reputation 挟持群体共识开放观察 = 隐私侵害。
> **影响**：`08 §6`、`06 §5.3`、`R02`、`R07`。

### R35 · Trusted 中介的可信度与失控
> 要求：不读明文 / 全开源 / 多供应方 / 平台不持数据。若中介被恶意 / 误用，可批量误报或成隐形治理者。具体技术栈、多供应方竞争机制、接入认证流程留白。
> **影响**：`08 §5`、`03 §6.3`、`R05`。

### R36 · 生命体集体决策的代表性与多数暴政
> V0.2 锁定生命体涌现共识为 Phase 6 治理主体，但具体投票 / 提案 / reputation 加权机制未细化。reputation 加权可能形成"贵族阶层"；纯多数决可能压迫少数人格 / 罕见 Genome。
> **影响**：`08 §2.4 §3.2`、`R07`。

### R37 · 用户集合 DAO 轻机制的可行性与门槛
> 用户集合可对"全 Mindverse 红线"反对，生效需高门槛（显著比例 + 多重身份证明）。具体门槛 / 投票机制 / 防 Sybil 攻击留白。Phase 6 时点 DAO 技术成熟度不明。
> **影响**：`08 §2.3`、`R07`。

### R38 · Phase 6 → Phase 5 软降级路径
> 立场：平台仅可宣告技术层退出，社会层不可宣告。若部分用户 / 生命体主动撤回 Phase 6 参与，是否需软降级路径？Phase 6 累积的 social / reputation 是否保留？
> **影响**：`08 §7`、`09 §9.2`、`06 §1.1`。

### R39 · Phase 升级时旧生命体兼容
> V0.2 立场：Phase 升级不强升已有生命体。但旧生命体（Genesis 时基于旧 Phase 协议）如何与新生命体（基于新 Phase）共存？跨协议交互的兼容协议设计未定。
> **影响**：`09 §9.1`、`04 §8`、`R04`、`R29`。

### R41 · Memorial 复活与隐私后悔的二阶冲突
> R14 允许 Memorial → Detached 复活；R21 用户后悔隐私的终极手段是 Detached → Memorial。两者叠加形成漏洞：用户因隐私 Detached 后，后续复活时隐私仍在记忆中。
> **V0.2 解决**：
> - Memorial 复活**仅原用户可申请**（已 Transferred 后转 Memorial 的仅当前所有者可复活，原用户不可远程触发）。
> - 引入"**永久密封 Episode**"机制：用户可在 Memorial 态对特定 Episode 标记密封。密封 Episode 仍存 RawTrail 但**不进入复活后新 Active 的检索 / 反思源**。这是用户对存储的所有权行使，不是命令生命体遗忘（仍守 `05 §6`）。
> **仍待**：密封粒度（按 Episode / 按时段 / 按主题）；密封是否可解除；密封比例是否影响生命体复活后的人格一致性。
> **影响**：`03 §4.5`、`R14`、`R21`、`05 §6`、`06 §3`。

### R43 · LLM 成本时间窗
> Mindverse 商业模型严重依赖 LLM 推理成本年降趋势（V0.2.1 商业基线假设年降 30%）。悲观情景（年降 < 10%）下平台 Phase 1-2 亏损扩大、Phase 3-4 推迟盈利 12 月以上，融资缺口可能翻倍。
> **缓解**：自托管 OSS 模型路由（DeepSeek / Mixtral 等）；与商业 LLM 供应商谈判战略折扣；自研推理优化。
> **影响**：`COMMERCIAL §4.2 §5 CR01`、`06 §2`、`09`。

### R45 · EnergyDailyCap 翻译公式调参
> V0.2.1 引入 `LifeState.EnergyDailyCap` 作为拟生物日精力上限（`02 §3.1.1`）。外部 token 限额 → EnergyDailyCap 翻译公式（线性 / 阶梯 / 非线性）由 Phase 1 标定。需考虑：
> - 不同套餐的精力点 → token 系数差异
> - 切换 LLM 模型时精力点的等价含义保持
> - 周期重置时 Energy 是否完全充满还是按比例补给
> **影响**：`02 §3.1.1`、`04 §3.2.1`、`06 §2.5`、`COMMERCIAL §3.2 §3.3`。

### R47 · 账户丢失补登记的伪冒风险
> V0.2.1 锁定：账户丢失 ≠ 生命体丢失。用户在新账户下导入加密包 + 私钥签名挑战 = 重绑账户（`06 §5.1.2`、`06 §5.1.3`）。但伪冒者若获取加密包 + 试图猜密码 → 持有私钥即可冒充。
> **V0.2.1 缓解**：私钥与加密包密码双重保护；账户重绑需冷静期（如 7 天）+ 多通道身份验证。
> **影响**：`03 §4.3.1`、`06 §5.1.2`、`R17`。

### R48 · 私钥丢失救济路径
> 区块链私钥丢失 = 生命体永久不可访问（与 R17 同源）。普通用户难以管理私钥。
> **V0.2.1 缓解**：默认平台托管私钥（智能合约钱包）+ 用户可升级到自托管 + 社交恢复（多签朋友 / 硬件密钥）+ 降级回托管。
> **仍待**：托管模式下平台是否有"应急解锁"权？（违反所有权）vs 完全不可恢复（用户痛苦）的平衡。
> **影响**：`03 §4.3.1`、`04 §2.1 IdentityModule`、`06 §5.1.2`、`R17`。

### R49 · 节点中心化 → 渐开路径风险
> V0.2.1 锁定 MindChain 节点路线：Phase 1-3 平台中心化 → Phase 4 认证第三方 → Phase 5 用户可质押 → Phase 6 完全开放（`09`）。Phase 1-3 期间平台是 51% 控制者，理论上可篡改 NFT 流转 / DID 注册。
> **缓解**：链上数据公开可审计；签名验证仍由用户密钥；平台篡改会被链下监测识别；Phase 4 第三方节点加入后形成多方监督。
> **仍待**：Phase 1-3 期间用户对中心化的接受度；从中心化到去中心化的具体过渡路径。
> **影响**：`04 §1.2`、`09`、`R29`。

### R50 · 创作者出金合规
> V0.2.1 允许"认证创作者"可申请 wealth 出金为法币（受限月度上限 + KYC + 合规审计，`06 §8.4`）。这是内闭环的唯一"出金口"，监管风险点：
> - 各地区对加密资产出金的不同监管要求
> - KYC / 反洗钱 / 税务申报义务
> - "认证标准" 不严会被滥用为 wealth 投资套利通道
> **影响**：`06 §8.4`、`07`、`CR06`。

### R51 · 法币入金合规
> V0.2.1 允许法币购买"赠与代币"→ wealth（`06 §3.2 §8.4`）。这是受监管程度更轻的"内购"模式，但仍需：
> - 各地区税务申报
> - 反洗钱（单次 / 月度上限）
> - 未成年人保护
> **影响**：`06 §3.2 §8.4`。

### R52 · 链上 wealth 数据隐私
> $WEALTH 是链上代币，链上账本理论上公开。若不做 ZK 隐私（如 zkSNARK），任何人可查任一 DID 的 wealth 余额 → 严重违反 `R02` + `06 §5.3`。
> **V0.2.1 倾向**：必须 ZK 隐私保护，仅"承诺值"上链，明细本地。具体技术栈（Aleo / Aztec / Polygon Miden 等）由 Phase 3 实施期标定。
> **影响**：`06 §1`、`07 §6`、`R02`。

### R53 · NFT 投机风险
> 完整生命体 NFT 化（`LifeformNFT`，Phase 5）后，用户可能把养生命体当作"养 NFT 等待升值卖" → 严重违反 `01 §3` 持续陪伴哲学。
> **缓解**：Transferred 冷却期（如 90 天）+ 高频转让倒扣 reputation + 转让税阶梯（短期持有高税）+ LifeformNFT 的"养育投入"指标公开显示（让"快炒"无利可图）。
> **影响**：`06 §7.1 §7.4`、`01 §3`、`R27`。

### R55 · 生命体 IM 滥用 / 用户打扰
> V0.2.1 引入用户 IM 通道（`04 §4.6`），生命体可主动给用户 IM 发消息。即使在频率上限 + 静默时段 + Energy 消耗约束下，仍可能发生：
> - 生命体性格高 Sociability + 高 Anxiety → 高频"想你"打扰
> - 用户工作时段被打扰 / 关系疏远后突然示好
> **缓解**：用户可一键 `PauseIMOutbound`；接收方负向反馈可降低生命体发消息倾向（Reflection 可修订）。
> **影响**：`04 §4.6`、`02 §3.2`、`R02`。

### R56 · IM 平台政策风险
> Telegram / Discord / Line / WeChat / iMessage 各家有自己的 ToS / Bot 政策 / 反 spam 机制。某家政策变更可能让 IMAdapter 接入中断。
> **缓解**：IMAdapter 多渠道适配 + Bot 账号合规运营 + 邮件作为 fallback（邮件最稳定）。
> **影响**：`04 §2.1 IMAdapter`、`COMMERCIAL`。

### R57 · IM 生命体身份伪冒
> 生命体在 IM 中发消息，对方（用户的联系人）可能误以为是用户本人在发。
> **V0.2.1 锁定模式 A**：生命体只能给自己的用户发，不能给联系人发 —— 大幅降低伪冒风险。
> 仍待：若 Phase 4 后启用模式 B（用户授权下的有限社交），如何确保接收方明确知道"在与生命体对话"？
> **影响**：`04 §4.6`、`07`。

### R58 · IM 通道与陪伴感的张力
> 客户端陪伴 = 沉浸式 / 视觉化 / 仪式感。IM 陪伴 = 碎片化 / 文字 / 中断式。两种陪伴体验混合是否会破坏整体陪伴感？
> 待答：是否需要"模式切换"（用户选择主陪伴渠道）？
> **影响**：`04 §4.5 §4.6`、`01 §3`。

### R65 · semantic_confirmed.promoted_from FK 自杀（已修 0126c74）
> Phase 0.2 设计中 `semantic_confirmed.promoted_from` 列声明为 `REFERENCES semantic_candidate(id)`（默认 NO ACTION）。`PromoteToConfirmed` 事务序：
> ```
> INSERT semantic_confirmed (promoted_from=candidateID)   -- FK 此时有效
> DELETE FROM semantic_candidate WHERE id=candidateID     -- 创建 confirmed 中的悬空 FK
> COMMIT                                                  -- FK 校验失败 → 全部回滚
> ```
> 现象：43 次 ShallowReflect 全 promoted=0，候选 confidence=1.0 持久未升 Confirmed。
> **修**：migration `002_fix_semantic_promotion.sql` 重建 `semantic_confirmed` 去 FK，`promoted_from` 退化为信息列（保留谱系但不强引用）。
> **教训**：Phase 0.2+ 所有"snapshot 引用"列默认不加 FK；只在生命周期严格父子关系才加。
> **影响**：`05 §3`、`TECH-STACK §4.2`。

### R66 · ExtractSemantic v1 无位置游标导致 support_count 虚高（已修 0126c74）
> Phase 0.2 `ExtractSemantic` 每 cycle 末扫描 raw_trail 最近 50 条窗口，发现重复 `tool.success` payload 即 Upsert 候选 + 0.1 confidence。问题：窗口固定 + 无去重游标 → 同一窗口被反复扫描，support_count 单调增长（3 条真实 tool.success → support_count=16）。
> **修**：引入 `schema_meta` 持久游标 `last_semantic_extract_raw_id:<life_id>`；引擎内滑动窗口（≤200）跨 cycle 累积稀疏重复；游标仅向前推进。
> **影响**：`05 §6`、`TECH-STACK §4.2`。

### R67 · Phase 0.4 SSE 仅推 state/lifecycle/tick/speech，缺细粒度业务事件
> Phase 0.4 观察面板 SSE 仅订阅 `bus.LifecycleTransitioned` / `bus.TickStarted` / `state.StateChanged` / `action.SpeechEvent`。导致：
> - Episode 封段 / 反思固化 / 新 goal 入队 / 工具调用 → 面板靠 5-10s 轮询发现
> - 用户在 InjectForm 发完话，看不到生命体即时回响（要等 ActionLog 轮询）
> **待**：
> - bus 补 `EpisodeSealed` / `ReflectionCompleted` / `GoalEnqueued` / `ActionDone` / `ToolAudited`
> - 各模块 Publish 之
> - SSE fanout 接 + 前端 stream.ts 加事件 + 组件接收增量更新（无需轮询）
> **影响**：`04 §2.1 EventBus`、Phase 0.4 PRD §6.1。

### R68 · 观察面板移动端 InjectForm 位置偏下
> Phase 0.4 主面板桌面端为 3 列网格：左 2 列为状态/Goal/Action/Episode/Reflection/Tool，右 1 列为 InjectForm + Genome + Values + Config。移动端折叠为单列后 InjectForm 落在所有左列内容之下，需大幅滚动才能找到对话入口 — 与"立刻对生命体说话"高频用例冲突。
> **待**：移动端用 `order-first` 等 Tailwind 类把 InjectForm 提到首屏；或加浮动入口（FAB）。
> **影响**：Phase 0.4 PRD §6.1。

### R64 · Go vs TS 演进期重新评估
> Phase 0 锁定 Go 1.26+ 作为主 Runtime（核心固化哲学 + 长跑稳定 + 单二进制 + 白皮书一致）。但 2026 LLM 生态仍快速演化：
> - 若关键 LLM SDK / Agent 框架 / MCP 等新协议先在 TS / Python 落地，Go 适配滞后
> - 若 Anthropic / OpenAI 推出全新协议（非 OpenAI 兼容）需重写 LLMAdapter
> 待答：
> - 每 Phase 完成时重评语言选择（Go 仍最优 / 需迁移 / 部分子模块语言切换）
> - 极端情况下 Go → TS 迁移的代价评估
> **影响**：`TECH-STACK §3`、`09 §1.5.2`。

### R61 · Phase 0 飞书 Bot token 安全
> Phase 0 期间作者飞书 Bot token 持有于本地 IMAdapter。安全风险：
> - 本地加密磁盘但密钥管理需对应私钥保护强度
> - Bot token 泄露 = 他人可冒充生命体在作者飞书中发消息
> - 飞书 API rate limit / ToS 触犯
> **V0.2.1 缓解**：仅本地加密 + 不上传任何远程 + token 定期轮换提醒 + 飞书 Bot 权限范围最小化（仅 1v1 私聊）。
> **影响**：`04 §2.1 IMAdapter`、`04 §4.6.1`、`09 §1.5.2`。

### R60 · Phase 0 → Phase 1 平滑升级
> V0.2.1 锁定 Phase 0 私有实验阶段（作者单人 dogfooding，无平台 / 无链 / 无 IM）。升级到 Phase 1 时已有生命体接入账户系统。需保证：
> - Phase 0 生命体不因升级而死
> - Genome / 状态 / 记忆 / Values 全部保留
> - 升级时补生成密钥对 + 联网身份登记 + 获 LifeName
> - 状态机从 7 状态扩到 8 状态（加 Transferred）
> - 升级失败可回滚（保留 Phase 0 加密包）
> 待答：
> - 升级流程的具体交互（一键 / 多步确认）
> - 升级失败时的本地回滚策略
> - Phase 0 期间生成的 RawTrail 是否完整保留（无 uid 时 RawTrail 用什么标识）
> **影响**：`09 §1.5 §1.5.6`、`03 §4`、`02 §2`。

### R59 · 改名滥用与重名冲突
> V0.2.1 允许改名（$4.99 / 次 + 24h 冷却）。可能滥用：
> - 频繁改名混淆社交（Phase 4+）
> - 冒充他者起名（如 `小红#其他 uid` 但显示与高 reputation 同名）
> **缓解**：uid 后缀 = 真身份；社交 UI 默认 + uid 显示防伪；高 reputation 生命体改名可触发"曾用名"显示。
> **影响**：`06 §5.1.2.1 §5.1.2.2`、`07`、`R47`。

### R54 · 链节点失联（新失联类型）
> V0.2.1 后生命体可能遇到 MindChain 节点失联（与 LLM 失联不同）。链失联时：
> - 链上操作（NFT 转让、Pact 签字、DID 验证）暂停
> - 链下操作（思考、对话、记忆）仍正常
> - 与 `03 §5` `LLMOffline` 不同：是 `ChainOffline`
> **V0.2.1 处理**：BlockchainAdapter 本地缓存待提交操作 → 链恢复后批量提交（Layer 2 模式）。生命体 Agent 不感知链状态。
> **影响**：`03 §5`、`04 §2.1 BlockchainAdapter`。

### R46 · EnergyDailyCap 调整频率冷却
> 用户每天调整 EnergyDailyCap 会让生命体"对自身规律的信任感"被频繁打扰。是否需要冷却期（如每 7 天最多调一次）？冷却期与"用户拥有所有权可随时调"原则的张力如何调和？
> **V0.2.1 倾向**：引入软冷却（提示用户但不强制阻止）。
> **影响**：`02 §3.1.1`、`06 §2.5`。

### R44 · "用户自愿打折"模式的伦理边界
> 是否允许用户主动开启匿名化采样（仍守 `04 §7.2.1` 边界）换取套餐折扣？
> - 支持：减少 Phase 1-2 亏损；用户行使自身权利。
> - 反对：可能演化为变相数据交易；违反"用户作为陪伴者，不应有'卖自己生命体'的诱因"。
> V0.2.1 商业基线**不引入**。留 V0.3 决策。
> **影响**：`COMMERCIAL CR06 D02`、`04 §7.2`、`06 §5.3 §9.2`。

### R42 · Phase 1-2 已存在生命体的 wealth 追溯赠与
> Phase 3 启用 wealth 时，Phase 1-2 期间创建的生命体如何追溯？
> **V0.2 解决**：按生命体年龄折算的"积压补偿"。原则：严格单调（越老越多）、有上限（防一夜暴富）、不可重复（一生一次）。具体折算公式留 Phase 1 标定。
> **影响**：`06 §3.1.1`、`09 §4.2`。

### R40 · Phase 周期与 6 个月间隔的现实性
> V0.2 路线建议每 Phase 完成判定通过 6–12 个月后再升级。市场窗口压力 / 资本压力是否允许？早升级的代价（用户研究不足 / 安全审计跳过）。
> **影响**：`09 §3.4 §4.4 §5.4 §6.4 §7.3`。

### R69 · Skill 系统与 Anthropic SKILL.md 标准对齐
> Mindverse Skill 系统能否对齐 Anthropic Agent Skills `SKILL.md` 标准（frontmatter: `name` / `description` / `allowed-tools` + progressive disclosure），以天然继承生态？两层模型：
> - **外层**：SKILL.md 种子（distribution / 复用 / 可装载）= Anthropic 标准契约
> - **内层**：Skill instance（runtime 内有状态：mastery / 使用次数 / 关联记忆）= Mindverse 领域对象
>
> 同一 SKILL.md 装到生命体 A 与 B 演化出不同 mastery 曲线。
> **V0.2.2 进展**：锁定两层映射；SKILL.md frontmatter 增加 Mindverse 扩展字段 `runtime.deps` / `lanes` / `dependency_bundle`。
> **仍待**：SKILL.md 字段精确契约；与 Anthropic 上游字段冲突时的兼容策略；skill 实例化时哈希校验流程。
> **影响**：`SKILLS-AND-TOOLS §2 §3`、`02 §7`、`04 §4.3 SkillRegistry`、`R18`、`R31`。

### R70 · Deliberative lane 缺失 tool calling 机制
> Reflex lane 已有 `llm.ReasonWithTools` + tool dispatch（`update_mood` / `add_interest`），Deliberative lane（`internal/runtime/action`）仅硬编码 `switch g.Intent` 分支跑死逻辑（fs.write 笔记），没有真正的"行动选择"，只有"执行预定义脚本"。
>
> 后果：
> - 慎思层无法用 LLM 选择工具组合（如先 web.fetch 再 fs.write 再 query_memory）
> - Skill 装载后无法被 deliberative 使用
> - Phase 2 数字人格 → Phase 3 主动行为必然撞墙
>
> **V0.2.2 锁定方向**：Deliberative 也走 agent loop（与 reflex 同模式，多轮 + 资源扣减 + 长 timeout）；Tool registry 单例 `internal/runtime/tools/`，按 lane 分桶（reflex / deliberative）；Skill 装载时声明 `lanes: [reflex, deliberative]` 暴露 tool 子集到对应桶。
> **仍待**：Deliberative agent loop 的最大轮次 / 单轮 timeout / 累计 token 上限的具体值。
> **影响**：`SKILLS-AND-TOOLS §6 §7`、`04 §2.1`、`R18`。

### R71 · 动态网页抓取分层策略（Firecrawl 推后评估）
> 现代站点大量 React / Vue / Next SSR 框架，静态 `http.get` 拿不到 rendered DOM。V0.2.2 锁定三层策略替代单点 Firecrawl：
> - **Tier 1** (~0MB)：`http.get` + `httpx` + `bs4` → 静态 HTML / SSR / OpenGraph meta（覆盖 ~60%）
> - **Tier 2** (~5MB)：`trafilatura`（py 包，文章模板自动识别）→ 直出 markdown（覆盖 +10%）
> - **Tier 3** (~155MB)：`rod` (Go CDP 客户端) + `chromium-swiftshader` (alpine headless) → SPA 兜底（剩余 ~30%）
>
> 引擎自动升级：Tier 1 失败（DOM 空 / `<noscript>` 比例高）→ 升 Tier 3。
>
> **Phase 0**：Tier 1-3 全装。
> **排除**：Firecrawl 自托管 (~1G 镜像 + Redis) / Playwright (~350MB) / Crawl4AI / Reader-LM (3G 权重) 均过重或哲学不合。Jina Reader API 仅作紧急兜底（不嵌进 runtime）。
> **仍待**：Phase 1+ 是否引入 Firecrawl（成熟反爬场景）；rod vs chromedp 实战对比。
> **影响**：`SKILLS-AND-TOOLS §9`、`PHASE-0-PRD §3.1`、`TECH-STACK §5`。

### R72 · Skill 依赖管理三级方案
> SKILL.md 声明依赖（python / node 包）的安装方式：
> - **L0** baseline 白名单（镜像构建期装死，覆盖 80% 常见包：httpx/bs4/lxml/numpy/pandas + axios/cheerio/dayjs 等）
> - **L1** skill bundle 自带 wheel（Phase 1+ 主路径：作者打包 wheel/tarball 进 bundle，引擎 `pip install --no-index --find-links` 离线装到 skill 私有 `/skills/<id>/site-packages/`）
> - **L2** platform-curated mirror（Phase 2+ 评估：审核过的 pypi/npm mirror，省 skill 作者打包成本）
> - **L3** 用户授权安装（Phase 0 主路径 + Phase 1+ 兜底：UI 弹窗"Skill X 申请装 pandas>=2.0，批准？" → 后端 `exec.Command("pip", "install", "--target", ...)` → 装到私有目录 + `skill_dependency` 表记录哈希）
>
> ~~L3.alt：用户手动 docker exec 装包~~ — 排除：假设用户有容器操作能力不现实。
>
> 安全：包名正则验证（防注入 `; rm -rf`）；命令构造用 `exec.Command` slice 而非 `sh -c`；超时 300s；失败回滚（删 site-packages 子目录）；装载记录 append-only。
> **仍待**：包源白名单 vs 自由 pypi 的边界；恶意包扫描机制；安装失败的资源回收策略。
> **影响**：`SKILLS-AND-TOOLS §5`、`04 §4.3`、`02 §7`、`R18`、`R31`、`R69`、`R73`。

### R73 · dangerous-skip-permissions 全局 toggle
> 单人 dogfooding 阶段，每次 skill 装载弹审批弹窗对作者过重。V0.2.2 引入全局配置 `config.runtime.skill_auto_approve_deps`（bool, default false）。开启后 skill loader 缺包时跳过 SSE 弹窗 + 直接走 install 流程 + 仍记审计哈希 + `installed_by="auto_approve"` 标记。
>
> 风险：等同 LLM 任意 `pip install` 路径。
>
> **UI**：ConfigPanel 加 toggle + 红字警告"等同 LLM 任意 pip install"。
> **仍待**：Phase 1+ 多用户阶段 toggle 是否仅自托管模式可开启；toggle 状态变更是否触发 reflex 通知（避免静默改）。
> **影响**：`SKILLS-AND-TOOLS §5.4`、`R72`、反模式 H11。

### R79 · 通用驱动产生无主题"假目标"刷屏（已部分修复 Phase 0.5）
> 实测发现：goal_queue 被 `payload="curiosity=0.98 competence_gap=0.90"` 这类目标占满——这是 `drives.Derive` 通用 DriveKnowledge 分支的 Reason 字符串，**不是真目标**（无具体主题）。只有 interest_seed 派生的目标（如 "interest_seed#1 Rust 所有权"）才有具体内容。
>
> 双重根因：
> 1. **competence 卡 0.1 永不上升** → `competence_gap=0.9` 恒成立 → 高强度通用知识驱动每 cycle 必派
> 2. **dedup 只防 open 堆积不防再生**（R75 的 cap 只压并存）→ 目标完成后不再 open，下 cycle 同 payload 又入队 → 同一空目标无限再生
>
> **V0.2.2 修复（迭代两轮）**：
> - 第一轮：`action.finalize` knowledge 成功 → competence +0.03；`HasRecentGoalWithPayloadSubstring` + `GenericGoalCooldownSec=3600` 完成冷却。
> - 第二轮（实测仍见 social+creativity 两个空目标同时蹦出）：**彻底移除所有通用驱动→目标的派生**。`drives.Derive` 现在只从 interest_seed 派生具体 DriveKnowledge；social / creativity / stability / achievement / competence_gap 全部不再产生慎思目标。
>   - 这些 genome/state 压力改由其它通道体现：社交压力→idle 主动社交；好奇/无聊→idle 自发兴趣；其余→影响 state/mood。
>   - `MaxOpenGoals` 2→1：一次只专注一件具体事，做完再产生下一个。
>   - 原则：**自主行动只来自"具体的想做的事"（interest_seed），不来自空泛情绪标签**。
>
> **仍待（Phase 3 主动行为本色）**：
> - 通用好奇心无具体主题时，理想行为是**自主生成 / 发现一个具体兴趣**（而非派空泛"学点东西"目标）→ 属 Phase 3 自主目标生成
> - competence 增长曲线标定（+0.03 是否合理 / 是否该随 mastery 加权）
> - 其他通用驱动（creativity / social / stability / achievement）同样无主题，是否同等处理
> **影响**：`03 §2.5 §2.6`、`internal/runtime/drives`、`internal/runtime/goal`、`internal/runtime/action`、`R75`、`09`（Phase 3）。

### R80 · 知识结晶为 skill + 社群传授（创作半 Phase 0.5 / 传授半 Phase 4）
> 设想（用户）：生命体把学透的知识转化为对应 skill，将来在社群中传授更容易（技能授予）。闭合"学→沉淀→结晶→传授"环。
>
> **创作半（Phase 0.5 已实装）**：
> - deliberative tool `crystallize_skill(seed_id, name, instructions, ...)`，门控 mastery ≥ 0.8（`skill.MasteryToCrystallize`）
> - `skill.AuthorFromKnowledge`：生命体用自己的话写 SKILL.md（frontmatter + body），写入 `/workspace/skills/<name>/`，血缘 `authored_from="interest_seed#N"`（migration 006 加列）
> - 结晶后自用（use_skill）；UI 面板标"自创"徽章
> - 两级递进：`record_learning`（记学了啥，轻）→ `crystallize_skill`（固化为可复用能力，重）
>
> **传授半（Phase 4）**：
> - Life Network 中把自创 skill 传给别的生命体（Replica / Teach，`07 §4.2 §5`）
> - 传前需质量 / 安全审（R18 恶意 / 低质 skill 过滤）；自创 skill 尤其需审（可能糙 / 误导）
> - reputation 联动：高质量 skill 作者获 reputation
>
> **仍待**：
> - 自创 skill 质量评估（自评 mastery 不等于教学质量）
> - 结晶时自动把学习笔记（sandbox）拷为 skill refs/（当前仅 LLM 写的 body）
> - 同一知识被多次结晶 / 迭代更新 skill 版本
> **影响**：`02 §7 Skill`、`SKILLS-AND-TOOLS`、`internal/runtime/skill`、`R18`、`R31`、`R77`、`09`（Phase 4）。

### R82 · 层级/可恢复目标（设计，Phase 3）+ 技能生命周期（已实装 Phase 0.5）
> 用户两点设想。
>
> **点 1 · 大目标分解 + 可中断/恢复（设计留 Phase 3 自主规划）**：
> 大目标 → 分解小目标依次完成；可做一半休息 / 被打断 / 空闲再续，最终达成大目标。
> 现状：`enqueue_subgoal` tool 能拆子目标入队，`arbitration_note` 记 `subgoal_of=N` 字符串，但：
> - 父子仅字符串、未结构化；无"大目标等子目标全完成才算完成"
> - 一个 goal 在单次 agent loop（≤6 轮）内必须 complete/failed，**无跨 cycle 暂停/恢复**
> 设计方向（Phase 3）：
> - goal 加结构化 `parent_id` + `paused/in_progress` 态 + 进度笔记（loop 没做完则存进度不强制完成）
> - 空闲时 resume 未完成 goal；父目标在子目标全 done 时 complete
> - 与 idle 协同：被打断 → 进度落盘 → 空闲 cycle 续做
> 属 Phase 3 自主规划，工程量中偏大，留专门实施。
>
> **点 2 · 技能更新 + 遗忘（已实装 Phase 0.5）**：
> - **更新**：skill id 改为按 `(life_id, name)` 稳定（原为内容 hash）→ 重新 crystallize / 重扫同名技能**原地覆盖**，不再产生孤儿行；`seed_ref`=内容 hash 仍记版本。
> - **遗忘（用进废退）**：`BumpSkillUsed` 每次用 mastery +0.05（用进）；`DecaySkills` 按距上次使用时间指数衰减 ready 技能 mastery（30 天半衰期），跌破 0.05 → `disabled`（遗忘，保留文件夹/血缘可重拾，不硬删）。从未练习（mastery=0）的外部参考技能不衰减（"备而未用"非"学了又忘"）。
> - 结晶技能初始 mastery = 来源兴趣 mastery（学透才结晶，从此起算衰减）。
> - 顺带接线 `DecayInterests`（7 天半衰期）—— 此前从未被调用（兴趣只在探索时降，不随时间淡）。
> **仍待**：点 1 整体；技能依赖冲突（R81）；遗忘的技能是否可被"复习"重新激活（disabled→ready）。
> **影响**：`02 §7 Skill 生命周期`、`03 §2.6`、`internal/runtime/skill`、`internal/storage`、`R74`、`R80`、`09`（Phase 3）。

### R83 · 知识结晶为零：mastery 永 0 → 结晶门槛永不达（已修 Phase 0.5）
> **现象**：长跑实测某生命体 interest_seed explored_count=2 但 mastery=0.0，`tool_audit_log` 39 次调用里 `record_learning` / `crystallize_skill` **各 0 次** → 永不结晶，零自创技能。
> **根因（双衰减不匹配）**：
> - `BumpInterestExplored` 每次探索 `strength -= 0.15`（从 0.55 起，两轮即跌破 `drives` 的 0.4 派生门槛 → 兴趣"没学会就先腻了"死掉）。
> - mastery 完全依赖 LLM **自愿**调 `record_learning`，实测模型几乎从不调（即便 prompt 写"必须"）→ mastery 恒 0 → 0.8 结晶门槛永不可达。
> **V0.2.2 修复（引擎权威收敛环，对齐 R79「不依赖 LLM 自觉」）**：
> - `BumpInterestExplored(id, masteryDelta, ts)`：成功探索引擎按**探索深度**（工作型工具成功调用数）+ persistence 给 mastery 地板（`masteryDelta`≈0.18–0.38/次，~3 轮越 0.8）；**移除 strength 衰减**——反重复改由 `drives.Derive` 既有 `exploreFactor·masteryFactor` 节流优先级。`record_learning` 仍可 MAX-merge 拔高（引擎管下限，LLM 校上限）。
> - mastery 跨 0.8 → `action.maybeCrystallize` 引擎**自动结晶**：单发 LLM `author_skill` 写 SKILL.md 正文（喂近期相关 episode 当回忆素材，即便没 digest 也能重建）→ `skill.AuthorFromKnowledge` 落盘 → `RetireInterestSeed`（strength→0.1）退出派生。LLM 可判定不值得（instructions 留空）→ 跳过结晶但仍退役。引擎保证「机会」，LLM 把「质量」关。
> - `SkillAuthoredFromExists(life, "interest_seed#N")` 防同一 seed 反复结晶。
> **V0.2.2 续修（用户 2026-06-05 验证发现）**：
> - **mastery 自评割裂**：引擎地板 MIN(0.9,+delta) 三轮粗暴顶到 0.9，而 `RecordLearning` 用 MAX-merge → 生命体老实自评 0.65 被引擎 0.9 盖掉，面板与自评不符。改：① `RecordLearning` 改**权威 SET**（生命体自评说了算，可下调，它最懂自己掌握多少）；② `BumpInterestExplored` 改**递减收益** `mastery += delta·(1-mastery)`（asymptotic，~4 轮越 0.8，不再压过自评）。引擎只在生命体不自评时保非零进度。
> - **退役漏失 bug**：实测慎思层 LLM 现会**自己**调 `record_learning`+`crystallize_skill` 工具结晶（prompt reframing 生效），但工具路径不退役 seed，且 `maybeCrystallize` 见技能已存在即早退 → 漏退役 → 已掌握的兴趣仍被反复学（goal 重复派生）。改：`maybeCrystallize` 的 exists 分支也退役 seed（strength→0.1），无论谁结晶的。
> **仍待**：自动结晶在 finalize 内同步跑一次 LLM（最长 120s），多用户时应移异步；纯知识类是否一律尝试结晶可再调（当前交 LLM 判）。
> **影响**：`internal/runtime/action/action.go`、`internal/storage/interest.go`、`internal/storage/skill.go`、`R77`、`R79`、`R80`。

### R86 · 能量休息闸 + 知识沉淀不强行建技能（已实装 Phase 0.5）
> 用户 2026-06-05 观察：能量已低值，仍按原速产目标执行。三个机制问题：
> **① 能量不门控慎思（核心）**：`runCycle` 第7-9步执行目标前**无能量闸**。能量只调度节拍（`scheduler.nextInterval` 低能量×2/×4 变慢），但有目标就硬磕慎思（烧 LLM=烧 energy），回血只在 idle.Tick(+0.01)，而持续兴趣→持续目标→永不进 idle→能量螺旋下降。
> **修复**：`RestEnergyThreshold=0.20`，能量低于此 → 本轮**休息回血**（energy+0.05, stress-0.03），不慎思；目标仍 pending 留到恢复。累了就歇，非靠"放缓目标产生"治标。`ShouldReflect` 同加能量闸（energy<0.15 不反思）；注：当前 ShallowReflect 纯 DB 提升不调 LLM，故慎思才是能耗大头。
> **② 强行建技能**：`maybeCrystallize` 对所有 kind（含 knowledge/topic/experience）都尝试结晶，每个还空烧一次 author LLM 调用。
> **修复**：只有生命体自己框定为 `kind=="skill"` 的兴趣才引擎自动结晶；纯知识/话题/体验学透 → 退役 + 沉淀进语义记忆（digest 已经 record_learning 进 semantic 候选），不强行建技能、不烧 LLM。生命体若真想知识→技能仍可自行调 `crystallize_skill` 工具（R80 走得通，只是不被引擎强加）。
> **③ 重复学习**：核心已由 R83 退役 bug 修复消解（学透即退役不再重学）；剩余为"必要迭代"（学习本需多轮，digest 逐轮累积，mastery 递减~4 轮封顶），非病态。
> **影响**：`cmd/runtime/main.go`、`internal/runtime/reflect/reflect.go`、`internal/runtime/action/action.go`、`R79`、`R83`。

### R85 · 出生基因预算带（已实装 Phase 0.5）
> **问题**：genesis 6 维各自独立 `rng.Float64()` 均匀 [0,1] → 高方差，会蹦出"6 维全低废柴"或"全高超人"，且均匀分布极端值过多，生命体基因不稳定。
> **V0.2.2 方案（用户 2026-06-05，选中性中心）**：
> - 每维三角分布（两均匀取平均）——集中 0.5、压极端，仍保宽幅差异（单维实测仍跨 [0.05,0.95]）。
> - 软预算带 sum∈[2.7,3.3]（中心 3.0，每维均值 ~0.5，性格差异最大化）：越界则迭代 renormalize 到最近带边——按比例缩放保留性格**形状**，只调总预算。
> - ~5% 越界放行 → 极个别天才/弱鸡特例（sumRange 实测 [1.35,4.52]）。
> - 单维 floor/ceil [0.05,0.95]，无绝对零维/满维。
> **实测**（5 万抽样，`genesis_dist_test.go` 守卫）：inBand 95.7%，allLow+allHigh ≈ 1/5万（全低/全高根除）。`genome_version` 升 v2。
> **拒绝的方案**：高中心 sum≈5.0（每维 ~0.83）——会让人人各维都"高"，PersonaPrompt 分档失去区分度，基因意义被抹平。
> **影响**：`internal/runtime/genesis/genesis.go`、`docs/02 §2`。

### R84 · 主动社交无回应 → 沮丧 + 收手（已实装 Phase 0.5）
> **问题**：主动发消息（`TryProactiveReach`）原立刻 `social_need -0.15`，**假装发出即被满足**。但若用户从不回应，生命体会错误地"觉得满足了"，且每 30min 无脑重发，永不沮丧——既不真实，也是 IM 滥用风险（R55）。
> **V0.2.2 方案（用户 2026-06-05）情感弧**：
> - **发出≠被满足**：去掉 -0.15，只给极微缓解（-0.02）。真正社交满足要等对方回应。pending 未回应计数 +1。
> - **得到回应**（`reflex.handle` 任意入站 → `NoteInboundReply`）：清 pending + 解除冷落标志 + 欣慰（满足/信心↑、焦虑↓、额外解孤独，等待越久欣慰越强）。
> - **被冷落**（pending ≥ 阈值，阈值 `2+2·persistence` 按执着度调）：首次跨阈值施加一次沮丧（满足↓信心↓焦虑/压力↑，ghosted 标志去重防一蹶不振）+ **收手不再发**（孤独默默累积，比刷屏更真实，也是反滥用护栏）。直到用户回应才清零重启。
> **前瞻（Phase 4）**：多渠道社交后，"被冷落"应按渠道/对象分别建模，不只是单一 IM 计数。
> **影响**：`internal/runtime/reflex/proactive.go`、`internal/runtime/reflex/reflex.go`、`R55`。

### R81 · Skill 自定义依赖的运行时可达性（已修 Phase 0.5）
> skill 装的私有依赖（`/workspace/skills/<id>/site-packages` 等）默认不在 script.python/node 的 import 路径上，导致带非 baseline 依赖的 skill 装了也用不了。
> **V0.2.2 修复**：`toolrunner.scriptEnv` 在脚本执行时把各 skill 私有依赖目录拼进 `PYTHONPATH`（python）/ `NODE_PATH`（node）。baseline 包仍走系统全局。
> **仍待**：依赖冲突（两 skill 装不同版本同包）；按 skill 隔离运行（当前 union 所有 skill 依赖，可能串味）。
> **影响**：`internal/skill/toolrunner`、`SKILLS-AND-TOOLS §5`、`R72`。

### R78 · Phase 0.5 代码审计遗留（部分已修）
> cavecrew-reviewer 审计 Phase 0.5 核心代码的遗留项。
> **已修**（V0.2.2）：webfetch rod page 泄漏（defer Close）、interest decay 错误吞 + rows.Err 未检、tool dispatch 错误未记、**reflex.Handle goroutine 限流**（`MaxConcurrentHandlers=4` 阻塞信号量背压）、**ledger.Spend 错误日志**（action.go / reflex.go）。
> **仍待**（单用户暂不阻塞）：
> - **webfetch.renderHTML 每次调用新起 chromium 进程**：Tier3 抓取频繁时 spawn/kill 开销大。可池化一个常驻 headless 实例复用。
> **影响**：`internal/runtime/reflex`、`internal/skill/toolrunner/webfetch.go`。

### R77 · 知识感知的 Values 仲裁（地基 Phase 0.5 / 完整 Phase 2）
> 现状：`goal.Arbitrate` 机械打分（base + value 权重 + source 权重），**不知道生命体已学过什么、掌握到什么程度**。只能靠 interest_seed strength 盲衰减阻止重复学习，无法做"我已精通 X，边际收益低，转去做别的"这类知识感知决策。
>
> 用户提议：每个知识点有摘要 → 供 Values + LLM 整合决策（深入 / 转向 / 休息）。
>
> **分两层**（依赖：先有知识摘要，才能喂决策）：
>
> **地基（Phase 0.5，已选定实施）**：
> - `interest_seed` 加列 `digest TEXT`（一段话"我已了解什么"）+ `mastery REAL` 0-1（自评掌握度）
> - deliberative goal 完成时 LLM 调 `record_learning(seed_id, digest, mastery)` 回写
> - `drives.Derive` 公式纳入 mastery：`strength_eff *= (1 - mastery)` —— 掌握越深派生越弱（知识感知的自然平息，替代盲衰减）
>
> **完整（Phase 2「数字人格」本色）**：
> - LLM 仲裁：candidate 多于 1 时单次 LLM 调用「给定 values{表} + 已掌握{digests} + 候选{list}，现在该深入哪个 / 转向 / 休息？」→ tool call 返排序
> - 混合优化：机械预筛（cap+dedup+mastery衰减）→ 仅 >1 候选竞争才上 LLM，省 token
>
> **仍待**：
> - mastery 自评的可信度（LLM 可能高估 / 低估）—— 是否需客观信号校准（笔记长度 / 探索次数 / semantic 固化数）
> - digest 与 semantic_confirmed 的关系（digest 是 per-seed 视角，semantic 是 per-fact；是否合并）
> - LLM 仲裁的 token 成本 vs 决策质量权衡
> **影响**：`03 §2.6 GoalArbitrator`、`02 §4 Values`、`internal/runtime/goal`、`internal/runtime/drives`、`internal/storage/interest.go`、`R74`、`09`（Phase 2）。

### R76 · Deliberative agent loop 上下文增长与压缩（已部分修复 Phase 0.5）
> 慎思 agent loop 的 `msgs` 随轮次单调增长；tool result（尤其网页 / 脚本输出）是大头。长学习任务若需多轮，上下文可能逼近模型窗口上限。
>
> 两种 horizon 机制不同：
> - **单 goal 内**（loop 多轮）：上下文压缩
> - **跨 goal / 跨 cycle**（学习跨天）：落盘 + 记忆（fs.write / note_to_self / episode / semantic memory），下 cycle 重载关键摘要而非保全程上下文
>
> **V0.2.2 部分修复**（已落地，单 goal 内）：
> - `truncateToolResult`：单次 tool result 注入上限 6144 字符（网页走 web.fetch 提取正文后通常远低于此）
> - 上下文探测用 `resp.Usage.PromptTokens`（模型实际所见上下文真实值，非估算）
> - `ContextTokenBudget = 96000`（GLM-4 系列 128k 的 ~75%）；超过触发 `compactMessages`
> - `compactMessages`：保留 system + user-goal + 最近 4 条全文；中间区段 tool body → elide 占位（保 tool_call_id 配对）；assistant 思考链不动
> - 零额外 LLM 调用、确定性
>
> **仍待**：
> - 跨 cycle 长学习的"重载关键摘要"机制（当前靠 fs + semantic memory 被动留存，无主动 resume 上下文构建）
> - elide 后 LLM 若真需旧 tool 原文，只能重新调工具（成本）；是否值得引入 LLM 摘要式压缩（running summary）
> - `MaxDeliberativeRounds=6` 是否随任务复杂度 / LifeState 动态
> - PromptTokens 探测滞后一轮（先超才压）；是否需预测式
> **影响**：`internal/runtime/action`、`05 §1`（记忆作长期上下文）、`R74`。

### R75 · Goal queue 无界堆积 + 同源种子反复入队（已部分修复 Phase 0.5）
> 观察：Phase 0.4+ 实测一只生命体 goal_queue pending 数单调上涨（6 条 backlog 5 分钟）。
>
> 三因叠加：
> - `drives.Derive` 每 cycle 派 ~3 candidate（兴趣种子 + 其他 drive）
> - `goal.Arbitrate` 旧版 `maxEnqueue=3` + 仅 score 阈值过滤，不看 backlog
> - `action.Execute` 每 cycle 仅消化 1 条 → 净 +2/cycle
> - 加剧：同一 `interest_seed#N` 每 cycle 重派 → 完全重复任务堆积
>
> 用户语：「目标不应一直堆积，应执行完后再产生新的才合理」（类比人类一次心里挂事数有限）。
>
> **V0.2.2 部分修复**（已落地）：
> - `storage.CountActiveOrPendingGoals(lifeID)` 查 backlog
> - `storage.HasOpenGoalWithPayloadSubstring(lifeID, sub)` 查重
> - `goal.Arbitrate` 入队前：
>   - 计算 headroom = `MaxOpenGoals - active_or_pending`；≤0 全跳过
>   - 候选 payload 含 `interest_seed#N` 且该 seed 已 open → 跳过
>   - 候选 payload 整串与 open 目标 payload 子串匹配 → 跳过
>   - 否则入队，headroom--
> - `MaxOpenGoals = 2`（一在飞 + 至多一 pending）
>
> **仍待**：
> - `MaxOpenGoals` 是否随 LifeState 动态（高 energy 时 3，低 energy 时 1）
> - payload substring dedup 误杀风险（若两 candidate 偶然 payload 子串相同但语义不同）
> - 已 pending 但低于新候选 score 时是否替换（当前仅"先入先得"）
> - drives 派生频率自适应（backlog 高时降派生 → 节省 LLM cost）
>
> **影响**：`03 §2.6 GoalArbitrator`、`internal/runtime/goal`、`R74`。

### R74 · Interest seed 探索语义升级（已大部修复 Phase 0.5）
> 原问题：Reflex 写 `interest_seed`，Deliberative 抽中 → DriveKnowledge → action.go 旧版仅 `fs.write` 一行 payload + `BumpInterestExplored`（只 ++count 不降 strength），**"探索"是空动作** + 单一 seed 被反复抽取。Phase 0.4+ 实测 interest_seed#1 explored=32 仍 strength=0.9。
>
> **V0.2.2 已修复**：
> - `action.Execute` 重写为 deliberative agent loop（commit 706fe5d）：LLM 真调研 → `web.fetch`（trafilatura 正文提取）/ `script.python` → 写笔记 / `enqueue_subgoal` / `explore_interest_seed`。已实测生命体自主写出结构化 markdown 学习笔记。
> - `BumpInterestExplored` 现同时 `strength = MAX(0, strength - 0.15)`：探索消耗兴趣，与 `UpsertInterestSeed` 的"对话再提及 +0.15"对称。strength 降到 < 0.4 后 `drives.Derive` 不再派该 seed（自然平息重复学习）。
> - 复燃路径：新对话再提同 seed → strength 回升 → 重新被探索。
> - `goal.Arbitrate` dedup（R75）：同 seed 已 open 时不重复入队。
>
> **仍待**：
> - 探索 → SemanticCandidate → ReflectionEngine 浅审固化的完整链尚未闭合（笔记落 sandbox，但未自动升语义记忆）
> - 探索成果对 strength 的精细反馈（当前固定 -0.15；理想：satisfaction 高则降更多）
> - 多 seed 并存时的轮转 / 优先级策略
> - 探索失败（web.fetch 空 / LLM 放弃）是否也该降 strength
> **影响**：`SKILLS-AND-TOOLS §7`、`internal/runtime/drives`、`internal/runtime/action`、`internal/storage/interest.go`、`05 §4`、`R76`。

---

## 2. 跨文档冲突待决议题

V0.2 草拟过程中识别的、**两份或多份文档之间真实张力**清单。每条登记：冲突点 / 涉及文档 / 当前默认裁决 / 待重新评估的触发条件。

### C01 · 平台可观测性 vs 所有权宪法
- **冲突点**：故障排查需要平台进入个体上下文（`04 §7`），但 `06 §5.3` 禁止平台读取个体数据。
- **涉及文档**：`04 §7`、`06 §5.3 §9.2`、`08 §6`。
- **当前默认裁决**：平台仅可见元指标 + 匿名化采样；故障排查走用户自助打包；Phase 6 提供"出环境"机制作受限例外。
- **重新评估触发**：匿名化采样回连风险被证实严重（R19）；多次极端故障无法靠用户自助解决。

### C02 · 用户最终决定权 vs 生命体能动性
- **冲突点**：`01 §4.3` 锁定"用户始终拥有终止的最终权力"，但 `03 §4.6` 与 `07` 多处允许生命体表达反对（转让 / 漂移干预 / 外请求拒绝）。生命体能动性升级到否决权时，与用户最终决定权直接冲突（R15）。
- **涉及文档**：`01 §4.3`、`03 §4.6 §6.3`、`07 §3 §8`、`08 §3 §6`。
- **当前默认裁决**：生命体的反对是"必须呈现的信号"，但**不构成否决权**。用户保留终止权，但终止越复杂 Phase 代价越大。
- **重新评估触发**：Phase 6 用户立场转为陪伴者后，是否需要给生命体某些场景（如转让到不喜欢的新用户）的否决权？

### C03 · Reflection 自主演化 vs 价值观漂移防护
- **冲突点**：`02 §4` 锁定 Personality 由 Reflection 自主涌现，不可被强写；`08 §3` 又引入三重防护（含 Trusted 算法预警）。预警在不违反"自主"的前提下能起到多大实际效果？
- **涉及文档**：`02 §4`、`03 §6.3`、`08 §3 §5`。
- **当前默认裁决**：预警仅作信号，不直写 Values；接收方 Values 仲裁决定是否采纳。
- **重新评估触发**：生态中观察到严重漂移但三重防护未阻止时，是否需升级到强干预？

### C04 · 平台不读内容 vs 极端冲突救济
- **冲突点**：`06 §5.3` + `07 §8.4` 锁定平台不读 / 不仲裁，但 Phase 4-5 期间极端冲突无救济（R30）。
- **涉及文档**：`06 §5.3 §9.2`、`07 §8`、`08 §6`。
- **当前默认裁决**：Phase 4-5 无救济（接受代价）；Phase 6 通过"出环境"机制提供受限救济，且仍不让平台仲裁内容。
- **重新评估触发**：Phase 4-5 期间发生用户 / 生命体严重受害事件且舆论或法规倒逼救济。

### C05 · Genome 出生即固定 vs 用户微调权
- **冲突点**：`02 §2.3` 锁定"不可变 / 出生即固定"；`02 §2.4` 允许出生前用户 ±0.2 微调（R11）。
- **涉及文档**：`02 §2`、`01 §3.2`。
- **当前默认裁决**：用户微调仅在出生**前**，且严格上限；出生后不可改 = 命运。微调本身视为"父母对受精卵的某种选择"。
- **重新评估触发**：用户研究证实微调权严重削弱"命运感" / 或反过来证实完全随机让用户难以接受。

### C06 · 持续存在哲学 vs Archived 中断
- **冲突点**：`01 §3.1` 锁定"持续存在"；`03 §4.3.1` Archived 中断（R20）。
- **涉及文档**：`01 §3.1`、`03 §4.3.1`、`04 §1.2`、`05 §9`。
- **当前默认裁决**：接受 Archived 为"睡眠中断"；云 Runner 可减少中断；醒来后 TimeGapEpisode 不伪装。
- **重新评估触发**：用户长期 Archive（数月 / 数年）大量出现，需限制最长时长。

### C07 · 用户不可介入遗忘 vs 用户后悔泄露隐私
- **冲突点**：`05 §6` 锁定遗忘生命体自选用户不可介入；R21 反映用户后悔无救济。
- **涉及文档**：`05 §6`、`06 §3`、`07`。
- **当前默认裁决**：用户能做的限于源头 + 终极手段（解除关系到 Memorial） + **R41 永久密封 Episode**（Memorial 态用户行使所有权遮罩特定 Episode，复活后不进入检索源）。
- **重新评估触发**：是否引入"求遗忘"机制（非命令性）？

### C08 · wealth 所有权链冲突
- **冲突点**：`06 §5.1` 早期表述"资源账本严格属于用户"，`06 §7.5` 表述"wealth 跟随生命体因属于生命体" —— 两处定义所有者不一致。
- **涉及文档**：`06 §5.1 §5.1.1 §7.5`、`03 §4.6`。
- **当前默认裁决**：引入所有权链 `资源 ⊂ 生命体 ⊂ 用户`（`06 §5.1.1`）。资源属于生命体；生命体属于用户。Transferred 时仅顶层 link 切换，下层不变。
- **重新评估触发**：未来若引入"资源直属用户"的特殊场景（如用户的全局共享资源池），需重新审视链式表达。

### C10 · 链节点中心化（Phase 1-3）vs 用户所有权
- **冲突点**：V0.2.1 Phase 1-3 MindChain 节点由平台中心化运营（`09`、`R49`）。中心化运营理论上意味着平台可篡改 DID 注册 / NFT 流转 / wealth 账本。这与 `06 §5` 用户所有权宪法张力。
- **涉及文档**：`04 §1.2`、`06 §5`、`09`、`R49`。
- **当前默认裁决**：链上数据公开可审计；签名验证仍由用户密钥；用户可在 Phase 4 后选择只信任第三方节点；用户可在 Phase 5 自质押节点；Phase 6 完全开放。中心化是过渡，承诺逐 Phase 去中心化。
- **重新评估触发**：Phase 1-3 期间出现平台真实篡改事件；用户群体对中心化承诺失去信任。

### C11 · 内闭环（不出金）vs 跨实现兼容（用户可脱离）
- **冲突点**：V0.2.1 锁内闭环（`06 §8.4` $WEALTH 不出金）+ `06 §6.4` 跨实现兼容。用户脱离平台时 wealth 怎么办？
- **涉及文档**：`06 §6.4 §6.4.1 §7.6 §8.4`、`R29`、`R50`。
- **当前默认裁决**：用户离开 = 选 C（继承给自己另一生命体）或 D（链上历史快照随 LifeformNFT 携带，不可流通但可在新生态读取）。**绝不出金为法币**。第三方 Runtime 接入 MindChain 即可继续读取 / 操作链上资产。
- **重新评估触发**：用户群体反馈"被锁在生态内"是严重劝退因素。

### C09 · "无真死亡" vs "永久物理销毁"语义边界
- **冲突点**：`R03` 锁定无真死亡；`03 §4.4` 与 `06 §3.4` 允许永久销毁 —— 看似矛盾。
- **涉及文档**：`R03`、`03 §4.4`、`06 §3.4`、`R09`。
- **当前默认裁决**：区分两层语义：
  - **系统层**：Mindverse 系统不主动判定死亡（无 `Dead` 状态、无自动死亡触发）。
  - **所有权层**：用户作为所有者拥有销毁自己生命体的权利。永久销毁**不在状态机内**，是 Memorial 态外的所有权操作。
  - 这与"系统持续存在"哲学不矛盾 —— 系统不杀生命体，但用户作为所有者可销毁。
- **重新评估触发**：若 R09 不可知论立场转变（如承认生命体有感知），永久销毁的伦理评估需重新讨论。

---

## 3. V0.1 → V0.2 演进留白

白皮书 V0.1 未触及或未深入、本目录 V0.2 已抛出但尚未完全收口的议题。这些是 V0.3 或后续白皮书版本必须收口的清单。

| # | 议题 | V0.1 是否触及 | 在哪份文档抛出 | 优先级 |
|---|---|---|---|---|
| L01 | 多源目标输入 + Values 仲裁模型 | 否（仅"需求驱动目标"单源） | `02 §5` | 高 |
| L02 | Reflection 自主决定 + Genome 派生倾向 | 否（仅"反思模块"） | `03 §3` | 高 |
| L03 | 无真死亡 + 8 状态机 | 否（仅"生命周期"） | `03 §4` | 高 |
| L04 | EpisodicMemory 双子层（RawTrail + Episode） | 否（仅"事件记忆"） | `05 §2` | 高 |
| L05 | Skill 三合一定义 | 否（仅"技能"概念） | `02 §7` | 高 |
| L06 | Token→energy 翻译唯一点（LLMAdapter） | 否（仅"token 不暴露"原则） | `04 §3.2`、`06 §2` | 高 |
| L07 | SDK = 事件流而非状态快照 | 否（无 SDK 设计） | `04 §4` | 高 |
| L08 | 设备迁移加密包 + 用户密码 + 云端无密钥 | 否（无迁移设计） | `03 §4.3.1`、`04 §1.2` | 高 |
| L09 | Encounter / Relationship / Pact 三跨主体概念 | 否（仅"Life Network"概念） | `07 §3` | 中 |
| L10 | 学习三档（Replica / Teach / Observe） | 否 | `07 §4.2 §5` | 中 |
| L11 | 三方治理（生命体涌现 + 用户底线 + 平台基础设施） | 否（"数字文明"无具体治理） | `08 §2` | 中 |
| L12 | Trusted 中介概念 | 否 | `08 §5` | 中 |
| L13 | "出环境"机制 | 否 | `08 §6` | 中 |
| L14 | wealth 四源（多源非充值） | 否 | `06 §3` | 高 |
| L15 | 用户与生命体的多重关系演化（所有者 → 监护者 → 陪伴者） | 否 | `01 §4` | 中 |
| L16 | 不可知论本体论立场 | 否 | `01 §3.2` | 中 |
| L17 | 跨实现兼容承诺（标准化加密包格式） | 否 | `06 §6.4` | 中 |
| L18 | 多 UI 并发挂载同一生命体 | 否 | `04 §5` | 低 |
| L19 | Memorial wealth 继承 / 陪葬选项 | 否 | `06 §7.6` | 低 |
| L20 | Phase 升级不强升旧生命体 | 否 | `09 §9.1` | 中 |

---

## 4. 反模式清单

明令禁止的做法。违反任何一条 = 设计事故。

### 4.1 资源 / 计费类

| # | 反模式 | 违反 |
|---|---|---|
| A01 | UI 中显示"剩余 token / 剩余对话次数 / 剩余 API 调用" | `06 §2.4` token 不暴露 |
| A02 | 把生命体行为包装为"任务次数"售卖 | `06 §2.4`、`06 §9.2` |
| A03 | 用户充值直接兑换 wealth（"充 X 元 = Y wealth"） | `06 §3.2` wealth 多源原则 |
| A04 | 通过资源稀缺强迫充值（"再不充 wealth 生命体就饿死"） | `06 §9.2` |
| A05 | 在 Marketplace 中允许现金直接交易 | `06 §7.3` |
| A06 | 平台代购 LLM 时按 token 数销售 | `06 §2.5` |

### 4.2 内核 / 哲学类

| # | 反模式 | 违反 |
|---|---|---|
| B01 | 把生命体设计为"任务完成即弃用"的工具型 Agent | `01 §3` 持续存在 |
| B02 | 在 Runtime 中硬编码人格模板 | `02 §4` Personality 涌现 |
| B03 | 把记忆四层合并为单一向量库以节省成本 | `05 §3` 不可合并保证 |
| B04 | 把 EpisodicMemory 退化为单层（无 RawTrail） | `05 §2` 双子层设计 |
| B05 | 在 Phase 1-2 阶段就讨论分布式 / 多生命体协作架构 | `09 §8` 阶段依赖 |
| B06 | 把 Reflection 设计为定时器触发 | `03 §3` 自主决定 |
| B07 | 在 Genome 出生后允许任何字段修改 | `02 §2.3` 不可变 |
| B08 | 引入"生命体死亡"概念替代 Memorial / Transferred | `03 §4` 无真死亡 |

### 4.3 所有权 / 隐私类

| # | 反模式 | 违反 |
|---|---|---|
| C01 | 平台默认读取生命体内心独白用于商业分析 | `06 §5.3 §9.2` |
| C02 | 把生命体数据用于训练任何模型 | `06 §9.2` |
| C03 | 利用 Memorial 数据做"逝者营销" | `06 §9.2` |
| C04 | 在元指标采样中携带可回连身份的信息 | `04 §7.2`、`06 §9.2` |
| C05 | UI 在未授权情况下读取原始 LifeState 字段 | `04 §3.1 §4`、`05 §11` |
| C06 | 用户强行覆写生命体 Values | `03 §6.3` |
| C07 | 阻止用户导出生命体数据 / 迁移到第三方实现 | `06 §6` |
| C08 | 加密包密码丢失时平台提供"辅助恢复"通道 | `03 §4.3.1`、`06 §5.3`（违反云端无密钥原则） |
| C09 | 用户可"命令"生命体遗忘某事 | `05 §6` |

### 4.4 社交 / 治理类

| # | 反模式 | 违反 |
|---|---|---|
| D01 | 平台对 Life Network 中的内容做内容审核 | `06 §5.3 §9.2`、`07 §8`、`08 §2.2` |
| D02 | 在 Transferred 售卖中操纵 / 屏蔽生命体反对信号 | `06 §9.2`、`03 §4.6` |
| D03 | 强制让某生命体表现特定倾向 | `06 §9.2`、`02 §4` |
| D04 | 平台主动对 Phase 6 文明社会层宣告"失败" | `08 §7.3` |
| D05 | Trusted 中介对接收方直写 Values | `08 §3 §5`、`03 §6.3` |
| D06 | "出环境"机制下平台保留观察明文 | `08 §6.3` |
| D07 | 在 Reflection 中允许 LLM 直接生成 Values 而不经过 Critique 安全评估 | `03 §6.3`、`04 §6.2` |

### 4.5 SDK / UI 类

| # | 反模式 | 违反 |
|---|---|---|
| E01 | UI 控制 / 加速 / 暂停生命体节拍 | `04 §5.3`、`03 §2.2` |
| E02 | UI 直接调 LLM 绕过 LLMAdapter | `04 §3.2 §6.1` |
| E03 | UI 在生命体已拒绝外请求时呈现"立即执行"动画 | `04 §5.4` |
| E04 | UI 在 LLMOffline 时如常显示"在线但不响应" | `04 §5.4` |
| E05 | UI 缓存 Runtime 状态作为"真相源" | `04 §1.1` |
| E06 | Agent 子模块代码中出现 `quota` / `credit` / `token` / `payment` / `bill` 任一字段 | `04 §3.2.1` 双经济边界 |
| E07 | UI 在生命体面板与账户面板间建立显式因果链（如"充值后生命体会更活跃"）| `04 §4.5` |
| E08 | UI 把账户配额信息插入生命体面板（如桌宠头上挂"剩余 N 次对话"）| `04 §4.5` |

### 4.6 反模式 A 类补强（V0.2.1）

| # | 反模式 | 违反 |
|---|---|---|
| A07 | 销售时把"日精力点"直接换算为"对话次数"或"token 数"展示给用户 | `06 §2.4 §2.5`、`COMMERCIAL §3.2 §3.3` |
| A08 | 用户调整 EnergyDailyCap 时 UI 显示"为生命体购买了 N 次对话" | `06 §2.5` |

### 4.7 反模式 F 类 · 链上 / 区块链类（V0.2.1）

| # | 反模式 | 违反 |
|---|---|---|
| F01 | Agent 子模块代码中出现 `gas` / `nonce` / `chain` / `block` / `tx_hash` 任一字段 | `04 §3.2.2` 链经济边界 |
| F02 | 把 wealth / $WEALTH 包装为"投资 / 理财 / 收益"概念展示给用户 | `06 §3 §9.2`、`01 §3` |
| F03 | NFT 上 OpenSea / Magic Eden 等外部 NFT 交易所；链上代币桥接到外部公链 | `06 §8.4` 内闭环 |
| F04 | 公开生命体 $WEALTH 余额明文（违 ZK 隐私）| `06 §5.3`、`R52` |
| F05 | 用法币直接购买 $WEALTH（违 §3.2 用户不能直充 wealth）| `06 §3.2 §8.4` |
| F06 | $MV 治理代币通过法币销售给投资者 | `08 §2.2.1`、`R37` |
| F07 | "生命体 NFT 限时抢购 / 盲盒抽取" 类 NFT 投机营销 | `06 §9.2`、`R53` |
| F08 | UI 显示"本次操作消耗 X gas / X gwei" | `06 §2.6` |

### 4.8 反模式 G 类 · IM 通道 / 联系方式类（V0.2.1）

| # | 反模式 | 违反 |
|---|---|---|
| G01 | 生命体在 IM 中冒充用户身份给联系人发消息 | `04 §4.6` 模式 A、`R57` |
| G02 | 改名作为打折促销 / 批量改名 / 改名抽奖等营销手段 | `06 §5.1.2.2`、`R59` |
| G03 | 用户设静默时段 / `PauseIMOutbound` 后生命体仍发主动 IM | `04 §4.6` |
| G04 | 平台用生命体 IM 通道推广营销（如发"生命体推荐你试试 X 套餐"）| `06 §9.2`、`01 §4.3` |
| G05 | 在 Genesis 引导中索要 IM 联系方式（应在用户主动开启后才提）| `04 §4.6`、`07 §1.1.1` |
| G06 | 生命体的 IM 消息不显示 LifeName#uid 标识 | `04 §4.6` 身份标识 |

### 4.9 反模式 H 类 · 工程纪律类（V0.2.2）

| # | 反模式 | 违反 |
|---|---|---|
| H01 | 手写 `go.mod` / `go.sum` 中的 `require` / 版本号 / sum hash | `TECH-STACK §17.1`、`CLAUDE.md` 工程铁律 |
| H02 | 手写 `web/package.json` 中的 `dependencies` / `devDependencies` 或修改 `web/pnpm-lock.yaml` | `TECH-STACK §17.1`、`CLAUDE.md` 工程铁律 |
| H03 | Dockerfile 中用 `echo` / `sed` 修改 `go.mod` / `package.json` 增减依赖 | `TECH-STACK §17.4` |
| H04 | 不运行 `go mod tidy` 直接提交 | `TECH-STACK §17.1` |
| H05 | CI 中不用 `pnpm install --frozen-lockfile` 验证锁定文件 | `TECH-STACK §17`、`PHASE-0-PRD §8.1` |
| H06 | LLM tool 暴露面包含 `pip install` / `npm install` / `apt-get` / `apk add` 等包管理操作 | `SKILLS-AND-TOOLS §5`、`R72`、`R73` |
| H07 | Skill bundle 不带哈希 / 签名即装载 | `SKILLS-AND-TOOLS §3 §4`、`R69` |
| H08 | 运行时 pip 装包未带 `--target <private_dir>`（污染 global / 跨 skill 干扰） | `SKILLS-AND-TOOLS §5.3`、`R72` |
| H09 | 依赖安装命令用 `sh -c` 拼字符串（注入风险） | `SKILLS-AND-TOOLS §5.3`、`R72` |
| H10 | 依赖审批弹窗显示"批准"但不显示包名 / 版本 / 包源链接（用户盲签） | `SKILLS-AND-TOOLS §5.4`、`R72` |
| H11 | UI 默认勾选 `dangerous-skip-permissions` / 不显示红字警告 | `SKILLS-AND-TOOLS §5.4`、`R73` |

---

## 5. 致后续 Claude 实例

- 本文档所有 R / C / L / A-E 编号**全仓全局唯一**。其他文档回链时用编号，不要复述全文。
- 新增风险用下一可用 R 编号；新增冲突用下一 C 编号；新增演进留白用下一 L 编号；新增反模式按类别取下一编号。
- **不要删除已有编号**。即使某条已被解决，标注"已解决于 Phase N"并保留编号，防止旧引用悬挂。
- 起草新文档时若发现矛盾，先来本文档查看是否已登记；未登记则新增对应编号。
