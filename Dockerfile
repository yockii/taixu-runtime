# Mindverse Dockerfile
#
# 多阶段：
#   1) frontend  — SvelteKit + Tailwind build → static SPA
#   2) builder   — Go 编译（embed SPA 到二进制）
#   3) llamacpp  — 取 llama.cpp server 二进制（嵌入服务子进程，面板自管）
#   4) runtime   — debian:trixie-slim 运行（glibc 2.41，ABI 匹配 llama.cpp；且有真 chromium .deb）
#
# 嵌入模型 GGUF 不打进镜像：由 runtime 按面板开关下到数据卷 /app/data/models/（见 embedsvc）。

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

# 版本注入：CI（docker.yml）按 git tag 传 --build-arg VERSION=v0.3.0；裸 build 默认 dev。
# 注入到 main.version（web.go），让面板「运行时版本」+ 自更新比对拿到真实版本，
# 否则永远是 dev → 自更新永远误报「有新版」。
ARG VERSION=dev
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w -X main.version=${VERSION}" -trimpath -o /out/taixu ./cmd/runtime
RUN go build -ldflags="-s -w" -trimpath -o /out/taixu-setup  ./cmd/setup

# ---------- 阶段 3：llama.cpp server 二进制来源 ----------
# 嵌入服务以 llama-server 子进程跑（面板自管，见 internal/runtime/embedsvc）。
# 该镜像 Ubuntu24.04/glibc 2.39 编译，与下方 debian:trixie（glibc 2.41）ABI 兼容。
# CN 拉不动 ghcr.io 时：docker pull ghcr.nju.edu.cn/ggml-org/llama.cpp:server 再 retag。
FROM ghcr.io/ggml-org/llama.cpp:server AS llamacpp

# ---------- 阶段 4：运行 ----------
# 基底从 alpine 换 debian:trixie-slim：
#   1) glibc 2.41 → 直接跑 llama.cpp 预编译二进制（alpine/musl 缺 __isoc23_* 等符号跑不了）
#   2) debian 有真 chromium .deb（ubuntu 的 chromium 是 snap，容器内不可用）
FROM docker.m.daocloud.io/library/debian:trixie-slim

# 构建期网络代理（CN 拉 deb.debian.org / pypi 慢或不通时用）。
# 经 `docker compose build --build-arg BUILD_PROXY=http://host.docker.internal:10808` 传入；
# 经 ENV 注入给 apt/pip（小写 http_proxy/https_proxy 它们都认）。**构建末尾清空，绝不留进运行镜像**。
# 注：`docker compose build --build-arg HTTP_PROXY=...` 不会自动注入 RUN 环境，故这里显式 ARG→ENV。
ARG BUILD_PROXY=
ENV http_proxy=${BUILD_PROXY} https_proxy=${BUILD_PROXY} \
    HTTP_PROXY=${BUILD_PROXY} HTTPS_PROXY=${BUILD_PROXY} \
    no_proxy=localhost,127.0.0.1 NO_PROXY=localhost,127.0.0.1

# 系统层：sqlite + Python3 + Node + headless Chromium（rod Tier3 抓取）+ libgomp1（llama.cpp OpenMP 运行时）
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata sqlite3 \
    python3 python3-pip python3-venv \
    nodejs npm \
    chromium \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

# rod 默认会下载自己的 chromium；指向系统 chromium 避免运行时联网下载。
ENV ROD_BROWSER_BIN=/usr/bin/chromium

# llama.cpp server 二进制 + 共享库（libggml*.so / libllama.so）。
# 整目录搬到 /usr/local/lib/llama，二进制软链进 PATH，LD_LIBRARY_PATH 指向库目录。
COPY --from=llamacpp /app/ /usr/local/lib/llama/
RUN ln -sf /usr/local/lib/llama/llama-server /usr/local/bin/llama-server
ENV LD_LIBRARY_PATH=/usr/local/lib/llama

# L0 Python baseline 白名单（docs/SKILLS-AND-TOOLS §5.2）
# --break-system-packages：debian trixie / PEP 668。镜像构建期一次性装，
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

# 清空构建期代理，绝不留进运行镜像（生命体运行时不应走宿主代理）。
ENV http_proxy= https_proxy= HTTP_PROXY= HTTPS_PROXY= no_proxy= NO_PROXY=

WORKDIR /app

COPY --from=builder /out/taixu        /usr/local/bin/taixu
COPY --from=builder /out/taixu-setup  /usr/local/bin/taixu-setup

ENV TAIXU_DATA=/app/data \
    NODE_PATH=/usr/local/lib/node_modules \
    TAIXU_LLAMA_BIN=/usr/local/bin/llama-server \
    TAIXU_SANDBOX=/workspace/sandbox \
    TAIXU_SKILLS=/workspace/skills
# /app/data = 生命的大脑（sqlite 记忆/状态/密钥）；/workspace = 生命的工作目录：
#   /workspace/sandbox 生命 fs.* 写出的诗/文/代码，/workspace/skills 投放的技能。
# 用户 bind-mount /workspace 到本地磁盘即可保留生命的创作（见 docker-compose volumes）。
VOLUME ["/app/data", "/workspace"]

EXPOSE 3000

ENTRYPOINT ["/usr/local/bin/taixu"]
