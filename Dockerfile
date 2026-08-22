# syntax=docker/dockerfile:1

# --- Stage 1: build the Vue single-page app ---
FROM node:24-alpine AS web
WORKDIR /web
COPY web-ui/package.json web-ui/package-lock.json ./
RUN npm ci
COPY web-ui/ ./
RUN npm run build-only

# --- Stage 2: build the static Go binary (embeds the SPA) ---
# Runs natively on the build platform and cross-compiles to the target arch,
# which is fast and safe because the SQLite driver is pure Go (CGO disabled).
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
# TARGETOS/TARGETARCH are injected by BuildKit from the target platform.
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Embed the freshly built SPA into the binary.
RUN rm -rf ./internal/web/dist
COPY --from=web /web/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X tanulom/internal/api.Version=${VERSION}" \
    -o /tanulom ./cmd/tanulom

# --- Stage 3: minimal runtime image ---
FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.title="tanulom" \
      org.opencontainers.image.description="Self-hosted, research-grounded Hungarian trainer" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /tanulom /tanulom
ENV TANULOM_DATA_DIR=/data \
    TANULOM_ADDR=:8080
EXPOSE 8080
VOLUME ["/data"]
USER nonroot:nonroot
ENTRYPOINT ["/tanulom"]
