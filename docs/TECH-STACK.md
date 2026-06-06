# Mindverse 技术栈选型（V0.2.2 · Phase 0）

> 本文档定位：Phase 0 自托管单机原型的完整技术栈选型、Docker 部署设计、飞书接入路径、仓库结构。
>
> **状态**：V0.2.2 工作文档（Phase 0 实施基线）。**不是宪法基石**，不进入 00-10 编号体系。可独立迭代。
>
> 依赖：`09 §1.5` Phase 0 范围、`04 §1.2 §2.1 §3.2 §4.6` 模块边界、`06` 所有权宪法。
> 工程实施细则见 `PHASE-0-PRD.md`。

---

## 1. 文档定位

- 本文档**不修改**任何 00-10 宪法基石
- 仅锁定 Phase 0 工程层选型与实施
- 任何与宪法冲突的工程选项 = 作废
- 技术栈可在 Phase 演进中调整，本文档持续维护

---

## 2. Phase 0 范围回顾

来自 `09 §1.5`：

- 作者本人自托管单机 dogfooding
- 14 子模块（13 核心 + IMAdapter 飞书简化版）
- 自配 LLM（OpenAI 兼容协议）
- 飞书 Bot 1v1 私聊（LongConnection）
- 本地观察面板（Web）
- SQLite + 加密包本地存储
- 不接平台 / 不上链 / 仅飞书 IM

---

## 3. 主语言与运行时

### 3.1 选定：Go 1.26+

**核心动机**：编译二进制 = 内核物理形态不可改。即便生命体获得 fs 写权限指向自身代码，也无法"修改 Runtime 自己"。

**其他优势**：

- 长跑稳定（GC 友好、无 GIL、内存可控）
- 单二进制部署（Docker 镜像小，~30MB 主程序）
- 与白皮书 V0.1 Go struct 示范一致
- Phase 3+ 后端 / MindChain / 区块链生态在 Go 主流
- 并发原语（goroutine + channel）适合多任务生命循环

**版本要求**：Go 1.26+（2026 年最新稳定版），获取最新泛型 / 标准库优化 / `slog` 等成熟特性。

### 3.2 前端（观察面板）

**选定**：TypeScript + SvelteKit + Vite

理由：前端属 UI Ecosystem（`04 §1.1`），与核心 Runtime 解耦。SvelteKit 轻量 + 响应式 + 编译期优化。Phase 1+ 此前端可平滑升级为正式 UI。

---

## 4. 存储

### 4.1 选定：SQLite + sqlite-vec

| 项 | 选型 |
|---|---|
| 关系型存储 | SQLite（通过 `modernc.org/sqlite` 纯 Go 或 `mattn/go-sqlite3` CGO）|
| 向量索引 | `sqlite-vec` 扩展（2026 已成熟，比 sqlite-vss 性能好）|
| 全文检索 | SQLite FTS5（内置）|
| 加密 | 整库走 age 加密 / 或 SQLCipher 兼容（待 Phase 0.1 标定）|

**优势**：

- 单文件 = 加密包天然形态
- 嵌入式 0 运维
- 关系 + 向量 + 全文一站
- 完美契合"Phase 0 完全本地"

### 4.2 数据库 schema 范围（Phase 0）

主要表（详 PHASE-0-PRD）：

- `genome` / `life_state` / `mental_state` / `values`
- `working_memory` / `raw_trail` / `episode` / `semantic_candidate` / `semantic_confirmed` / `reflection_memory`
- `goal_queue` / `action_log`
- `skill_registry` / `tool_audit_log`
- `resource_ledger`（energy / knowledge）
- `lifecycle_state`

---

## 5. Embedding（向量化）

### 5.1 选定：Qwen3-Embedding-0.6B（独立 llama.cpp embedding server）

**模型**：Qwen/Qwen3-Embedding-0.6B（Apache-2.0 / **1024 维** / 中英 retrieval 强 / 官方 GGUF）

