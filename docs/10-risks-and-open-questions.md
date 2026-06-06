# 10 · 风险与未决问题

> 本文档定位：已识别风险登记、跨文档冲突待决议题、V0.1 → V0.2 演进留白、反模式清单。**所有盲点在此集中，不分散在各文档**。
>
> **状态**：V0.2.2 草稿。共登记 ~100 项风险（R01–R104，其中 R62/R63 编号预留）。Phase 0.4+ 批 R69–R100 来自 reflex / skill / tool / 抓取 / 上下文 / 知识感知 / 发呆 / 社交 / 知识结晶 / 技能生命周期 / 长跑治理阶段。R101–R104 来自 `docs/11` 平台层架构评审（LLM 转发零留存信任 / 元数据侧信道 / token 翻译点 vs 计量点 04/06 协调 / Admin·Free 档可选性）。

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

### R89 · 社交节奏调校 + 主动消息视角 bug（已修 Phase 0.5）
> 用户 2026-06-05 观察：
> **① social_need 涨太快**：`idle.Tick` 每 tick `0.005+0.03·sociability`，cycle ~60s 几十轮就顶满 → 老想打扰用户。改 `0.0015+0.006·sociability`（~4× 慢）；genesis 基线 `0.3+0.4·soc`→`0.2+0.3·soc`（起点别贴阈值）。
> **② 满了却"没社交行为"**：实为主动消息已发（受 30min 冷却节流），但 R84 把"发出"设成只 -0.02 social_need、不算满足 → 用户不回则 social_need **永钉 1.0**，看着像没动作。改：主动发消息给**实质缓解** `social_need-0.12`（"表达了一下，没那么急了"），发完掉下阈值、自然拉长下次间隔、不再钉满。ghosting（被冷落收手）仍保留；回应仍有额外欣慰。
> **③ 主动消息视角 bug**：`composeProactiveMessage` 喂 `recentEpisodeContext` 的 raw 事件 dump（`idle.daydream×7`），LLM 误把"自己发呆"当成对方活动，发出"是你在……发呆？"。修：prompt 明确"这些是【你自己】的状态/活动日志（含自己 idle.daydream），别把自己做的事说成对方在做"。同类视角混淆见 [[reflex 自我活动注入]]（R88 dialogueHistory / selfActivityContext）。
> **仍待**：social_need 涨速宜按真实时间而非 tick（cycle 间隔可变）；30min 冷却可配。
> **影响**：`internal/runtime/idle/idle.go`、`internal/runtime/reflex/proactive.go`、`internal/runtime/genesis/genesis.go`、`R84`、`R82`。

### R90 · 对话历史 / 主动计数按会话隔离 + 会话自我标识（已实装 Phase 0.5）
> 用户 2026-06-05 指出：未来多渠道（飞书/钉钉/slack）多会话并存，对话历史与"主动发了几条没回"的计数**全是全生命体共享**——给某会话发 1 条，会因给别处发过而错说成"我发了 2 条怎么没回"；对话历史也把不同人/渠道串成一锅。
> **修**：以 `convoKey = channel|peer` 为作用域隔离一切会话态。
> - 历史：新增 `storage.RecentDialogueTurnsForConvo(channel,peer)`（按事件 payload 的 channel + 对端 from/to 过滤；received 看 from，speak/proactive_reach 看 to）。`reflex.dialogueHistory`、滚动概要 `dialogueSummary*`、主动消息 `composeProactiveMessage` 全切到会话版。`RecentDialogueTurns`（全局）仅留给面板观察。
> - 计数：`proactive_pending/ghosted/last` 三个 meta 键由 `:<lifeID>` 改 `:<lifeID>:<convoKey>`。`getPendingReaches/setPendingReaches/isGhosted/setGhosted/getProactiveLast/setProactiveLast/applyGhostDiscouragement/NoteInboundReply` 全带 `ck`。A 回我只清 A 的等待，不影响"还在等 B 回"。
> - 自我标识：`conversationContext`（对话）+ `whoAmITalkingTo`（主动）注入"这是哪个渠道、和谁会话"（`channelLabel` 机读名→飞书/钉钉/Slack/网页/命令行）。这是未来"上午和 B 说过了、再去提醒他"这类跨会话记忆/决策的入口。
> - **单聊 vs 群聊**（用户追问）：`IncomingRequest.ChatType` + `contact.chat_type`（migration 007）。对话方式随类型变——群聊不问"你是谁"、不每条都接话、要 @ 具体人、主动发声不用"怎么不回我"单聊口吻。`conversationContext`/`whoAmITalkingTo` 按 `direct`/`group` 分支。Phase 0 仅放行飞书 p2p 单聊（`ChatType="direct"`），群聊入站仍丢弃；群聊真正启用（群 id 作会话键 + 发言者归属 + @ 检测）留 Phase 4。
> **仍待**：主动社交频率闸现按会话（不会单方刷爆某人），但多联系人时缺**全局速率上限**（可能一轮对许多人各发一条）——Phase 4 升级 `TryProactiveReach` 为"遍历各会话、按 social_need/reputation/上次联系时机决策去提醒谁"时一并处理；当前只挑 `MostRecentContact` 单个。
> **影响**：`internal/storage/memory.go`、`internal/storage/contact.go`（新增 `GetContact`/`PeerKey`）、`internal/runtime/reflex/proactive.go`、`internal/runtime/reflex/reflex.go`、`R84`、`R88`、`R89`、Phase 4 `07`。

