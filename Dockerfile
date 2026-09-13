FROM golang:1.27.1-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY migrations/ migrations/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot/

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -u 10001 -h /data bot

WORKDIR /data

COPY --from=build /out/bot /app/bot
COPY configs/config.example.yaml /app/configs/config.yaml
COPY migrations/ /app/migrations/

RUN mkdir -p /data /app/configs \
	&& chown -R bot:bot /data /app

USER bot

ENV CONFIG_PATH=/app/configs/config.yaml \
	MGKE_DB_PATH=/data/sqlite3.db \
	MGKE_CHAT_DB_PATH=/data/bot_chats.db \
	MGKE_CACHE_DIR=/data/cache/rasp \
	MGKE_HTTP_PORT=8081

EXPOSE 8081

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
	CMD wget -q -O /dev/null "http://127.0.0.1:${MGKE_HTTP_PORT}/api/health" || exit 1

ENTRYPOINT ["/app/bot"]