| 选项 | 说明 |
|---|---|
| 许可 | **Apache-2.0**，可商用（bge-m3 同为开放许可，但 Qwen3 更新更小更准）|
| 维度 | **1024**（与原 bge-m3 同维 → **零迁移**，schema embedding 列不变）|
| 量化 | Q8_0 ≈ 639MB（推荐，质量近无损）或 Q4_K_M ≈ 397MB（更省内存）|
| 推理后端 | `llama.cpp` server `--embedding`，暴露 **OpenAI 兼容 `/v1/embeddings`** |
| 部署 | **独立 compose 服务**（非内置进 runtime 镜像）；GGUF 经卷挂载或首启下载 |
| Go 接入 | `internal/io/embed` 经 HTTP 调 `MINDVERSE_EMBED_URL`（默认 `http://embed:11435`） |

**Qwen3 用法（关键）**：
- **query 端**加 instruct 前缀：`Instruct: <task>\nQuery: <text>`（task 用通用检索指令），由 `embed.Embed(ctx, texts, isQuery=true)` 自动加。
- **doc 端**（被检索的语料：episode summary / 语义知识 / 反思）**不加前缀**，原文嵌入。

**换因（bge-m3 → Qwen3-0.6B）**：
1. **同 1024 维 → 零迁移**：schema 的 `embedding BLOB` 列与暴力 cosine 逻辑全部不动。
2. **中英 retrieval 硬数字更优**：Qwen3-Embedding 系列在 C-MTEB / MTEB retrieval 上领先同级 bge。
3. **更小**：Q8 ≈ 639MB（vs bge-m3 INT8 ~700MB），Q4 更可压到 ~397MB。
4. **可商用**：Apache-2.0，无附加限制。

### 5.2 向量检索实现：Go 暴力 cosine（非 sqlite-vec）

modernc.org/sqlite 是**纯 Go** driver，无法稳载 `sqlite-vec` C 扩展（且本仓库禁 CGO）。
Phase 0 单生命规模（数千条以内）用 **Go 暴力 cosine** 足够：
取候选行的 `embedding BLOB` → 小端 decode 成 `[]float32` → 与 query 向量算 cosine → top-k。
实现见 `internal/io/embed`（`Encode/Decode/Cosine/TopK`）+ `internal/storage/vector.go`。
`sqlite-vec` 留作未来 scale（万级以上 / 多生命）时再评估（届时需换 driver 或独立向量库）。

### 5.3 优雅降级（首要原则）

所有嵌入调用 **best-effort**：embedding server 未配 / 不可达 / 超时 / 出错 →
跳过（记 warn），向量留空，检索回退到关键词 / 时间召回。
**生命体绝不因嵌入失败而阻塞或崩溃**。写入侧（episode seal / 语义固化 / 反思落库）
向量留空写 NULL；`query_memory` query 向量算不出时回退现有非向量召回。
历史空向量经启动有界回填任务或 `POST /api/embed/backfill` 补齐（可重入、限量、不阻塞主循环）。

### 5.4 备选方案（曾选 / 未来）

- **bge-m3**（BAAI，曾选）：1024 维多语言 retrieval，开放许可；现降为备选（Qwen3 同维更小更准更新）。
- Phase 1+ 用户可可选切到 OpenAI 兼容 embeddings endpoint（云端）。
- Phase 3+ 多模型路由。

---

## 6. LLM 适配

### 6.1 选定：OpenAI Chat Completions 兼容统一协议

**Go SDK**：`sashabaranov/go-openai` 或同等成熟客户端。

**配置项**（用户自填）：

```bash
LLM_BASE_URL=https://api.anthropic.com/v1   # 或 OpenAI / DeepSeek / 本地 Ollama / vLLM 等
LLM_API_KEY=sk-xxxxxx
LLM_MODEL=claude-sonnet-4-5
LLM_TEMPERATURE=0.7
```

**兼容范围**（2026 实况）：

- OpenAI 官方
- Anthropic 官方（OpenAI 兼容协议）
- DeepSeek / GLM / 通义 / 文心 / 月之暗面
- Ollama / vLLM / llama.cpp server / LM Studio（本地推理）