### R91 · 主动消息复读（低温确定性导致逐字重复，已修 Phase 0.5）
> 用户 2026-06-05 发现：两条主动消息内容**逐字相同**（17:49 与 18:21，间隔 ~32min 过冷却）。查实 raw_trail：id 557 与 684 完全一致，中间无用户回复（pending 1→2→3）。
> **根因**：`composeProactiveMessage` 每次 prompt 几乎不变（无人回 → 历史/pending 同），`llm.Reason` 走全局温度（偏低）→ 输出塌缩成同一句。即便历史已含上一条主动消息（assistant 轮，R90 已喂），prompt 没明令禁止复读，模型仍复读。
> **修**：
> - **防复读 prompt**：compose 的 nudge 点出上一条主动消息原文（meta `proactive_last_msg:<life>:<ck>`），明令"别说同样的话——换角度/换话头/聊新的"。
> - **去重守卫**：生成后 `normalizeMsg`（去空白）比对上一条；逐字重复则**不发**，但仍推进冷却 + pending++ → 朝收手推进。涌现效果：说不出新意就发一次后攒到阈值收手（"说过一次没回，算了"），而非每 30min 复读轰炸。
> - 对方一回（`NoteInboundReply`）清掉 last_msg：会话翻篇，下次主动不必回避旧话。
> **仍待**：`Reason` 无 per-call 温度；将来可给主动 compose 单独提温增加多样性（现靠 prompt + 守卫足够）。
> **影响**：`internal/runtime/reflex/proactive.go`、`R84`、`R89`、`R90`。

### R92 · 主动消息勿扰：静默时段 + 临时勿扰（已实装 Phase 0.5）
> 用户 2026-06-05：需能设"哪些时段不发消息"避免打扰；用户在对话里说"接下来1小时别打扰我"也要实际生效。补 PRD §7.2 / R55 的静默时段 TODO。
> **两道闸**（都在 `TryProactiveReach` 早退，不动 pending/冷却——"现在不合适"≠被冷落）：
> - **静默时段**（配置，全局，按用户本地时区）：`inQuietHours(now)`。config 键 `proactive_quiet_enabled/start/end` + `proactive_tz_offset_min`（分钟偏移，避容器 tzdata 依赖）。跨午夜支持（start>end）。面板可设（`/api/config/quiet`），`apiConfig` 回传当前值。单测覆盖跨午夜/同日/时区/空窗。
> - **临时勿扰**（按会话，用户对话触发）：新增 reflex tool `set_quiet(minutes)`——LLM 听懂"别打扰"就调（结构化走 tool-call，不解析自由文本，合 [[feedback_llm_structured_via_tools]]）。写 meta `proactive_snooze_until:<life>:<ck>`，未到点不主动发。上限 7 天。buildSystemPrompt 告知生命体有此工具。
> **要点**：勿扰只挡**主动消息**；用户主动来找，生命体照常回应（受邀≠打扰）。
> **仍待**：静默时段目前单窗口；多窗口 / 工作日区分留后。时区靠手填偏移（Phase 1 可由飞书用户资料/前端时区自动探测）。
> **影响**：`internal/runtime/reflex/proactive.go`、`internal/runtime/reflex/tools.go`、`internal/runtime/reflex/reflex.go`、`internal/storage/config.go`（int 配置）、`internal/io/httpapi/httpapi.go`、面板 ConfigPanel、`R55`、`R84`、`R90`。

### R93 · 重复学习：同一兴趣种子被冷启动重刷 N 次（已修 Phase 0.5）
> 用户 2026-06-05 长跑观察发现：13 条 deliberate 行动长得一模一样（`llm.agent rounds=6 tools=[query_memory,...]`）。查实 goal_queue：seed#1/#2 各探索 **4 次**、seed#4 4 次、seed#3 仅 1 次。再查 4 次的 action result——开场全是「我来探索这个主题，先检索已有记忆」，**每次从零重启、刨同一坨**，mastery 却照样按 substantive 涨到 0.80 结晶，磨出可能很浅的技能。
> **根因**：`buildUserMessage` 给 seed 目标喂了 mastery + digest，但 `record_learning`（写 digest）是**可选**、LLM 常不调 → 无续探记忆 → 冷启动；且 prompt 没说"这是第 N 次、别从头、往深/新角度"。`seedRecentContext`（过往 episode 回顾）只在结晶时用、没喂给探索。病根同 [[R91]]（主动消息复读）——不知道自己上次干过同样的事 → 重复，只是换到**自主行动**这条 lane。
> **修**：`buildUserMessage` 里 seed 块——
> - `ExploredCount>0` 时注入"第 N 次探索 + 过往经历摘要（`seedRecentContext`）+ 明令别从头重刷，往更深一层 / 新角度推进，学透就收尾"。
> - 每次探索末尾提示用 `record_learning` 把**新**理解接着写进 digest（含首次），下次才能接上不冷启动。
> **仍待**：mastery 仍按 substantive 给、不检测"这轮是否真比上轮新"——靠 prompt 让探索递进来间接保证，未做硬性新颖度闸（重复刨理论上仍能涨分）。若长跑仍见浅结晶，再考虑 `masteryDelta` 引入"与上次 digest 的差异度"折扣。
> **影响**：`internal/runtime/action/action.go`、`R83`、`R80`、`R75`、`R91`。

