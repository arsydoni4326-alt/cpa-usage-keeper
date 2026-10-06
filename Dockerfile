FROM --platform=$BUILDPLATFORM node:24-alpine AS web-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS go-builder
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends build-essential && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=web-builder /app/web/dist ./web/dist
COPY web/static.go ./web/static.go
ARG APP_VERSION=dev
ARG APP_COMMIT=unknown
RUN set -eux; \
    export BUILD_DATE="$(date +%Y-%m-%d)"; \
    CGO_ENABLED=1 GOOS=linux go build \
        -ldflags="-s -w -X cpa-usage-keeper/internal/version.Version=${APP_VERSION} -X 'main.Commit=${APP_COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" \
        -o /out/cpa-usage-keeper ./cmd/server/main.go

FROM --platform=$BUILDPLATFORM debian:bookworm-slim
WORKDIR /
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    gosu \
    wget \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -r -s /bin/bash -d /app -m app \
    && mkdir -p /data \
    && chown -R app:app /data
COPY --from=go-builder /out/cpa-usage-keeper /app/cpa-usage-keeper
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN sed -i 's/\r$//' /usr/local/bin/docker-entrypoint.sh \
    && chmod +x /usr/local/bin/docker-entrypoint.sh
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -q --spider "http://127.0.0.1:${APP_PORT:-8080}${APP_BASE_PATH:-}/healthz" || exit 1
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["/app/cpa-usage-keeper"]