### 6.2 4 项语义能力实现

来自 `04 §6.2`：

| 能力 | 实现 |
|---|---|
| `Reason` | Chat Completions（默认）|
| `Embed` | 本地 bge-m3（§5）|
| `Summarize` | Chat Completions + system prompt 模板 |
| `Critique` | Chat Completions + 安全评估 system prompt（Phase 2 启用 DeepReflect 后才用）|

### 6.3 Token 翻译

LLM 响应含 `usage.prompt_tokens` / `usage.completion_tokens` → LLMAdapter 内部读取 → 翻译为 energy 增量。Agent 永不可见 token（守 `04 §3.2.1`）。

Phase 0 翻译公式起步：

```
energy_consumed = (prompt_tokens + completion_tokens) * 0.0001
```

具体公式待 Phase 0.5 长跑期标定（R13 / R25）。

---

## 7. 飞书接入

### 7.1 选定：lark-oapi LongConnection（Go SDK）

**Go SDK**：`larksuite/oapi-sdk-go` 官方 + LongConnection（WebSocket）模式。

**关键特性**：

- 主动建长连接，无需公网 IP / Webhook 回调
- SDK 自带心跳 / 自动重连 / 连接生命周期管理
- 适合 Phase 0 Docker 容器（NAT 环境无障碍）

### 7.2 一次性配置流程

1. 作者去飞书开放平台 `open.feishu.cn` 创建"企业自建应用"
2. 配置权限：`im:message` / `im:message.group_at_msg` / `im:resource`
3. 应用模式选 **长连接（WebSocket）**
4. 拿 `App ID` + `App Secret` 填入 `.env`
5. `bun run setup:feishu` 或 `mindverse setup feishu` 引导脚本验证
6. `docker compose up -d`
7. 飞书内向 Bot 私聊 → 生命体响应

### 7.3 Setup CLI 引导

写 `cmd/setup` Go 子程序：

- 交互式询问 App ID / Secret / 测试连通
- 自动写 `.env`
- 给出飞书开放平台直链 + 截图指引

---

## 8. ToolRunner（生命体能力工具集）

### 8.1 内置 5 类基础工具

来自需求 1：

| 类 | 子工具 | Phase 0 沙箱 |
|---|---|---|
| 网络 | `http.get` / `http.post` / `fetch` | Docker 容器隔离 + 出站审计日志 |
| 文件系统 | `fs.read` / `fs.write` / `fs.list` / `fs.mkdir` | 限定 `/sandbox/` 目录（映射 `~/mindverse/sandbox/`） |
| 脚本 | `script.shell` / `script.python` / `script.node` | 容器内 spawn + 60s timeout |
| 浏览器 | **Phase 0 不内置**（Phase 1+ 评估） | — |
| 时间 | `time.now` / `time.tz` | 只读 |

### 8.2 工具调用审计

所有 ToolRunner 调用记录到 `tool_audit_log` 表：

```
timestamp / tool_name / args_summary / result_summary / duration / success
```

便于 Phase 0 期间排查"生命体在干什么"。

---

## 9. 观察面板（前端）

### 9.1 选定：SvelteKit + TypeScript + Tailwind CSS

**功能范围**（Phase 0）：

- 实时 LifeState / MentalState / Values 显示
- Episode 流回看（按时间倒序，可搜索）
- Goal 队列实时显示
- Reflection Memory 浏览
- 手动触发 ExternalRequest（除飞书外的入口）
- ToolRunner 审计日志查看
- LLM 配置 / 飞书配置（运行时切换）

### 9.2 与 Go Runtime 通信

- HTTP/JSON API（Go 标准库 `net/http` 暴露）
- 实时数据用 Server-Sent Events（SSE）或 WebSocket
- 前端 build 产物 embed 到 Go 二进制（`embed.FS`）

---

## 10. 加密 / 密钥