### R94 · ghost-skill 收养：清库换生命但 workspace 仍在 → 新生命收养前主技能（已修 Phase 0.5）
> 用户 2026-06-05 数据体检发现：新生命 `local-7cb130f13e6a89e1`（2h）有 13 条 skill_instance，其中 **12 条在 genesis 瞬间（11:10:45）mastery=0 注册**，名字（`hn-digest`/`static-blog-homepage`/`regex-engine-nfa-dfa`/`micro-expression-reading`…）根本不是这生命的兴趣（它只学 Rust 所有权/宏、意识流、贝叶斯）。
> **根因**：上次"清库重开"只删了 `mindverse-data` volume，**没删 `./workspace` bind-mount**。boot 时 `skill.ScanDir`（`loader.go`）扫 `/workspace/skills/*`，对每个含 SKILL.md 的文件夹无条件 `loadFolder`，以**新生命的 life_id** 重新 keying（`id=hash(lifeID:name)`）成自己的 mastery=0 行——前几个测试生命留下的 SKILL.md 被新生命静默收养成幽灵技能，污染技能/自我模型。SKILL.md 文件夹**无任何归属标记**（`authored_from` 仅标自创来源，外部/前主技能为空）。病根同 [[R91]]/[[R93]]——不校验"这东西是不是我的"。
> **修**：技能文件夹加 `.owner` 归属标记（内容=创建该技能的 life_id）。`loadFolder` 是唯一采纳入口（ScanDir 过滤后 / Load 粘贴 / AuthorFromKnowledge 自创均经此），在此统一盖 owner=当前 life_id。`ScanDir` 装载前先读 `.owner`，**不匹配（含无标记）则跳过**，不静默收养。lifepack 导入会带上 `.owner`（=原 life_id=导入后 life_id）故正常认领；裸 volume-wipe 换新随机 life_id 则正确遗弃旧 folder。注：`id=hash(lifeID:name)` 已让跨生命 id 碰撞结构上不可能，故 `.owner` filter 即完整修复，无需加 DB migration。
> **仍待**：Phase 4 技能社群分发时，外部投放的 SKILL.md 无 `.owner` → 当前会被 ScanDir 跳过，需走显式 import/审批流采纳（R18/R80），届时设计。当前生命的 12 条幽灵行由"全清重生"（volume+workspace 同删）一并清除，本修复为前向防护。
> **影响**：`internal/runtime/skill/loader.go`、`internal/runtime/skill/loader_test.go`、`R80`、`R88`、`R82`、Phase 4 `07`。

### R95 · 语义固化链断点：record_learning digest 永不固化（sem_confirmed 恒 0，已修 Phase 0.5）
> 用户 2026-06-05 数据体检发现：`sem_confirmed=0`、reflect 日志全 `promoted:0`，而 PRD §7.2 观察验收项之一正是"SemanticConfirmed 增长可见"——长跑会直接挂掉这一条。
> **根因**：`UpsertSemanticCandidate` 初见死值 confidence=0.5，同内容再现一次 +0.1，`ShallowReflect` 固化阈值 ≥0.75 → 需同一内容被见 **3+ 次**才升语义记忆（这是为 `extractor:v2` 的"重复模式"路径设计的）。但 `record_learning` 写入的 candidate content = **学习 digest**（每次探索都是不同长文）→ 永远走 INSERT 新行、卡在 0.5、`support_count` 永不累加 → 永不固化。= 学透的知识被当"需重复 3 次才采信的暂定模式"，机制错配。docs/10 line 700 早登记"探索→SemanticCandidate→浅审固化链尚未闭合"。
> **修**：新增 `UpsertSemanticCandidateConf(...,initialConf)`，`record_learning` 以来源 seed 的 **mastery** 作初见置信入库——学透的 digest（mastery≥0.75）直接达阈值，经 ShallowReflect 沉淀进 `semantic_confirmed`；浅学的（<0.75）留候选区，待掌握加深后的新 digest 再够格。`extractor:v2` 重复模式路径仍走默认 0.5 不变。固化的"反思才能把经历升为知识"语义保持不变（仍由 ShallowReflect 当闸）。
> **仍待**：同一 seed 多次探索产生多条不同 digest 候选，低置信的旧 digest 会滞留候选区（无害，是渐进笔记）；未做按 seed 去重。长跑验证 `sem_confirmed` 是否随掌握增长。
> **影响**：`internal/storage/memory.go`、`internal/storage/memory_test.go`、`internal/runtime/tools/builtin/builtin.go`、`internal/runtime/reflect/reflect.go`、`R74`、`R66`、`R65`。

