# === Сборка ===
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Копируем модули и скачиваем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект и собираем бинарь server
COPY . .
RUN go build --no-cache -o server ./cmd/server

# === Релизный образ ===
FROM alpine:latest
RUN apk add --no-cache ca-certificates

WORKDIR /app
# Копируем из builder собранный бинарь
COPY --from=builder /app/server .