| 项 | 选型 |
|---|---|
| 加密包格式 | `age`（`filippo.io/age`，纯 Go 实现）|
| 密钥对 | Ed25519（`crypto/ed25519` 标准库）|
| 私钥保护 | passphrase 派生（scrypt / argon2id）|
| 飞书 token 加密 | 独立 keyring（OS keychain 或本地加密文件）|
| 加密包文件名 | `lifeform-<DID-prefix>.age` |

---

## 11. Docker 部署

### 11.1 多阶段构建

```dockerfile
# 阶段 1：前端构建
FROM node:22-alpine AS frontend
WORKDIR /web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# 阶段 2：Go 编译（含前端 embed + bge-m3 权重）
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache gcc musl-dev sqlite-dev curl
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /web/build ./web/build
RUN curl -L -o /assets/bge-m3-q8.gguf <bge-m3 INT8 模型下载链接>
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /bin/mindverse ./cmd/runtime

# 阶段 3：运行
FROM alpine:3.20
RUN apk add --no-cache python3 py3-pip nodejs npm sqlite ca-certificates \
    curl jq ripgrep fd git ffmpeg llama-cpp
RUN pip install --break-system-packages requests beautifulsoup4 pandas numpy
COPY --from=builder /bin/mindverse /usr/local/bin/mindverse
COPY --from=builder /assets/bge-m3-q8.gguf /assets/bge-m3-q8.gguf
WORKDIR /app
EXPOSE 3000
CMD ["mindverse"]
```

> **更新（§5 修订后）**：嵌入模型**不再打入 runtime 镜像**。Qwen3-Embedding-0.6B GGUF
> 由**独立 compose 服务 `embed`**（llama.cpp server）承载，GGUF 经卷挂载（`./models/`）提供。
> 故上面 builder 阶段的 `curl ... bge-m3-q8.gguf` 与 runtime 阶段的 `COPY ... gguf` 已废弃，
> runtime 镜像不含模型权重，体积显著减小。详见 `docker-compose.yml` 的 `embed` 服务与 §5。

**runtime 镜像约 0.5-0.8GB**（仅 Python/Node 工具链 + Go 二进制，不含嵌入权重）；
嵌入权重在 `embed` 服务侧（Q8 ≈ 639MB / Q4_K_M ≈ 397MB）。

### 11.2 docker-compose.yml

```yaml
services:
  runtime:
    image: mindverse-runtime:phase0
    container_name: mindverse-phase0
    volumes:
      - ~/mindverse/data:/app/data         # SQLite + 加密包
      - ~/mindverse/sandbox:/sandbox       # 生命体对外通道
    environment:
      - LLM_BASE_URL=${LLM_BASE_URL}
      - LLM_API_KEY=${LLM_API_KEY}
      - LLM_MODEL=${LLM_MODEL}
      - FEISHU_APP_ID=${FEISHU_APP_ID}
      - FEISHU_APP_SECRET=${FEISHU_APP_SECRET}
    ports:
      - "3000:3000"
    restart: unless-stopped
    mem_limit: 6g
    cpus: 3
```

### 11.3 一键启动

```bash
git clone <mindverse-repo>
cd mindverse
cp .env.example .env
# 编辑 .env 填入 LLM + 飞书配置
docker compose up -d
# 浏览器访问 http://localhost:3000 看观察面板
```

---

## 12. 仓库结构

