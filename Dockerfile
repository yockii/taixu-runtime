# Mindverse Phase 0.1 Dockerfile
#
# 当前为最小可运行版本：
#   - 仅 Go runtime + SQLite（modernc 纯 Go 驱动，无 CGO）
#   - 不含 bge-m3 模型权重（Phase 0.3 加入）
#   - 不含前端构建产物（Phase 0.4 加入）
#   - 不含 Python/Node 工具链 / llama.cpp（Phase 0.2-0.3 加入）
#
# 参考最终形态：docs/TECH-STACK.md §11.1。

# ---------- 阶段 1：Go 编译 ----------
# TECH-STACK 锁 1.26+；当前作者机器 1.25.6，待 1.26 GA 后切换。
FROM docker.m.daocloud.io/library/golang:1.25-alpine AS builder

WORKDIR /src

# 先拷依赖文件以利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

# 拷源
COPY cmd/      ./cmd/
COPY internal/ ./internal/

# 静态构建（modernc/sqlite 纯 Go，无需 CGO）
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -trimpath -o /out/mindverse        ./cmd/runtime
RUN go build -ldflags="-s -w" -trimpath -o /out/mindverse-setup  ./cmd/setup

# ---------- 阶段 2：运行 ----------
FROM docker.m.daocloud.io/library/alpine:3.20

RUN apk add --no-cache ca-certificates tzdata sqlite

# Phase 0.1 暂以 root 运行（Phase 0.2+ 加 entrypoint chown 后切回 mindverse 用户）
WORKDIR /app

COPY --from=builder /out/mindverse        /usr/local/bin/mindverse
COPY --from=builder /out/mindverse-setup  /usr/local/bin/mindverse-setup

ENV MINDVERSE_DATA=/app/data
VOLUME ["/app/data", "/sandbox"]

EXPOSE 3000

ENTRYPOINT ["/usr/local/bin/mindverse"]
