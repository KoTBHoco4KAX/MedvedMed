# syntax=docker/dockerfile:1

# 1. Сборка фронтенда
FROM node:22-alpine AS frontend
WORKDIR /app/frontend

ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0

RUN corepack enable

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm build

# 2. Сборка статического Go-бинарника
FROM golang:1.26-alpine AS backend
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/medbot .

# 3. Минимальный runtime-образ
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/data

WORKDIR /app

COPY --from=backend /app/medbot /app/medbot
COPY --from=frontend /app/frontend/dist /app/frontend/dist

EXPOSE 8080

CMD ["/app/medbot"]
