# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.23-alpine AS builder
WORKDIR /app

# RUN echo 'nameserver 8.8.8.8' >> /etc/resolv.conf
# OSパッケージ（tzdata だけ入れておくと JST が使える）
RUN apk add --no-cache tzdata

# 依存だけ先に解決してビルドキャッシュを効かせる
COPY go.mod go.sum ./
RUN go mod download

# アプリのソース
COPY . .

# 静的リンクで小さめに
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./...

# ---- Runtime stage ----
FROM gcr.io/distroless/base-debian12
# タイムゾーン（JST で日次ジョブを動かすため）
ENV TZ=Asia/Tokyo
WORKDIR /app

# data ディレクトリ（JSON 保存先）
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/server /app/server


# ポートは Gin 側で :8080
EXPOSE 8080
USER 65532:65532

# .env は --env-file / compose で渡す
ENTRYPOINT ["/app/server"]