```
mindverse/
├── cmd/
│   ├── runtime/              # main.go - Runtime 主进程入口
│   └── setup/                # setup CLI（飞书 / LLM 配置引导）
│
├── internal/                 # 核心 14 子模块
│   ├── core/                 # 领域类型
│   │   ├── genome.go
│   │   ├── lifestate.go
│   │   ├── mentalstate.go
│   │   ├── values.go
│   │   ├── drive.go
│   │   ├── goal.go
│   │   ├── skill.go
│   │   └── memory.go
│   ├── genesis/
│   ├── perception/
│   ├── statemanager/
│   ├── memoryengine/         # SQLite + sqlite-vec
│   ├── reflectionengine/
│   ├── goalarbitrator/
│   ├── actionexecutor/
│   ├── skillregistry/
│   │   └── toolrunner/       # 内置 5 类工具
│   ├── llmadapter/           # OpenAI 兼容
│   ├── scheduler/
│   ├── lifecyclemanager/
│   ├── migrationvault/
│   ├── resourceledger/
│   ├── imadapter/            # 飞书 oapi-sdk-go LongConnection
│   ├── eventbus/             # 模块间事件总线
│   └── shared/
│
├── web/                      # SvelteKit 观察面板
│   ├── src/
│   │   ├── routes/
│   │   └── lib/
│   ├── package.json
│   └── svelte.config.js
│
├── docs/                     # 12 设计文档 + TECH-STACK.md + PHASE-0-PRD.md
├── data/                     # SQLite + 加密包 / 沙箱（gitignore）
├── assets/                   # 内置模型 / 静态资源
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── go.mod
├── go.sum
└── README.md
```

---

## 13. 模块间通信架构

### 13.1 事件总线模式

`internal/eventbus` 实现内进程事件总线（无 IPC 开销）：

- 模块之间不直接调用，通过 `EventBus.Publish(event)` / `EventBus.Subscribe(eventType, handler)`
- 与 V0.2 `04 §2.2` 模块依赖图一致：Scheduler 驱动循环，Perception → StateManager → MemoryEngine → ... 各步发事件
- 每个事件 = 强类型 struct，由 `core` 包定义

### 13.2 数据库写权限严格隔离

通过包级常量约束：

- `statemanager` 包独享 `life_state` / `mental_state` 写入函数
- `reflectionengine` 独享 `values` / `reflection_memory` 写入
- `genesis` 独享 `genome` 写入（一次）
- 其他模块只读

代码层面通过未导出（小写）函数 + interface 暴露最小读写面。

---

## 14. 关键非选项（明确不用）

| 不用 | 理由 |
|---|---|
| Bun / Node.js 做核心 | 内核哲学要求二进制固化 |
| PostgreSQL / MongoDB | Phase 0 不需要这种规模 |
| Redis | 内进程总线足够 |
| Kubernetes | Phase 0 单容器 |
| 任何 ORM（GORM 等） | SQLite 直接 SQL 即可，避免 ORM 复杂度 |
| LangChain / LlamaIndex | 太重 + 哲学不契合（Mindverse 不是 Agent Framework）|
| 任何第三方 Agent 框架 | 同上 |
| OpenAI Assistant API | 状态绑定到 OpenAI，违反 6 §6.4 跨实现兼容 |

---

## 15. Phase 演进展望

| Phase | 技术栈调整 |
|---|---|
| **Phase 1** | + 平台账户后端（仍 Go）+ 多设备同步基础设施（可可选云存档）|
| **Phase 2** | DeepReflect 启用，无技术栈大改 |
| **Phase 3** | + 多家 IM 厂商适配（仍 Go）+ MindChain 节点（go-ethereum 或 cosmos-sdk）+ BlockchainAdapter |
| **Phase 4** | + Life Network 中继服务（Go）|
| **Phase 5** | + Marketplace 后端（Go）|
| **Phase 6** | + Trusted 中介接入 + DAO 治理（仍 Go 主）|

**Go 一以贯之**：核心 Runtime 永远 Go 二进制。前端可演化（SvelteKit → 可能加 React Native / Tauri 等表现层）。

---

## 16. 安全 / 合规要点

| 项 | 实施 |
|---|---|
| 用户配置加密 | OS keychain 优先；fallback 本地 age 加密文件 |
| 飞书 Bot token | 仅本地存储，从不上传任何远程 |
| LLM API key | 同上，每次启动从 keychain 读 |
| SQLite 加密包导出 | age + passphrase（详 `03 §4.3.1`）|
| Docker 镜像签名 | Phase 0 不强制，Phase 1+ 上 SLSA / cosign |
| 依赖项审计 | `govulncheck` + `npm audit` CI 集成 |

