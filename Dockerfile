# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.27.1
ARG BUN_VERSION=1.3.10

# Build the static SPA on the build host, even when targeting another architecture.
FROM --platform=$BUILDPLATFORM oven/bun:${BUN_VERSION} AS bun

FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS frontend
COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
WORKDIR /src/frontend
COPY frontend/package.json frontend/bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile
COPY frontend/ ./
# Production uses the API on the same origin; do not bake in a development URL.
RUN NODE_OPTIONS=--dns-result-order=ipv4first npm run build

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS backend
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY main.go ./
COPY internal/ ./internal/
COPY migrations/ ./migrations/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X github.com/chand1012/lyphe/internal/version.Version=${VERSION} -X github.com/chand1012/lyphe/internal/version.CommitHash=${COMMIT} -X github.com/chand1012/lyphe/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/lyphe .

# Collect target-architecture FFmpeg/FFprobe libraries and assimilate llamafile.
FROM debian:trixie-slim AS runtime-deps
ARG TARGETARCH
RUN case "$TARGETARCH" in amd64|arm64) ;; *) echo "Local models require amd64 or arm64" >&2; exit 1;; esac
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg python3-minimal \
    && rm -rf /var/lib/apt/lists/*
COPY bin/llamafile /models/
COPY bin/whistle/ /models/whistle/
COPY scripts/verify-whistle.py /verify-whistle.py
RUN python3 /verify-whistle.py /models/whistle
COPY docker/assimilate.py /assimilate.py
RUN python3 /assimilate.py "$TARGETARCH" /models/llamafile \
    && chmod 755 /models/llamafile
# Copy only the tools and their shared-library closure, not a Debian installation.
COPY docker/copy-runtime.sh /copy-runtime.sh
RUN /bin/sh /copy-runtime.sh \
    && mkdir -p /runtime/app/pb_data /runtime/tmp \
    && chown 65532:65532 /runtime/app/pb_data /runtime/tmp \
    && chmod 1777 /runtime/tmp

FROM gcr.io/distroless/base-debian13:nonroot AS final
WORKDIR /app
COPY --from=runtime-deps /runtime/ /
COPY --from=backend /out/lyphe /app/lyphe
COPY --from=frontend /src/frontend/build/client/ /app/frontend/build/client/
COPY --from=runtime-deps /models/ /app/bin/
ENV LYPHE_LLAMAFILE=/app/bin/llamafile \
    LYPHE_WHISTLE_WASM=/app/bin/whistle/needle.wasm \
    LYPHE_WHISTLE_MODEL=/app/bin/whistle/whistle.cact \
    LYPHE_FFMPEG=/usr/bin/ffmpeg \
    LYPHE_FFPROBE=/usr/bin/ffprobe \
    LYPHE_LLM_ADDRESS=127.0.0.1:8081 \
    TMPDIR=/tmp
USER 65532:65532
# Exec-form checks work without a shell and catch missing runtime libraries.
RUN ["/usr/bin/ffmpeg", "-version"]
RUN ["/usr/bin/ffprobe", "-version"]
EXPOSE 8090
VOLUME ["/app/pb_data"]
ENTRYPOINT ["/app/lyphe"]
CMD ["serve", "--http=0.0.0.0:8090", "--dir=/app/pb_data", "--staticPath=/app/frontend/build/client"]