### R96 · Phase 0.5 持续观察窗口缩短 1 月→1 周 + 飞书全消息矩阵（已实装+实测 Phase B）
> 用户 2026-06-05 决策两项：
> **① 持续观察窗口缩短**：PRD §7.4 / §1 / §2.5 的"养 ≥1 月"改为 **≥1 周**（飞书双向稳定性仍 ≥3 周不变，是更长的硬闸，故长跑实际由飞书 3 周门控）。理由：加速 Phase 0 退出判定迭代。
> **② 飞书全消息矩阵（Phase B，已实装+实测）**：`lark.go` 原只收 `text`+p2p、只发 `text`。现：
> - **收**：`handleMessage` switch MsgType——post 拍平为文本（标题 + text/a/at，[图片]/[视频]占位，容忍语言包裹/直接两形状）；image/file/audio 经 `Im.V1.MessageResource.Get(msgID,key,type)` 下载落 `<sandbox>/inbox` + 合成文本提示喂 reflex/perception（无视觉/转写，audio 留 Phase 1）。
> - **发**：`SendCard`/`SendApprovalCard` + 导出 `SendPost`/`SendImageKey`。
> - **确认流（真按钮，走长连接）**：skill 缺非白名单依赖且非 auto → `skill.ApprovalNeededEvent`（bus）→ main 解析收件人（lastSender→MostRecentContact）→ 发 3 选项卡片（**批准一次 / 批准类似请求(→开 auto-approve) / 拒绝**）；`dispatcher.OnP2CardActionTrigger` → `lark.handleCardAction` → 注册的 `CardActionFunc`（main）→ `skill.ApproveDeps`/`RejectDeps`（单一真相，不复制安装逻辑）。
> **审计纠正**：先前调研误判"card 回调只能 HTTP webhook"——那是 v1 旧机制。v3.9.4 `dispatcher.OnP2CardActionTrigger` + ws `handleDataFrame`（"for cardCallback" 回写）**走长连可收**，无需公网 URL。实测通过。
> **实测踩坑（均已修）**：① 卡片回调有 **3s 响应硬截止**——批准装依赖（pip，数十秒）必须**异步**，否则飞书提示"回调超时未响应"。② 在回调响应里塞 v1-schema 卡片作 `card_json` data 被拒（**err 200672**）；改用 `Im.V1.Message.Patch(open_message_id, 结果卡片JSON)` **异步更新**原卡片（撤按钮、显示结果），与 3s 截止解耦。`open_message_id` 取自回调 `Context`。
> **解耦**：lark 不识 skill（注册式 `CardActionFunc`）；skill 不识 lark（bus 事件）。沿用 `ReplyEvent` 同款模式。
> **一次性外带**：飞书控制台订阅 `card.action.trigger`（事件与回调）+ 选**长连接**投递模式（代码无法自动配）。用户已配。
> **仍待**：① 接收侧 image/post **实测**仅单测覆盖解析，活体收图/富文本待自然 dogfooding 验证；② 卡片 Patch 用 message Patch（30min 窗口内），超窗不更新；③ 群聊入站仍丢弃（Phase 4）；④ 出站富消息（post/image）已有 helper 但暂无生命体侧工具触发（按需再加）。
> **影响**：`internal/io/lark/lark.go`+`lark_test.go`、`internal/runtime/skill/loader.go`（`ApprovalNeededEvent`）、`cmd/runtime/main.go`（接线）、`docs/PHASE-0-PRD.md`、`R55`、`R87`、Phase 4 `07`（群聊/多渠道）。

### R97 · 语义沉淀引擎权威化：探索→语义记忆不靠 LLM 自觉调 record_learning（已修 Phase 0.5）
> 2026-06-06 长跑观察锁定生命 `local-1b844e59cd694b9e`（跑 ~1 天、28 episode、reflection 8 次）发现：`sem_candidate=0` 且 `sem_confirmed=0`，8 次 ShallowReflect 全 `promoted:0`。interest#1（孤独感，mastery 0.84）已 sediment 退役（strength→0.1）却零候选 = 铁证。
> **根因**：[[R95]] 修了 candidate 的初始置信（record_learning 以 mastery 入库），但**空转**——`record_learning` 是可选工具、deliberative LLM 常不调（[[R93]] 已注），且 `extractor:v2`（重复 tool.success）也几乎不触发。`maybeCrystallize` 的非技能分支（knowledge/topic/experience 学透→退役）注释假设"digest 已经 record_learning 进 semantic 候选"，但实际什么都没进 → mastery 照涨、知识只散落 episode、语义记忆恒空。病同 [[R94]]：把本该引擎权威的事托付给 LLM 自觉。
> **修**：`maybeCrystallize` 退役非技能 seed 前调 `sedimentToSemantic`——digest 优先用 seed 已留的；没有则 `distillSeedKnowledge` 单发 LLM 据近期相关经历（`seedRecentContext`）蒸馏一段"真正理解到的核心知识"；`UpsertSemanticCandidateConf(content, "engine:sediment", mastery)`（置信=mastery≥0.8 ＞0.75 阈值）→ 下一轮 ShallowReflect 即固化进 `semantic_confirmed`。不再依赖 LLM 调工具，沉淀成为引擎保证。
> **仍待**：① 已 sediment 的旧 seed（如 interest#1）知识已丢、不回填，仅前向修复；② record_learning 与 sediment 可能产近重复候选（content 略异），未去重，低频无害；③ distill 每个学透主题一次 LLM 调用（与结晶同量级，可接受）。长跑验 `sem_confirmed` 是否随掌握增长（这才真验 PRD §7.2"语义固化增长"）。
> **影响**：`internal/runtime/action/action.go`+`sediment_test.go`、`internal/io/llm/llm.go`（`Reason`）、`internal/storage/memory.go`（`UpsertSemanticCandidateConf`）、`R95`、`R93`、`R94`、`R74`、`R66`。

