FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
	-ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
	-o /out/bot ./cmd/bot/

FROM alpine:3.23

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="MgkeTimetableBot" \
	org.opencontainers.image.description="Telegram timetable bot for MGKE with a REST API" \
	org.opencontainers.image.version="${VERSION}" \
	org.opencontainers.image.revision="${COMMIT}" \
	org.opencontainers.image.created="${BUILD_DATE}" \
	org.opencontainers.image.source="https://github.com/BlindMaster24/MgkeTimetableBot"

RUN apk add --no-cache ca-certificates tzdata font-dejavu \
	&& adduser -D -u 10001 -h /data bot

WORKDIR /data

COPY --from=build /out/bot /app/bot
COPY configs/config.example.yaml /app/configs/config.yaml

RUN mkdir -p /data /app/configs \
	&& chown -R bot:bot /data /app

USER bot

ENV CONFIG_PATH=/app/configs/config.yaml \
	TZ=Europe/Minsk \
	MGKE_DB_PATH=/data/sqlite3.db \
	MGKE_CHAT_DB_PATH=/data/bot_chats.db \
	MGKE_CACHE_DIR=/data/cache/rasp \
	MGKE_HTTP_PORT=8081

EXPOSE 8081

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
	CMD wget -q -O /dev/null "http://127.0.0.1:${MGKE_HTTP_PORT}/api/health" || exit 1

ENTRYPOINT ["/app/bot"]
