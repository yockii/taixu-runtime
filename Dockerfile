# Mindverse Phase 0.4 Dockerfile
#
# 多阶段：
#   1) frontend  — SvelteKit + Tailwind build → static SPA
#   2) builder   — Go 编译（embed SPA 到二进制）
#   3) runtime   — Alpine 最小运行
#
# 暂未含 bge-m3 权重（Phase 0.5+ 加）+ Python/Node 工具链（按需）。

# ---------- 阶段 1：前端构建 ----------
FROM docker.m.daocloud.io/library/node:22-alpine AS frontend

# corepack 拉 pnpm 易超时；改 npm 装。
# 用 npmmirror 装 pnpm 本身（小依赖）；之后 pnpm 用默认 registry 拉项目依赖。
RUN npm install -g pnpm@10.28.1 --registry=https://registry.npmmirror.com

WORKDIR /web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm run build

# ---------- 阶段 2：Go 编译（embed SPA） ----------
# TECH-STACK 锁 1.26+；作者机器 1.25.6，待 1.26 GA 后切换。
FROM docker.m.daocloud.io/library/golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/      ./cmd/
COPY internal/ ./internal/

# 将前端 build 产物拷到 embed 目录（cmd/runtime/webbuild/）。
COPY --from=frontend /web/build/ ./cmd/runtime/webbuild/

ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -trimpath -o /out/mindverse        ./cmd/runtime
RUN go build -ldflags="-s -w" -trimpath -o /out/mindverse-setup  ./cmd/setup

# ---------- 阶段 3：运行 ----------
FROM docker.m.daocloud.io/library/alpine:3.20

RUN apk add --no-cache ca-certificates tzdata sqlite

WORKDIR /app

COPY --from=builder /out/mindverse        /usr/local/bin/mindverse
COPY --from=builder /out/mindverse-setup  /usr/local/bin/mindverse-setup

ENV MINDVERSE_DATA=/app/data
VOLUME ["/app/data", "/sandbox"]

EXPOSE 3000

ENTRYPOINT ["/usr/local/bin/mindverse"]
