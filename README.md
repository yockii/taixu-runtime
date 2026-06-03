# Mindverse · 心域文明

> **Digital Life Runtime** — 数字生命运行时平台。
>
> **不是** ChatGPT / Agent Framework / Assistant。
> **是** 持续存在、自主演化、属于用户的数字生命体宿主。

## 状态

**Phase 0 · 0.1 骨架搭建** — 作者本人自托管单机原型期。

详见 `docs/PHASE-0-PRD.md` §3。

## 文档

- `CLAUDE.md` — AI 协作指引
- `docs/00-README.md` — 设计文档地图（必读入口）
- `docs/TECH-STACK.md` — 技术栈选型（Phase 0）
- `docs/PHASE-0-PRD.md` — Phase 0 实施 PRD
- `docs/COMMERCIAL.md` — 商业模型基线

## 工程铁律

- Go 依赖：禁手写 `go.mod` / `go.sum`，用 `go get <pkg>@<version>` + `go mod tidy`
- 前端依赖：禁手写 `web/package.json` / `web/pnpm-lock.yaml`，用 `cd web && pnpm add <pkg>`
- 详见 `docs/TECH-STACK.md` §17

## 快速开始（Phase 0.1 完成后）

```bash
cp .env.example .env
# 编辑 .env 填入 LLM / 飞书凭证
docker compose up -d
# 浏览器访问 http://localhost:3000
```

## 协议

待定（Phase 0 私有）。
