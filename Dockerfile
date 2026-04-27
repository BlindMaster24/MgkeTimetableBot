ARG NODE_VERSION=22

FROM node:${NODE_VERSION}-bookworm-slim AS deps
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        python3 \
        pkg-config \
        libcairo2-dev \
        libpango1.0-dev \
        libjpeg-dev \
        libgif-dev \
        libpixman-1-dev \
        libsqlite3-dev \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml .npmrc ./
ENV npm_config_build_from_source=true
RUN pnpm install --frozen-lockfile \
    && pnpm rebuild sqlite3 canvas

FROM node:${NODE_VERSION}-bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
        libcairo2 \
        libpango-1.0-0 \
        libpangocairo-1.0-0 \
        libjpeg62-turbo \
        libgif7 \
        libpixman-1-0 \
        libsqlite3-0 \
        fonts-dejavu-core \
        tini \
        curl \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN corepack enable
WORKDIR /app

ENV NODE_ENV=production
ENV TZ=Europe/Minsk

COPY package.json pnpm-lock.yaml .npmrc tsconfig.json ./
COPY --from=deps /app/node_modules ./node_modules
COPY src ./src
COPY scripts ./scripts
COPY public ./public
COPY config.example.ts config.scheme.ts ./
COPY config.t[s] ./
RUN if [ ! -f config.ts ]; then cp config.example.ts config.ts; fi

RUN useradd --system --uid 1001 --home /app --shell /usr/sbin/nologin app \
    && mkdir -p /app/cache /app/logs \
    && chown -R app:app /app
USER app

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl --silent --max-time 3 -o /dev/null -w '%{http_code}' http://127.0.0.1:8081/api/parser-health | grep -qE '^[234]' || exit 1

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["pnpm", "start"]