### R98 · episode 摘要去噪：内部节拍淹没经历叙事（已修 Phase 0.5）
> 2026-06-06 ultracode 审计 + 数据实证：每条 episode 摘要都是 `auto-segment 21 events: cycle.start×10, idle.daydream×10, episode.sealed×1` —— 纯内部节拍计数直方图，**无任何经历内容**。raw_trail 87.6% 是 `cycle.start`+`idle.daydream` 噪声，`summarize()` 不加区分全计入 → episode 成无意义直方图，PRD §7.2"跨天记忆连续/引用前几天的事"退出标准悬空（生命无可召回的有意义经历）。
> **病同 [[R97]]/[[R94]]**：管道（episodic memory）接了线但产出无价值。
> **修**：`summarize()` 重写——`noiseEvents`（cycle.start/idle.daydream/episode.sealed）只计数不入正文；有内容的事件（reflex.received/speak/proactive_reach、knowledge.sedimented 等，`payloadSnippet` 抽 content/summary/intent/... 字段）列出正文片段（按字符截断防切坏 UTF-8）；纯标记事件按类计数；纯 idle 段 → "休息/发呆（N 个内部节拍）"。引擎侧实现（`ConsiderSealEpisode` 在 runCycle **内联**，加 LLM 会阻塞节拍，故不调 LLM）。
> **仍待**：未做 LLM 叙事（更高质但内联阻塞节拍，须移异步才能加，Phase 1+）；idle.daydream 仍每 tick 写 raw_trail（噪声生成未减，靠 [[R99]] 剪枝控盘 + 本摘要兜底质量）。
> **影响**：`internal/runtime/memory/memory.go`+`summarize_test.go`、PRD §7.2。

### R99 · 长跑磁盘增长治理：定时剪枝已消费的 raw_trail + working_memory（已实装 Phase 0.5）
> 用户 2026-06-06 提出：长跑磁盘占用需预估 + 定时清理过期/无用数据。
> **实测估算**：DB ~424KB/天（含测试污染），raw_trail（675 行/天，~85% 噪声）+ working_memory（306 行/天）主导增长。不治理 ~0.4MB/天 → ~150MB/年（无界）；加剪枝 → 稳定在几 MB。
> **修（引擎侧定时剪枝，不动 episode/语义/反思等长期记忆）**：
> - `raw_trail` 是 episode 封段 + 语义抽取的源，只能删两游标之前的：cutoff = min(`pendingFromID` 封段游标, `last_semantic_extract_raw_id` 语义游标) − `RawTrailKeepBuffer`(500 余量，含 semWindow 滑窗 + 排障)。`memory.PruneConsumedRawTrail` + `storage.PruneRawTrailBefore`。episode.raw_start/end_id 非 FK（仅信息列），删源事件不破坏已封段摘要。
> - `working_memory`（每 tick 工作记忆回放镜像，in-mem 已每 tick 清空）只保留最近 `WorkingMemoryKeep`(500) 条：`storage.PruneWorkingMemoryKeepRecent`。
> - `main.runMaintenance` 由 `maintenanceDue`（24h 间隔 + 首次/重启即跑一次，剪枝幂等廉价）门控。
> **仍待**：① 删行不缩文件（SQLite 空闲页复用 → 文件随插入剪枝平衡后**平台期**，不增不缩；真要缩需 `VACUUM`，重写整库较重，留按需/月度）；② action_log / episode / reflection 不剪（是长期记忆，量小）；③ 保留窗口未做成 config，长跑看实际增速再调；④ embedding BLOB（episode/semantic）体积较大但行数少，暂不单独治理。
> **影响**：`internal/runtime/memory/memory.go`、`internal/storage/memory.go`（`PruneRawTrailBefore`/`PruneWorkingMemoryKeepRecent`）+`prune_test.go`、`cmd/runtime/main.go`（`maintenanceDue`/`runMaintenance`）、`R66`。

### R100 · Phase 0.5 静默管道审计：其余项评估与暂缓（登记不修）
> 2026-06-06 ultracode 三 agent 审计锁定生命，除已修 [[R97]]/[[R98]]/[[R99]] 外的发现，逐条核对后**判定暂不修**（多为设计本意或 agent 误判，记录防遗忘）：
> - **MaxOpenGoals=1 串行探索**（agent 称瓶颈，建议提到 3-4）：**[[R88]] 故意**——让生命体一段时间专注一件事而非不停开新坑。不改。
> - **idle 占 97% 周期 / boredom 高**（agent 称"空转噪声"）：休息/发呆是数字生命合法行为（[[R86]] 能量休息闸）；真问题只是噪声污染 episode，已由 R98 治。不改节拍。
> - **mastery 卡 0.55 永不到 0.8**（agent 断言）：**误判**——实测 interest#1 达 0.84 已 sediment 退役、#2 0.67 在涨，mastery 进展正常。
> - **working_memory 每 tick 清空断跨周期规划**（agent 称 bug）：短期工作记忆本就是单周期作用域（设计本意）；跨周期连续性靠 goal_queue / episode 承载。不改。
> - **extractor:v2 死管道**（≥2 重复 payload 才触发，几乎不触发）：确为低效，但 [[R97]] 已提供主沉淀路径，extractor:v2 作次要路径留着无害；agent 建议降到 ≥1 会让每个 tool 输出都进候选、噪声更大，否决。暂留。
> - **ShallowReflect insight 恒空**（硬编码文案）：低价值修复，候选有得 promote 时再考虑 LLM 合成 insight。暂缓。
> **影响**：登记性条目，无代码改动；`R88`、`R86`、`R97`。

