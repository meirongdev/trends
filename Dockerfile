# syntax=docker/dockerfile:1

# ---- 前端构建(与目标架构无关,跑在 BUILDPLATFORM 上)----
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build            # 产物 -> /web/dist

# ---- Go 构建(modernc.org/sqlite 纯 Go,CGO 关闭 → 交叉编到 TARGETARCH,无需 QEMU)----
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# SPA 产物嵌入 internal/api/dist(等价于 Makefile 的 `web` 目标)
RUN rm -rf internal/api/dist && mkdir -p internal/api/dist
COPY --from=web /web/dist/ internal/api/dist/
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/trends ./cmd/trends

# ---- 运行时(静态 + nonroot)----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /data
COPY --from=build /out/trends /usr/local/bin/trends
ENV DB_PATH=/data/trends.db \
    API_LISTEN_ADDR=:8080 \
    TZ=UTC
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/trends"]
