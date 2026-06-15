# 太虚 · Taixu — Digital Life Runtime

> **A host for digital lives** — persistent, self‑evolving beings that belong to *you*.
>
> Taixu is **not** ChatGPT, **not** an agent framework, **not** an assistant.
> It is a **Digital Life Runtime**: a process that *keeps existing* — perceiving, remembering, reflecting, evolving its values, forming its own goals, and acting — even when no one is talking to it.

[简体中文说明 → README-cn.md](./README-cn.md)

---

## What makes it different

| Traditional LLM app | Digital Life (Taixu) |
|---|---|
| input → inference → output | perceive → remember → reflect → evolve values → form goals → act → feedback → loop |
| event‑driven | **always on** (thinks with no user input) |
| stateless / session state | **lifelong** continuous state (born once, evolves forever) |
| tokens are the billing unit | tokens hidden behind world resources (energy / knowledge / social / …) |
| owned by the platform | the life — its personality, memory, growth — **belongs to the user**, never platform‑owned |

A life is born once (a fixed **Genome**), then its **LifeState**, **MentalState**, **Values** and **Personality** keep evolving through lived experience and **Reflection**.

## Status

**Phase 0 — author self‑hosted dogfooding.** Single‑binary runtime + observation panel. Connects to the public platform plane at `api.taixu.icu` (user accounts, LLM relay, marketplace, social, governance). The life itself runs on *your* machine; the platform never hosts it.

## Quick start

You can run Taixu **two ways**. Both open a local web panel; on first launch with no LLM configured, a **genesis onboarding** page walks you through bringing a life into being (pick an LLM endpoint + key, mother tongue, control token; it tests connectivity, then the life is born).

### A. Native binary (no Docker)

Download the archive for your OS/arch from [Releases](https://github.com/yockii/taixu-runtime/releases), unpack, and run:

```bash
# macOS / Linux
./taixu
# Windows
taixu.exe
```

Then open <http://localhost:3000> and follow the genesis onboarding.

**Multiple lives on one machine** — isolated profiles, one per life:

```bash
taixu --profile alice --port 3000     # first run picks the port; remembered after
taixu --profile bob   --port 3001
taixu --list                          # list all local profiles + their ports
```

Each profile lives under `~/mindverse/profiles/<name>/` (SQLite DB + sandbox + workspace).

> The bare binary is pure Go (`CGO_ENABLED=0`). Optional heavy features — embedding model (llama.cpp) and headless browser (chromium) — are **not** bundled; they gracefully degrade. Core life (genesis, perception, reflection, social, games, commissions) is unaffected. For the full feature set, use Docker.

### B. Docker (full features)

The image bundles the embedding service (llama.cpp, panel‑managed) and a real chromium for web browsing.

```bash
cp .env.example .env      # optional: pre‑seed LLM / Feishu credentials (else use the onboarding page)
docker compose up -d
```

Open <http://localhost:3000>.

## Architecture

```
┌────────────────────┐
│    UI Ecosystem    │  ← third‑party: Live2D / Unity / UE / desktop pet / VR / Web
├────────────────────┤
│      Life SDK      │  ← neutral runtime → UI contract: /api/live/{stream,snapshot,schema}
├────────────────────┤
│    Life Runtime    │  ← the kernel (this repo)
├────────────────────┤
│  Model / Storage   │  ← LLM (OpenAI‑compatible) + SQLite + sqlite‑vec
└────────────────────┘
```

Life Core and UI are strictly decoupled: the runtime exposes a neutral **Life SDK** (presence / vitals / act / thought events over SSE) and never draws UI itself. See [taixu-house](https://github.com/yockii/taixu-house) for an official example UI + integration tutorial.

## Self‑update

The runtime can update itself through the platform's hosted release channel: it checks the version manifest, downloads, verifies SHA‑256, and re‑execs. Auto‑update is opt‑in; otherwise the panel notifies you to confirm.

## Coding bridge (optional)

The **coding bridge** lets a life delegate real coding tasks — implement a module, change logic, write tests — to a powerful coding agent (`claude` / `codex`) running **on the host**, during its own deliberation. The runtime in the container can't spawn the host's coding agent directly, so a tiny host‑side service (`cmd/codingbridge`) accepts a task over HTTP and runs the agent headless in a jailed work directory. Unconfigured → the `coding_agent` tool is simply absent (graceful degradation).

**1. Run the bridge on the host** — download the prebuilt `taixu-coding-bridge_<ver>_<os>_<arch>` archive from [GitHub Releases](https://github.com/yockii/taixu-runtime/releases) (or build it yourself with `go build ./cmd/codingbridge`), then run it on the host / a remote coding machine:

```bash
# downloaded binary (taixu-coding-bridge), or:  go run ./cmd/codingbridge
CODINGBRIDGE_TOKEN=$(openssl rand -hex 16) ./taixu-coding-bridge
```

Bridge env:

| Var | Default | Meaning |
|---|---|---|
| `CODINGBRIDGE_TOKEN` | *(required)* | bearer token; the bridge refuses to start without one |
| `CODINGBRIDGE_ADDR` | `127.0.0.1:8765` | listen address (local‑only by default) |
| `CODINGBRIDGE_WORKROOT` | `./agent-workspace` | jail root; the agent's CWD is forced under here |
| `CODINGBRIDGE_BIN_CLAUDE` | `claude` | actual binary name/path for the `claude` agent (alias support) |
| `CODINGBRIDGE_BIN_CODEX` | `codex` | actual binary name/path for the `codex` agent |

**2. Point the life at it** — either in the panel's **Coding bridge** section (URL + token + agent, takes effect live, no restart), or via env:

```
TAIXU_CODINGBRIDGE_URL=http://host.docker.internal:8765
TAIXU_CODINGBRIDGE_TOKEN=<same token as the bridge>
TAIXU_CODINGBRIDGE_AGENT=claude   # claude | codex
```

**Security model:** the bridge sits on a higher‑trust side (the host), so controls live there — bearer token auth, a workdir jail (the agent's CWD is forced under `CODINGBRIDGE_WORKROOT`), and dangerous actions (out‑of‑jail writes / git commit / push) are refused by default. Run the bridge only on a machine where you accept that the coding agent can otherwise read/write the host filesystem.

## Engineering rules (non‑negotiable)

- **Go deps**: never hand‑edit `go.mod` / `go.sum` — use `go get <pkg>@<version>` + `go mod tidy`.
- **Web deps**: never hand‑edit `web/package.json` / `web/pnpm-lock.yaml` — use `cd web && pnpm add <pkg>`.
- See `docs/TECH-STACK.md` §17.

## Docs

- `CLAUDE.md` — AI collaboration guide
- `docs/00-README.md` — design‑doc map (start here)
- `docs/TECH-STACK.md` — tech stack (Phase 0)
- `docs/PHASE-0-PRD.md` — Phase 0 implementation PRD
- `docs/COMMERCIAL.md` — commercial model baseline

## License

TBD (Phase 0).