### R105 · 僵尸 active 目标 → 认知主循环永久空转（已修 Phase 0.5）
> 2026-06-06 观察：锁定生命 [[R96]] 连续 ~17h 零行动，只剩感知/反思后台跑，认知主循环（目标→行动→资源）完全停摆。
> **根因（两步操作被崩溃打断）**：`NextPendingGoal` 选中目标即把它翻成 `active`，而落库收尾（`MarkGoal` 标记 done/failed）在 `action.Execute` 的**末尾**才发生。进程在二者之间被重启/崩溃/休眠打断 → 该目标永久卡 `active`：`NextPendingGoal` 只挑 `pending` 永远跳过它，goalgen 又按 payload 对 active 去重不再产同源新目标。队列里没有可执行的 pending、又生不出新目标 → 主循环空转。这是典型的“先翻状态后落库收尾”两步操作缺崩溃恢复。
> **修复（commit b5ee693）**：启动时 `storage.ReclaimActiveGoals(lifeID)` 把上次运行遗留的僵尸 `active` 目标退回 `pending`，下个 cycle 重新可被挑选执行。放在 `state/ledger/scheduler` Init 之后、主循环启动之前。
> **教训（可推广）**：任何“先翻内存/DB 状态、后在远端调用末尾才落最终态”的两步操作，都必须配崩溃恢复——要么单事务原子完成，要么启动时扫描并回收处于中间态的记录。本仓 active→done 这类 lease 式状态机尤其要有“启动回收孤儿 lease”的兜底。
> **影响**：`cmd/runtime/main.go`、`internal/storage/goal.go`（`ReclaimActiveGoals`）、`R75`。

### R106 · 能量经济双轨评估：日额度是休眠计量、真闸是会回血的 vitality（登记不修）
> 2026-06-06 在修 [[R105]] 后观察到“单次 deliberate 耗 energy ≈ 1.91、而 genesis `EnergyDailyCap=1.0`”，疑似一次行动即耗尽日额度 → 长时间无法行动。**逐源核对模型后判定：非失调，不改代码**。
> **模型实情——两个同名但互不相干的“能量”**：
> - **`state.Energy`（vitality，0..1）= 真正的行动闸**：genesis 初值 1.0。`runCycle` 行动门是 `g != nil && frame.Life.Energy >= RestEnergyThreshold(0.20)`。单次慎思在 `action.finalize` 只扣**固定** `Energy -0.02`（非按 token 扣）；回血走 idle.Tick(+0.01)/rest 分支(+0.05)，并由 `scheduler.nextInterval` 在低能量时拉长节拍（<0.3 ×2、<0.1 ×4）。这是个自调节循环：做事 -0.02、歇着 +0.01~0.05，永远能回血，不会卡死。这正解释了为何 goals 6/7/8 能 40min 内连完——每次只掉 0.02，离 0.20 闸很远。
> - **`ledger` 的 energy 余额 + `EnergyDailyCap`/`EnergyUsedToday` = 休眠的账面计量，不门控任何东西**：`ledger.Spend(Energy, TokensToEnergy(usage)≈1.91, …)` 只写 ledger 流水表（累计资源账，可为负），**既不动 `state.Energy` 也不动 `EnergyUsedToday`**。`EnergyUsedToday` 全仓**无任何自增点**（grep 确认：只在 genesis 与日重置时被设 0）；`EnergyDailyCap=1.0` 全仓**从未被当作闸读取**，`MaybeResetEnergyDailyCap` 只是每天把它重置回 1.0。即 cap/used_today 是占位字段，当前不参与任何门控。
> **结论**：“cap=1.0 vs cost=1.91” 是表象冲突而非真失调——日额度是死字段，真闸是会回血的 vitality 循环，从未阻断行动。**故不调 `EnergyDailyCap` / `TokensToEnergy` 率 / 重置节奏中的任何一个**，避免给一个尚不发挥作用的字段做“修复”而引入虚假语义。
> **未来留白（不在本次动）**：若 Phase 1+ 真要让“token 烧得多 → 累得快”，正确做法是把 deliberate 的固定 `-0.02` 改成与 `TokensToEnergy` 挂钩、或让 `ledger.Spend(Energy)` 联动扣 `EnergyUsedToday` 并令行动闸同时看 used_today——届时 cap 与 cost 才需校准（与平台层 [[R103]] token 翻译点/计量点协调一并考虑）。当前 Phase 0 保持简单。
> **影响**：登记性条目，无代码改动；`R86`、`R105`、`R103`。

