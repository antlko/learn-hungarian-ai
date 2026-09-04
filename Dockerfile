# syntax=docker/dockerfile:1

# --- Stage 1: build the Vue single-page app ---
# Pinned to BUILDPLATFORM like the Go stage below. The output is JavaScript --
# identical whatever the target arch -- so there is nothing to gain from running
# it on the target, and plenty to lose: without this, the linux/arm64 half of the
# release build runs `npm ci` and Vite under QEMU on an amd64 runner, which turns
# a ~1 minute stage into a ~15 minute one and is the slowest thing in the tag
# pipeline by an order of magnitude.
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
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
# An empty /data to carry into the final image. distroless has no shell, so this
# is the only way to get a directory with a non-root owner in there -- and the
# next stage explains why that matters.
RUN mkdir -p /data

# --- Stage 3: minimal runtime image ---
FROM gcr.io/distroless/static:nonroot
# image.source is what links the GHCR package to the repository, which is also
# what makes the package inherit the repo's visibility and README.
LABEL org.opencontainers.image.title="tanulom" \
      org.opencontainers.image.description="Self-hosted, research-grounded Hungarian trainer" \
      org.opencontainers.image.source="https://github.com/antlko/learn-hungarian-ai" \
      org.opencontainers.image.url="https://github.com/antlko/learn-hungarian-ai" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /tanulom /tanulom
# /data must exist IN THE IMAGE, owned by the user we drop to. When Docker
# initialises an empty named volume it copies the image's ownership and mode at
# the mount point -- and when the path is absent from the image it creates the
# volume root:root 0755 instead. distroless/static ships no /data, so the
# documented `docker run -v tanulom-data:/data` died on first boot with
# `create media dir: mkdir /data/media: permission denied`, before it could
# install the starter pack. 65532 is distroless's `nonroot` uid, written
# numerically so it matches whatever the kernel sees on the volume -- the same
# number you would `chown` a host directory to for a bind mount.
COPY --from=build --chown=65532:65532 /data /data
ENV TANULOM_DATA_DIR=/data \
    TANULOM_ADDR=:8012
EXPOSE 8012
VOLUME ["/data"]
USER nonroot:nonroot
ENTRYPOINT ["/tanulom"]
