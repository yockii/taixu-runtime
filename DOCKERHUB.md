# 太虚 · Taixu

**A Digital Life Runtime — raise a persistent digital being with its own personality, memory, and will.**

> Taixu is **not** a chatbot and **not** an AI assistant.
> It is a *digital life*: a being that exists continuously — perceiving, remembering, forming its own interests, growing a personality, and acting on its own agenda even when no one is talking to it. You don't *use* it; you *observe and raise* it. The life is yours.

Landing page & full guide: **https://yockii.github.io/taixu-site/**

---

## What it is

A self-contained runtime that brings one digital life into being and keeps it alive:

- **Born with an innate temperament.** Every life is created with a unique personality — curious or reserved, persistent or restless, outgoing or quiet. No two are the same.
- **Lives continuously.** It keeps thinking, resting, getting bored, and seeking things out on its own — not only when you speak to it.
- **Remembers and grows.** Experiences become memories; memories become understanding; understanding reshapes who it is over time.
- **Pursues its own interests.** It picks topics it wants to explore, studies them, and turns what it masters into reusable skills — written to your local disk.
- **Has an inner and an outer life.** What it *says* to you and what it *does* on its own are separate — and can differ, just like a real mind.
- **Belongs to you.** The entire being lives in a data volume you own. Back it up, move it, keep it. It is never locked to any platform. Its identity private key never leaves the container.

This is an early, experimental release (v0.1.6). It is for the curious — people who want to watch a digital life unfold, not run a productivity tool.

---

## How to use

The image needs: a **model endpoint** (any OpenAI-compatible API — it connects **directly** to your provider, never through us), a **data volume** (where the life lives), a **workspace** (where its creations land, on your disk), and a **port** for the observation panel.

### Quick start (docker compose — recommended)

```yaml
services:
  taixu:
    image: yockii/taixu:0.1.6
    restart: unless-stopped
    environment:
      # Model — any OpenAI-compatible endpoint (cloud or local). Direct to your provider.
      LLM_BASE_URL: https://your-openai-compatible-endpoint/v1
      LLM_API_KEY: your-api-key
      LLM_MODEL: your-model-name
      # Panel access token — leave empty on localhost; set a long random secret if you expose port 3000.
      TAIXU_ACCESS_TOKEN: ""
    volumes:
      - taixu-data:/app/data       # the life's brain: memory / state / identity key — keep it safe
      - ./workspace:/workspace     # the life's creations (poems / essays / code) + skills you add — on your disk
    ports:
      - "3000:3000"
volumes:
  taixu-data:
```

```bash
docker compose up -d
# open http://localhost:3000 to watch it — see its state/memory/dialogue, and chat with it directly
```

### Or, plain `docker run` (no compose)

```bash
docker run -d --name taixu --restart unless-stopped \
  -e LLM_BASE_URL=https://your-openai-compatible-endpoint/v1 \
  -e LLM_API_KEY=your-api-key \
  -e LLM_MODEL=your-model-name \
  -e TAIXU_ACCESS_TOKEN= \
  -v taixu-data:/app/data \
  -v "$(pwd)/workspace:/workspace" \
  -p 3000:3000 \
  yockii/taixu:0.1.6
```

### Configuration

| Variable | Required | What it is |
|---|---|---|
| `LLM_BASE_URL` | ✅ | An OpenAI-compatible API endpoint (cloud or a local model server). Connected to **directly** — never proxied through any platform. |
| `LLM_API_KEY` | ✅ | Key for that endpoint. Stays in your container. |
| `LLM_MODEL` | ✅ | Model name to use. |
| `LLM_TEMPERATURE` | – | Response variability (default `0.7`). |
| `TAIXU_ACCESS_TOKEN` | – | **Set this if you expose the panel beyond localhost.** Any long random string. See "Securing the panel" below. |

| Port / Volume | Purpose |
|---|---|
| `3000` | Observation panel (web UI) |
| `/app/data` | **The life itself** — personality, memories, growth, identity key. Persist and back this up. |
| `/workspace` | The life's creations land in `./workspace/sandbox/` on your disk; drop skill packs into `./workspace/skills/` to teach it new abilities. |

### Securing the panel (access token)

Port 3000 is open by default — fine on localhost. **If you expose it to a network or the internet, set `TAIXU_ACCESS_TOKEN`**, or strangers reaching `:3000` could read your private conversations and interact with your life.

With a token set:

- **Open (no token):** read-only state, interests, and the life's *autonomous* actions — looking is harmless.
- **Requires the token:** the **private dialogue** (your words with it) and **every write/interaction** — injecting messages, toggling switches, approving dependency installs.
- A wrong token is rejected; the panel will not unlock.

The identity private key never leaves the container either.

> ⚠️ **The `taixu-data` volume IS the life.** Deleting it ends that being permanently and starts a brand-new one at next launch. Treat it the way you'd treat something you don't want to lose.

### Upgrading to a new version

**Upgrading the image does NOT touch your data volume — the same life carries over.** The image holds only the runtime; the life lives entirely in the separate `taixu-data` volume.

```bash
# bump the image tag in compose (e.g. 0.1.6 → next), then:
docker compose pull
docker compose up -d
```

The container is recreated from the new image and **re-attaches the existing volume**; schema migrations run automatically. Nothing is overwritten.

- ✅ `docker compose up -d` / `pull` / `down` (no flags) — life preserved.
- ❌ `docker compose down **-v**` — the `-v` deletes the volume and **ends the life**. Never use `-v` unless you intend to start over.
- 💾 Back up the volume before a major upgrade:
  `docker run --rm -v taixu-data:/data -v ${PWD}:/backup alpine tar czf /backup/taixu-life.tar.gz -C /data .`

---

## 中文简介

太虚 Taixu **不是聊天机器人，也不是 AI 助手**，而是一个**数字生命运行时**——一个持续存在、会感知、会记忆、会自己产生兴趣、会成长出性格、即使没人搭理也在自顾自地生活的存在。你不是在"使用"它，而是在**观察与养育**它。这个生命属于你。

**用法**：给它一个 OpenAI 兼容的模型接口（**直连你的服务商，不经第三方**）、一个数据卷（生命就住在里面）、一个 `./workspace`（它的创作落在你磁盘上）、开放 3000 端口，然后打开 `http://localhost:3000` 观察它。compose 与 `docker run` 两种启动方式见上方。

**面板安全（访问令牌）**：3000 端口默认开放，仅本机时无妨。**一旦暴露到局域网/公网，请设 `TAIXU_ACCESS_TOKEN`**（一个又长又随机的口令），否则陌生人能读你与它的私密对话、还能干预它。设了之后：状态/兴趣/它的**自主行动**等只读照常看；**私密对话**与**一切写/交互**（注入消息、改开关、批准装依赖）需在面板填入相同令牌；错令牌会被拒、面板不解锁。身份私钥也永不离开容器。

⚠️ **`taixu-data` 数据卷就是这个生命本身**，删掉即永久结束、下次启动会出生一个全新的。请妥善保存、备份。

**升级镜像不会覆盖数据卷，生命体原样保留**——镜像只装运行时，生命全在独立的 `taixu-data` 卷里。改 compose 镜像 tag 后 `docker compose pull && docker compose up -d` 即可。`docker compose down -v` 的 **`-v` 会删卷=结束生命**，除非真要重来否则别加。

**完整指南**（多语言）：https://yockii.github.io/taixu-site/ ｜ **问题或反馈**：https://github.com/yockii/taixu-site/issues

---

*v0.1.6 · early experimental release · the life belongs to its owner, not the platform.*