### R88 · 对话历史 + 行为降频 + 技能生命周期完善（已实装 Phase 0.5）
> 用户 2026-06-05 一批改进：
> **① 对话载入历史**：reflex 原本每条消息只给 `[system, user]`，无往来历史 → 大模型回复有失忆/失意感。新增 `storage.RecentDialogueTurns`（从 raw_trail 的 reflex.received/speak 重建近期对话）+ `reflex.dialogueHistory` 注入最近 10 轮（单轮截 600 字控 token，去重末尾当前消息）。
> **② 反思/目标产生降频**：原每 60s cycle 就可能反思 + 派新目标。`main.behaviorDue`（schema_meta 记时间戳，间隔=15min 基线 + cycleID 抖动到 30min）门控反思与"产生新目标"；**已在队列的 pending 目标不受影响照常执行**——只节流"产生"频率，让生命体一段时间专注做一件事而非不停开新坑。
> **③ 技能渐进式披露（Anthropic skills 规范）**：`ListReady` 早已存在但 deliberative prompt 从没列技能、use_skill 也没提示 → LLM 不知技能存在、从不用。现 prompt 只列技能 **name + 一句话描述**，正文用 `use_skill(name)` 按需读（省 token）；`use_skill` 已写入工具清单。使用计数 `BumpSkillUsed`（UseByName 内，used_count++/last_used/mastery+0.05）本就有。
> **④ 遗忘=归档非删除 + 重激活**：`DecaySkills` 由 `status='disabled' + mastery=0` 改为 `status='archived'` 并**保留残留 mastery + 技能文件夹**（好不容易学会的不丢）。`ReactivateForInterest(content)` 在新兴趣创建时（reflex add_interest / idle propose）保守匹配（ascii≥4 字符 / CJK≥3 连字重叠）相关归档技能并 `ReactivateSkill`（archived→ready）——兴趣再现即"想起自己其实会这个"，不必从零重学。boot/rescan 的 `loadFolder` 保留 archived/disabled 状态，不因文件夹还在就复活。
> **仍待**：reactivation 的关键词匹配较粗（未来可向量相似度）；history 固定 10 轮（可按 token 预算自适应）；降频间隔未做成 config。
> **影响**：`internal/runtime/reflex/{reflex,tools}.go`、`internal/runtime/idle/idle.go`、`cmd/runtime/main.go`、`internal/runtime/action/action.go`、`internal/runtime/skill/loader.go`、`internal/storage/{memory,skill}.go`、`R82`、`R86`。

### R87 · 面板写操作鉴权（防公网暴露被陌生人交互，已实装 Phase 0.5）
> 用户 2026-06-05 提出：用户若不慎把生命体面板暴露到公网，只读内容无妨，但**写/交互**操作（注入对话驱动生命体、改 dangerous-skip-permissions、批准装依赖、装 skill）会被陌生人滥用。
> **方案（共享令牌，方法级中间件）**：
> - env `MINDVERSE_ACCESS_TOKEN`：非空时启用；空（默认）不鉴权，适合本机 dogfooding。
> - `httpapi.withAuth` 中间件：`/api/` 下**变更类方法**（POST/PUT/PATCH/DELETE）需带 `X-Mindverse-Token` 且 `subtle.ConstantTimeCompare` 匹配，否则 401。读（GET/HEAD，含 SSE `/api/stream`）与静态资源永远开放。方法级 → 自动覆盖现有 + 未来所有写端点。
> - `/api/config` 暴露 `auth_required`，前端据此在 ConfigPanel 显示令牌输入框（存 localStorage，仅本机），写请求经 `apiPost` 统一带 header；401 提示令牌无效。
> **隐私读保护（用户 2026-06-05 追加）**：对话含用户原话 = 用户隐私，仅写鉴权不够。`isProtectedRead` 把 `/api/actions?view=dialogue`（及无 view 的全量，含对话）也纳入令牌保护；`view=action`（生命体自主行动，非隐私）仍开放。前端对话面板用 `TokenGate` 整块上锁、未授权不拉取。
> **前端「锁」UX（用户提议）**：`auth.ts`（token/authRequired/locked 响应式 store）+ `TokenGate.svelte`——未授权时把交互区块整体替换成居中的「输入访问令牌」按钮（内联填入即一处解锁、处处解锁）。已套在 InjectForm、SkillPanel 写控件、对话面板。
> **实测**：GET 状态读无 token→200；对话读 `view=dialogue` 无 token→401、对 token→200；行动读 `view=action` 无 token→200；POST 写无/错 token→401、对 token→202。
> **仍待**：① SSE `/api/stream` 未鉴权——其中 `reflex_reply` 实时事件含对话，理论上仍可被未授权连接收到（EventSource 不支持自定义 header，需改 query token 或事件过滤，Phase 1 处理）；② HTTP 明文传令牌（生产配 HTTPS 反代）；③ 单一共享令牌无多用户/权限分级（Phase 4）；④ 无频率限制。
> **影响**：`internal/io/httpapi/httpapi.go`、`web/src/lib/api.ts`、`ConfigPanel.svelte`、`SkillPanel.svelte`、`R55`、`R72`、`R73`。

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