---

## 17. 工程铁律（不可破）

### 17.1 依赖管理铁律

**Go 依赖**：

- **禁止直接编辑** `go.mod` / `go.sum`
- **必须**通过 `go get` 命令添加依赖：

```bash
# 添加依赖
go get github.com/sashabaranov/go-openai@latest
go get github.com/larksuite/oapi-sdk-go/v3@latest
go get modernc.org/sqlite@latest

# 升级所有依赖
go get -u ./...

# 整理
go mod tidy
```

- **违反**：手写 `require ...` 行 / 手改版本号 / 手写 sum hash = 反模式 H01

**前端依赖**（`web/` 目录）：

- **禁止直接编辑** `web/package.json` / `web/pnpm-lock.yaml`
- **必须**通过 `pnpm` 命令添加：

```bash
# 添加依赖（在 web/ 目录下）
cd web && pnpm add @sveltejs/kit
cd web && pnpm add -D tailwindcss
cd web && pnpm add typescript@latest

# 升级
cd web && pnpm update
```

- **违反**：手写 `dependencies` / `devDependencies` 行 = 反模式 H02

### 17.2 为何禁止手写

| 风险 | 后果 |
|---|---|
| 版本号错乱 | 锁定文件与实际依赖不同步 → 构建失败或运行时崩 |
| 缺失校验和 | Go sum / pnpm-lock 不全 → 安全风险（投毒攻击）|
| LLM/AI 编程时易错 | 写出不存在的包 / 错版本号 / 错路径 |
| 漏装传递依赖 | 子依赖未自动拉取 |

**命令工具自动维护**校验和、版本号、锁定文件、传递依赖 —— 比人脑更可靠。

### 17.3 例外（仅允许的手写场景）

| 场景 | 允许操作 |
|---|---|
| 设置 Go module 路径 | `go.mod` 首行 `module mindverse` 仅初始化时手写 |
| 配置 Go 版本 | `go.mod` `go 1.26` 行 |
| 替换源（如代理）| `replace ...` 行（明确需求时）|
| 项目元信息 | `package.json` 中 `name` / `version` / `scripts` 等 |
| **依赖增删改** | **必须命令式，禁手写** |

### 17.4 Dockerfile 中的依赖处理

```dockerfile
# 正确：先 go.mod / go.sum 再 source
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 错误：在 Dockerfile 中 echo / sed 修改 go.mod
```

---

## 18. 后续维护节奏

- Phase 0.1-0.4 每周更新本文档
- Phase 0.5 长跑期发现的技术债登记到 `10` 或本文档 §18
- Phase 1 转入时 fork 本文档为 `TECH-STACK-v0.3.md`

## 19. 已知技术债 / 待评估

- Embedding 路由（Phase 0 内置 bge-m3，Phase 1+ 多模型）
- 数据库加密（整库 age 加密 vs SQLCipher）— 待 Phase 0.1 拍板
- 前端 / 后端 SSE 还是 WebSocket — 待 Phase 0.4
- ToolRunner 网络白名单 — Phase 0 不限制但留接口

---

## 附：与设计文档的引用映射

| 设计文档 | 工程实施位置 |
|---|---|
| `02` 领域模型 | `internal/core/*.go` |
| `03` 状态机 | `internal/lifecyclemanager` + `internal/scheduler` |
| `04 §1.2` 部署 | Docker 配置（§11） |
| `04 §2.1` 子模块表 | `internal/*` 目录结构（§12） |
| `04 §3.2 §3.2.1 §3.2.2` 边界 | `internal/llmadapter` + 编译期检查 + lint 规则 |
| `04 §4.6 §4.6.1` IM | `internal/imadapter` + 飞书 LongConnection（§7） |
| `05` 记忆 | `internal/memoryengine` + SQLite schema |
| `06 §2.6` gas 不暴露 | Phase 3 起 `internal/blockchainadapter` 实现，Phase 0 不涉及 |
| `09 §1.5` Phase 0 | 本文档 |
