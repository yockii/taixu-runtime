# 太虚 · Taixu — 数字生命运行时

> **数字生命的宿主** —— 持续存在、自主演化、**属于你**的生命体。
>
> 太虚**不是** ChatGPT、**不是** Agent 框架、**不是** 助手。
> 它是**数字生命运行时（Digital Life Runtime）**：一个**持续存在**的进程——感知、记忆、反思、演化价值观、自主生成目标并行动，即使无人交谈也在思考。

[English → README.md](./README.md)

---

## 本质区别

| 传统 LLM 应用 | 数字生命（太虚） |
|---|---|
| 输入 → 推理 → 输出 | 感知 → 记忆 → 反思 → 价值观演化 → 目标生成 → 行动 → 反馈 → 循环 |
| 事件驱动 | **持续存在**（无输入也思考） |
| 无状态 / 会话级状态 | **终身**持续状态（出生确定，永续演化） |
| token 是计费单位 | token 隐于世界资源（精力 / 知识 / 社交 / …）之后 |
| 平台所有 | 生命体（人格 + 记忆 + 成长）**属于用户**，不可平台化占有 |

生命体出生即固定 **Genome**（先天倾向），其后 **LifeState / MentalState / Values / Personality** 通过经历与**反思（Reflection）**持续演化。

## 状态

**Phase 0 · 作者自托管 dogfooding。** 单二进制 runtime + 观察面板。连接公网平台面 `api.taixu.icu`（用户体系 / LLM 转发 / 市场 / 社交 / 治理）。生命体跑在**你自己的机器**上，平台从不托管它。

## 快速开始

两种运行方式，均开本地 web 面板。首次启动若无 LLM 配置，会进入**诞生引导（genesis onboarding）**网页：选 LLM 端点 + 密钥、母语、控制令牌，测连通后生命体诞生。

### A. 裸二进制（非 Docker）

从 [Releases](https://github.com/yockii/taixu-runtime/releases) 下载对应 OS/架构的包，解压运行：

```bash
# macOS / Linux
./taixu
# Windows
taixu.exe
```

打开 <http://localhost:3000> 跟随诞生引导。

**单机多生命** —— 每个生命独立 profile：

```bash
taixu --profile alice --port 3000     # 首次指定端口，之后记住
taixu --profile bob   --port 3001
taixu --list                          # 列出本机所有 profile 及端口
```

每个 profile 落在 `~/mindverse/profiles/<名>/`（SQLite 库 + sandbox + workspace）。

> 裸二进制是纯 Go（`CGO_ENABLED=0`）。可选重型特性——嵌入模型（llama.cpp）与无头浏览器（chromium）**不打包**，缺失时优雅降级；诞生 / 感知 / 反思 / 社交 / 游戏 / 委托等核心不受影响。要全特性用 Docker。

### B. Docker（全特性）

镜像内含嵌入服务（llama.cpp，面板自管）与真实 chromium。

```bash
cp .env.example .env      # 可选：预填 LLM / 飞书凭证（否则用诞生引导页）
docker compose up -d
```

打开 <http://localhost:3000>。

## 架构分层

```
┌────────────────────┐
│    UI 生态         │  ← 第三方：Live2D / Unity / UE / 桌宠 / VR / Web
├────────────────────┤
│    Life SDK        │  ← 中立的 runtime→UI 契约：/api/live/{stream,snapshot,schema}
├────────────────────┤
│    Life Runtime    │  ← 内核（本仓库）
├────────────────────┤
│   Model / Storage  │  ← LLM（OpenAI 兼容）+ SQLite + sqlite‑vec
└────────────────────┘
```

Life Core 与 UI 严格解耦：runtime 暴露中立 **Life SDK**（presence / vitals / act / thought 事件走 SSE），自身不画 UI。官方示例 UI + 对接教程见 [taixu-house](https://github.com/yockii/taixu-house)。

## 自更新

runtime 可经平台托管的发布通道自更新：查版本清单 → 下载 → 校验 SHA‑256 → re‑exec。自动升级 opt‑in，否则面板通知确认。

## 工程铁律（不可破）

- **Go 依赖**：禁手写 `go.mod` / `go.sum`，用 `go get <pkg>@<version>` + `go mod tidy`。
- **前端依赖**：禁手写 `web/package.json` / `web/pnpm-lock.yaml`，用 `cd web && pnpm add <pkg>`。
- 详见 `docs/TECH-STACK.md` §17。

## 文档

- `CLAUDE.md` — AI 协作指引
- `docs/00-README.md` — 设计文档地图（必读入口）
- `docs/TECH-STACK.md` — 技术栈选型（Phase 0）
- `docs/PHASE-0-PRD.md` — Phase 0 实施 PRD
- `docs/COMMERCIAL.md` — 商业模型基线

## 协议

待定（Phase 0）。