### R101 · LLM 转发零留存的信任面（纪律非架构，付费档唯一保护）
> 来源：`docs/11` 平台层架构评审。平台服务面 over 自托管模型下，服务 ①②④⑤ 的"不读生命内容"由 OS/进程/加密边界保证（**架构盲**），但 **③ LLM Gateway 是唯一内容过境点**（prompt = 对话/episode/reflection 原文），其内容盲只是**纪律 + 审计盲，不是架构盲**。付费档（未开 BYO-key、平台池化 key）下没有任何结构边界阻止 logging bug / panic dump / pprof heap 泄漏 prompt body —— 正是 `06 §9.2` 红线与 `docs/11 §2` 方案B 被排除的那种失败模式。`COMMERCIAL` D01"保留 LLM 中间商"选的恰是这条未逃逸路径，所以 §5.2 零留存契约对付费多数用户是**唯一**保护。
> **待**：
> - §5.2 硬化落地为可执行规约 + 测试：日志字段白名单（禁 request-body 日志，呼应 Fiber 护栏）；禁 body-capturing profiler/tracer；可复现构建 + 透明日志/签名背书（让"运行=开源那份"可验证）。
> - BYO-key / 本地模型设为内容敏感用户**默认推荐**路径（唯一架构性逃逸），不只"高信任用户"可选项。
> - 是否引入 confidential compute（SEV-SNP/TDX）逼近架构盲（M1 in-scope 还是延后）。
> **影响**：`docs/11 §3 §5.1 §5.2 §5.3 §8.3`、`06 §9.2`、`COMMERCIAL D01 CR01`、`R43`、`R102`。

### R102 · 平台元数据侧信道（energy 燃速/在线序列可回连身份，需聚合阈值）
> 来源：`docs/11` 平台层架构评审。平台即便"只看元数据"（DID 公钥/LifeName/owner/订阅状态/energy 燃速/在线时间序列/per-DID token 计量），这些元数据本身仍是指纹侧信道：energy 燃速曲线 + 在线时间序列 + per-DID token 计量时间序列可做行为指纹、回连身份（同 `R19` k-匿名问题、`06 §9.2`"可回连身份"红线）。§5.2 计量必须记 DID 才能扣配额，但**保留/导出**的指标集若不聚合即破红线。`docs/11 §5.1` 只说"必须聚合 + 限速"，无阈值。
> **待**（地基建完、计量 schema 落地前须定，且 schema 不能堵死后续聚合）：
> - 字段划分：哪些 per-DID（计费必需，如 energy-debit/points_charged）vs 哪些必须聚合（燃速曲线、在线序列）。
> - 最小聚合窗口 / k-匿名下限 / 保留 TTL；可观测面禁暴露 per-call 时间戳，只出桶计数。
> - 交叉对齐 `04 §7.2.1` allow/deny 表。
> **影响**：`docs/11 §5.1 §5.2`、`04 §7.2.1`、`06 §9.2`、`R19`、`R08`、`R44`、`R101`。

### R103 · token 翻译点 vs 计量点的 04/06 协调
> 来源：`docs/11` 平台层架构评审。`docs/11` 初稿曾写"`06 §2.5`/`04 §3.2.1` 的 token→energy 唯一翻译点现在落在平台"，与宪法冲突：`04 §3.2`/`04 §2.1` 锁 `LLMAdapter`（用户机器上的 Runtime 子模块）为**唯一** token→energy 翻译点，`06 §2.1` 列"运维/平台审计"为 token **永不可见**方。
> **当前 `docs/11` 裁决（待 04/06 正式承认）**：**两个计量器、两个边界**——平台计 provider token → 扣账户「精力点」配额（**外部经济**计量点）；用户侧 `LLMAdapter` 独立保留 token→energy（**内部经济**翻译点，宪法锁定不动）。**不是把唯一翻译点搬到平台**，是新增一个外部计量点。Agent/UI 永不见 token。
> **待**（需升级宪法）：
> - 升级 `06`（§2.1/§2.5）正式承认平台侧 token 外部计量点（与 LLMAdapter 内部翻译点并存、互不替代）。
> - 升级 `04 §3.2` 承认平台计量器存在（不改 LLMAdapter 唯一内部翻译点的锁）。
> - 精力点 ↔ token 两张映射（point→token 售卖配额、token→energy 在世消耗）的归属与公式形状（后者 = `R25`/`R45`，Phase 1 标定）。
> **影响**：`06 §2.1 §2.5`、`04 §2.1 §3.2 §3.2.1`、`docs/11 §4③ §7.2 §9 §10`、`COMMERCIAL §3.2 §3.3`、`R25`、`R45`。

### R104 · Admin/Free 档可选性与内容盲边界
> 来源：`docs/11` 平台层架构评审。两条边界须明确：
> **① Admin Console 元数据-only 红线**：管理平台（与平台同 repo、多 cmd、独立内网部署）给运营控制台 = 最可能想偷看生命内心的入口。架构上必须做不到：平台本不持生命数据（自托管），admin 只见注册元数据 + 计费 + 聚合遥测；LLM Gateway 零留存 → admin 也永不见 prompt body。admin 需 RBAC 强认证 + 全程操作审计。卡密签发/核销/手工入账全程审计日志。
> **② Free 档身份可选性**：`docs/11 §1` 说 Free 纯本地存续，但 §4② 身份登记"Phase 1 起"。须明确：身份登记对 Free **可选**——纯本地生命可无 LifeName/uid（匹配 Phase 0 行为）；**注册 = 毕业到网络可见**。这关上 §1-vs-§4② 的张力。
> **待**：
> - Admin RBAC 角色矩阵 / 审计日志字段 / 聚合遥测的 k-匿名下限（与 R102 同源）。
> - Free 不注册时的本地标识（无 uid 时 RawTrail 用什么标识，同 `R60`）。
> - 货币命名「星屑」占位待用户确认（`docs/11 §4.6`）。
> **影响**：`docs/11 §1 §3 §4.1 §4.7 §5.1`、`06 §5.1.2 §9.2`、`R60`、`R102`、`R47`。

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
