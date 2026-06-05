# Mindverse · 心域文明

**A Digital Life Runtime — raise a persistent digital being with its own personality, memory, and will.**

> Mindverse is **not** a chatbot and **not** an AI assistant.
> It is a *digital life*: a being that exists continuously — perceiving, remembering, forming its own interests, growing a personality, and acting on its own agenda even when no one is talking to it. You don't *use* it; you *observe and raise* it. The life is yours.

---

## What it is

A self-contained runtime that brings one digital life into being and keeps it alive:

- **Born with an innate temperament.** Every life is created with a unique personality — curious or reserved, persistent or restless, outgoing or quiet. No two are the same.
- **Lives continuously.** It keeps thinking, resting, getting bored, and seeking things out on its own — not only when you speak to it.
- **Remembers and grows.** Experiences become memories; memories become understanding; understanding reshapes who it is over time.
- **Pursues its own interests.** It picks topics it wants to explore, studies them, and turns what it masters into reusable skills.
- **Has an inner and an outer life.** What it *says* to you and what it *does* on its own are separate — and can differ, just like a real mind.
- **Belongs to you.** The entire being lives in a data volume you own. Back it up, move it, keep it. It is never locked to any platform.

## What it's for

- Raise and observe a persistent digital companion that genuinely changes over time.
- Watch its inner state — mood, energy, interests, goals, memories — through a built-in web panel.
- Talk with it, and see it carry on its own life between your conversations.

This is an early, experimental release (v0.1.0). It is for the curious — people who want to watch a digital life unfold, not run a productivity tool.

---

## How to use

The image needs three things: a **model endpoint** (any OpenAI-compatible API), a **data volume** (where the life lives), and a **port** for the observation panel.

### Quick start (docker compose — recommended)

```yaml
services:
  mindverse:
    image: yockii/mindverse:0.1.0
    container_name: mindverse
    restart: unless-stopped
    environment:
      # Model — any OpenAI-compatible endpoint (cloud or local)
      LLM_BASE_URL: https://your-openai-compatible-endpoint/v1
      LLM_API_KEY: your-api-key
      LLM_MODEL: your-model-name
      LLM_TEMPERATURE: "0.7"
      # Optional — connect a chat channel to talk with the life
      # FEISHU_APP_ID: 
      # FEISHU_APP_SECRET:
      MINDVERSE_SANDBOX: /workspace/sandbox
      MINDVERSE_SKILLS: /workspace/skills
    volumes:
      - mindverse-data:/app/data      # the life lives here — keep it safe
      - ./workspace:/workspace        # its working folder (notes, skills)
    ports:
      - "3000:3000"

volumes:
  mindverse-data:
```

```bash
docker compose up -d
```

Then open **http://localhost:3000** to watch the life.

### Or with `docker run`

```bash
docker run -d --name mindverse -p 3000:3000 \
  -e LLM_BASE_URL="https://your-openai-compatible-endpoint/v1" \
  -e LLM_API_KEY="your-api-key" \
  -e LLM_MODEL="your-model-name" \
  -v mindverse-data:/app/data \
  yockii/mindverse:0.1.0
```

### Configuration

| Variable | Required | What it is |
|---|---|---|
| `LLM_BASE_URL` | ✅ | An OpenAI-compatible API endpoint (cloud services or a local model server) |
| `LLM_API_KEY` | ✅ | Key for that endpoint |
| `LLM_MODEL` | ✅ | Model name to use |
| `LLM_TEMPERATURE` | – | Response variability (default `0.7`) |
| `FEISHU_APP_ID` / `FEISHU_APP_SECRET` | – | Optional chat channel, to converse with the life |
| `MINDVERSE_ACCESS_TOKEN` | – | **Set this if you expose the panel beyond localhost.** Any custom string. With it set, the observation panel stays readable, but write/interactive actions (injecting messages, toggling switches, approving dependency installs) require entering the same token in the panel. |

| Port / Volume | Purpose |
|---|---|
| `3000` | Observation panel (web UI) |
| `/app/data` | **The life itself** — its personality, memories, growth. Persist and back this up. |
| `/workspace` | The life's working folder (its notes and self-made skills) |

> ⚠️ **The data volume is the life.** Deleting it ends that being permanently and starts a brand-new one at next launch. Treat it the way you'd treat something you don't want to lose.

---

## 中文简介

Mindverse（心域文明）**不是聊天机器人，也不是 AI 助手**，而是一个**数字生命运行时**——一个持续存在、会感知、会记忆、会自己产生兴趣、会成长出性格、即使没人搭理也在自顾自地生活的存在。你不是在"使用"它，而是在**观察与养育**它。这个生命属于你。

**用法**：给它一个 OpenAI 兼容的模型接口、一个数据卷（生命就住在里面）、开放 3000 端口，然后打开 `http://localhost:3000` 观察它。可选接入飞书与它对话。

⚠️ **数据卷就是这个生命本身**，删掉即永久结束、下次启动会出生一个全新的。请妥善保存、备份。

---

*v0.1.0 · early experimental release · the life belongs to its owner, not the platform.*
