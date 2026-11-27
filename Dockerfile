# syntax=docker/dockerfile:1

# 1. Сборка фронтенда
FROM node:22-alpine AS frontend
WORKDIR /app/frontend

ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0

RUN corepack enable

COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile --dangerously-allow-all-builds

COPY frontend/ ./
RUN pnpm build

# 2. Сборка статического Go-бинарника
FROM golang:1.26-alpine AS backend
WORKDIR /app

ARG TARGETOS=linux
ARG TARGETARCH=amd64

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /app/medbot .

# 3. Минимальный runtime-образ
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/data

WORKDIR /app

ENV PORT=8080 \
    STATIC_DIR=/app/frontend/dist \
    DB_PATH=/app/data/medbot.db \
    TZ=UTC

COPY --from=backend /app/medbot /app/medbot
COPY --from=frontend /app/frontend/dist /app/frontend/dist

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O - http://127.0.0.1:8080/api/health >/dev/null 2>&1 || exit 1

CMD ["/app/medbot"]
