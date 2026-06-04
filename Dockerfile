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

# 容器内默认 GOPROXY=proxy.golang.org 在国内常 TLS 超时；用 goproxy.cn。
ENV GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=off

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

# 系统层：sqlite + Python3 + Node20 + headless Chromium（rod Tier3 抓取）
RUN apk add --no-cache \
    ca-certificates tzdata sqlite \
    python3 py3-pip \
    nodejs npm \
    chromium-swiftshader

# rod 默认会下载自己的 chromium；指向系统 chromium 避免运行时联网下载。
ENV ROD_BROWSER_BIN=/usr/bin/chromium-browser

# L0 Python baseline 白名单（docs/SKILLS-AND-TOOLS §5.2）
# --break-system-packages：alpine 3.20 / PEP 668。镜像构建期一次性装，
# Phase 0 可接受全局 site-packages（私有目录隔离留给 D.2 skill loader）。
RUN pip3 install --break-system-packages --no-cache-dir \
    httpx requests \
    beautifulsoup4 lxml \
    trafilatura \
    pyyaml pillow markdown feedparser \
    python-dateutil

# L0 Node baseline 白名单（global，挂 /usr/lib/node_modules）
RUN npm install -g --registry=https://registry.npmmirror.com \
    axios cheerio dayjs js-yaml marked

WORKDIR /app

COPY --from=builder /out/mindverse        /usr/local/bin/mindverse
COPY --from=builder /out/mindverse-setup  /usr/local/bin/mindverse-setup

ENV MINDVERSE_DATA=/app/data \
    NODE_PATH=/usr/local/lib/node_modules
VOLUME ["/app/data", "/sandbox"]

EXPOSE 3000

ENTRYPOINT ["/usr/local/bin/mindverse"]
