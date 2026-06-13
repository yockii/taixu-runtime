# 统一上下文装配（ContextAssembler）— 消灭经历割裂

状态：草案 + 慎思层首实现（2026-06-12）。用户要求：所有行为归一处，每次 LLM 调用注入最近经历（但不进 system），并能压缩/管理上下文 + 主动回忆。

## 问题

生命体的经历割裂：游戏、社交、知识、对战各自为政。每次 LLM 调用只看到「当前任务」+ 人格 system，**不被动看到最近发生了什么**。最近经历仅靠 LLM 主动调 `recall_recent` 工具才进上下文——它常忘了调，于是「游戏因人不够取消了」这种经历不会自然流进随后的社交决策。结果：行为之间没有连续性，像每次重新出生。

## 目标

一个 **ContextAssembler** 收口所有 LLM 调用的上下文装配，每次调用统一组装：

```
system   = 人格自述（IdentityPreamble + PersonaPrompt）   ← 稳定、缓存友好、不随经历变
assistant= 【最近经历块】                                  ← 被动注入(自动)，跨域、recency+salience、token 预算
user     = 本轮任务（goal / 对话）                         ← 当轮具体输入
tools    = ... + recall_recent / query_memory             ← 主动深挖通道
```

双通道记忆：**被动**（最近经历自动进 assistant 历史消息，保连贯）+ **主动**（recall 工具按需深挖相关旧事）。经历块**绝不进 system**——system 必须稳定以利 KV 缓存，且经历是「上下文」非「身份」。

## 脑 = 已有四层记忆（不新建存储）

working / episodic（带 salience+emotion）/ semantic / reflection digest 已存在。ContextAssembler **只做收口装配**，不新建存储层。最近经历块取自 `storage.ListEpisodes`（封段后的跨域经历，含 salience）。

## 装配规则（最近经历块）

- 取最近 N=15 段 episode（`ListEpisodes` recency 序）。
- 排序：在「最近」窗口内按 **salience 降序** 选 top-K（显著经历优先浮现），再按 **时间正序** 排列（叙事连贯，读起来是时间线）。
- token 预算：按 lane 分层裁剪（见下）。每段摘要截断到 ~200 字。
- 滚动压缩：episode 本身已是封段摘要（DistillEpisode），天然压缩；进一步压缩复用 reflection digest（后续）。

## 分层预算（lane）

| lane | system | 经历块 | 说明 |
|---|---|---|---|
| 慎思 deliberative | 全量人格 | 全量（~1500 字, top-6） | 自主作为，需最强连贯 |
| reflex 反射 | 精简 | 精简（~600 字, top-3，只态+最紧义务） | 对话快路径，省 token |
| idle | — | 兴趣偏重（后续） | 不调 LLM 慎思，偏重兴趣种子 |

## 实现进度

- ✅ 慎思层（action.go Execute）：`contextasm.RecentExperience(lifeID, maxChars)` 生成经历块，作 **assistant 历史消息**插在 system 与 user task 之间。empty 则不插。
- ⏭ reflex 层接装配器（精简预算）。
- ⏭ idle 兴趣偏重。
- ⏭ reflection digest 滚动压缩接入经历块。
- ⏭ 验证经历连贯：游戏取消/输赢能被随后社交目标自然提及。

## 不做

- 不新建记忆存储层（收口装配，非重写记忆）。
- 不把经历塞 system（破缓存 + 语义错位）。
- 不删 recall 工具（主动通道保留，与被动互补）。
