# tanulom — the whole project, locally, without Docker.
#
# Two toolchains live side by side (Go in backend/, npm in web-ui/) and in
# development they have to run at the same time: Vite serves the SPA and proxies
# /api and /media to Go. This file is the one place that knows how to drive both
# together, so a fresh clone needs nothing but the toolchains themselves.
#
# `make` with no target lists everything. Nothing here needs Docker; the
# Dockerfile automates the same `build` steps for the published image.

SHELL := /bin/sh
.DEFAULT_GOAL := help

BACKEND_DIR  := backend
WEB_DIR      := web-ui
LANGPACK_DIR := langpack
STARTER_DIR  ?= backend/internal/lang/starter
VOICES_DIR   := $(LANGPACK_DIR)/voices
EMBED_DIR    := $(BACKEND_DIR)/internal/web/dist
BIN          := bin/tanulom

DATA_DIR      ?= $(CURDIR)/data
PACK          ?= $(DATA_DIR)/langpack-hu-v1.sqlite
PYTHON        ?= python3
# The ear trainer's three CC0 Hungarian talkers. Pinned to `main` because
# rhasspy/piper-voices publishes no tags; the .json voice cards beside each
# model are tracked, so a swapped model is visible in a diff.
PIPER_BASE    ?= https://huggingface.co/rhasspy/piper-voices/resolve/main/hu/hu_HU
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GO_TEST_FLAGS ?= -count=1

# The dev-only listen address. It is NOT a default and it is not read from
# internal/config: it must equal the proxy target spelled literally in
# web-ui/vite.config.ts, so if that file moves, this moves with it.
#
# Three files write this number down outside internal/config — here,
# vite.config.ts, and the Dockerfile's TANULOM_ADDR/EXPOSE. They drifted once:
# config.DefaultAddr moved 8080 -> 8012 and the other three did not follow, which
# left `make dev` proxying into nothing and `docker compose up` publishing a port
# the container was not listening on. If you change the port, change all four.
#
# `make run` passes no --addr at all, so the single binary goes on using
# config.DefaultAddr and nothing here can make that default drift.
DEV_ADDR ?= :8012

GO_HINT   := Install Go 1.26 or newer from https://go.dev/dl (macOS: brew install go)
NODE_HINT := Install Node 22+ and npm from https://nodejs.org (macOS: brew install node)
PY_HINT   := Install Python 3.11+ from https://www.python.org/downloads (macOS: brew install python)

# A missing toolchain should name itself and say what to install, rather than
# surfacing as "command not found" fifteen lines into a build.
define need
@command -v $(1) >/dev/null 2>&1 || { printf '\n  Required tool "%s" is not on PATH.\n  %s\n\n' '$(1)' '$(2)' >&2; exit 1; }
endef

##@ Develop

.PHONY: dev
dev: $(WEB_DIR)/node_modules ensure-spa ## Backend + Vite together in the foreground; Ctrl-C stops both
	$(call need,go,$(GO_HINT))
	@mkdir -p '$(DATA_DIR)'
	@printf '\n  backend   http://localhost%s\n  frontend  http://localhost:5173   <- open this one\n\n' '$(DEV_ADDR)'
# `kill 0` signals the whole process group, which is what reaches the binary that
# `go run` compiled and left running underneath itself — killing only the jobs
# make can see would strand it holding the port. Exiting 0 from the trap keeps a
# deliberate Ctrl-C from being reported as a failed build.
	@trap 'kill 0; exit 0' INT TERM; \
		( cd '$(BACKEND_DIR)' && exec go run ./cmd/tanulom --data-dir '$(DATA_DIR)' --addr '$(DEV_ADDR)' ) & \
		( cd '$(WEB_DIR)' && exec npm run dev ) & \
		wait

.PHONY: backend
backend: ensure-spa ## Run the Go API alone, on the port the Vite proxy expects
	$(call need,go,$(GO_HINT))
	@mkdir -p '$(DATA_DIR)'
	cd '$(BACKEND_DIR)' && go run ./cmd/tanulom --data-dir '$(DATA_DIR)' --addr '$(DEV_ADDR)'

.PHONY: frontend
frontend: $(WEB_DIR)/node_modules ## Run the Vite dev server alone on :5173
	cd '$(WEB_DIR)' && npm run dev

##@ Build and run

.PHONY: spa
spa: $(WEB_DIR)/node_modules ## Build the SPA and embed it into the Go tree
	cd '$(WEB_DIR)' && npm run build
	rm -rf '$(EMBED_DIR)'
	cp -r '$(WEB_DIR)/dist' '$(EMBED_DIR)'

.PHONY: build
build: spa ## SPA + static binary into bin/tanulom (CGO_ENABLED=0, one file, no runtime deps)
	$(call need,go,$(GO_HINT))
	@mkdir -p '$(CURDIR)/bin'
	cd '$(BACKEND_DIR)' && CGO_ENABLED=0 go build -trimpath \
		-ldflags='-s -w -X tanulom/internal/api.Version=$(VERSION)' \
		-o '$(CURDIR)/$(BIN)' ./cmd/tanulom
	@printf '\n  built %s (version %s)\n\n' '$(BIN)' '$(VERSION)'

.PHONY: run
run: build ## Build, then serve UI + API from the single binary against ./data
	@mkdir -p '$(DATA_DIR)'
	./$(BIN) --data-dir '$(DATA_DIR)'

##@ Quality

.PHONY: test
test: test-backend test-frontend ## Everything CI gates on, backend then frontend

.PHONY: test-backend
test-backend: ensure-spa ## go vet + go build + go test ./...
	$(call need,go,$(GO_HINT))
	cd '$(BACKEND_DIR)' && go vet ./...
	cd '$(BACKEND_DIR)' && go build ./...
	cd '$(BACKEND_DIR)' && go test ./... $(GO_TEST_FLAGS)

.PHONY: test-frontend
test-frontend: $(WEB_DIR)/node_modules ## type-check, lint and production build
	cd '$(WEB_DIR)' && npm run type-check
	cd '$(WEB_DIR)' && npm run lint
	cd '$(WEB_DIR)' && npm run build

.PHONY: lint
lint: $(WEB_DIR)/node_modules ensure-spa ## gofmt check + go vet + oxlint/eslint
	$(call need,go,$(GO_HINT))
	@cd '$(BACKEND_DIR)' && bad=$$(gofmt -l .) && { [ -z "$$bad" ] || { printf '\n  gofmt would rewrite:\n%s\n\n  Run: make fmt\n\n' "$$bad" >&2; exit 1; }; }
	cd '$(BACKEND_DIR)' && go vet ./...
	cd '$(WEB_DIR)' && npm run lint

.PHONY: fmt
fmt: $(WEB_DIR)/node_modules ## gofmt -w the backend, prettier the frontend
	$(call need,go,$(GO_HINT))
	cd '$(BACKEND_DIR)' && gofmt -w .
	cd '$(WEB_DIR)' && npm run format

##@ Language pack

.PHONY: langpack
langpack: ## Rebuild data/langpack-hu-v1.sqlite (hours, tens of GB of scratch space)
	$(call need,$(PYTHON),$(PY_HINT))
	@mkdir -p '$(DATA_DIR)'
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.build --out '$(PACK)'

.PHONY: starter
starter: ## Rebuild the starter pack + audio that ship INSIDE the binary
	$(call need,$(PYTHON),$(PY_HINT))
	@test -s '$(PACK)' || { echo 'need $(PACK) first — run: make langpack'; exit 1; }
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.starter \
		--full '$(PACK)' --out '$(STARTER_DIR)/langpack-hu-starter.sqlite'
	@# The ear trainer's clips ride along, so a fresh install has working audio
	@# with no download. Opus, so the whole inventory is well under a megabyte.
	@# COPYFILE_DISABLE and the exclude are both needed on macOS: without them
	@# bsdtar packs an AppleDouble `._x` sidecar next to every clip AND HIDES
	@# THEM FROM `tar tzf`, so the binary ships 385 junk files that only a
	@# non-Apple reader can see. Ask python3 -m tarfile -l if you ever doubt it.
	COPYFILE_DISABLE=1 tar czf '$(STARTER_DIR)/media.tar.gz' \
		--exclude '._*' --exclude '.DS_Store' -C '$(DATA_DIR)/media' hvpt

.PHONY: voices
voices: ## Fetch the three Piper voice models the ear trainer renders with (180 MB)
	$(call need,curl,Install curl)
	@mkdir -p '$(VOICES_DIR)'
# 60 MB each, redistributable, and identical on every machine — a download, not
# a repository. The .json voice cards ARE tracked, so what the ear trainer
# expects is recorded even before anyone runs this.
	@for v in anna berta imre; do \
		f='$(VOICES_DIR)'/hu_HU-$$v-medium.onnx; \
		if [ -s "$$f" ]; then printf '  have %s\n' "$$f"; continue; fi; \
		printf '  fetching hu_HU-%s-medium.onnx (60 MB)\n' "$$v"; \
		curl -fsSL --retry 3 -o "$$f.part" \
			"$(PIPER_BASE)/$$v/medium/hu_HU-$$v-medium.onnx" \
			&& mv "$$f.part" "$$f" \
			|| { rm -f "$$f.part"; echo "  failed: hu_HU-$$v-medium.onnx" >&2; exit 1; }; \
	done
	@printf '\n  voices in %s — now run: make audio\n\n' '$(VOICES_DIR)'

.PHONY: audio
audio: voices ## Render the ear-trainer clips into data/media (needs piper + ffmpeg)
	$(call need,$(PYTHON),$(PY_HINT))
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.minimal_pairs \
		--pack '$(PACK)' --media '$(DATA_DIR)/media' --voices-dir ./voices

##@ Housekeeping

.PHONY: deps
deps: ## Fetch Go modules, npm packages and the pipeline's Python requirements
	$(call need,go,$(GO_HINT))
	$(call need,npm,$(NODE_HINT))
	$(call need,$(PYTHON),$(PY_HINT))
	cd '$(BACKEND_DIR)' && go mod download
	cd '$(WEB_DIR)' && npm ci
	# Point PYTHON at a virtualenv to keep this out of the system interpreter:
	#   make deps PYTHON=langpack/.venv/bin/python
	'$(PYTHON)' -m pip install -r '$(LANGPACK_DIR)/requirements.txt'

.PHONY: clean
clean: ## Delete build output. Never touches ./data — that is the learner's database
	rm -rf '$(CURDIR)/bin' '$(WEB_DIR)/dist' '$(EMBED_DIR)' '$(WEB_DIR)/.eslintcache'
	rm -f '$(CURDIR)/tanulom'
# Put the placeholder straight back. Without it `go build`, `go vet` and
# `go test` all fail with "pattern all:dist: no matching files found", which
# reads like a broken checkout rather than the last command you ran. Cleaning
# build output must not leave the tree unable to build.
	@$(MAKE) --no-print-directory ensure-spa
	@printf '  node_modules and langpack/cache survive; remove them by hand if you mean it\n'

.PHONY: help
help: ## Show this help
	@printf '\n  tanulom — local development, no Docker required.\n\n  usage: make <target>\n'
	@awk 'BEGIN { FS = ":.*##" } \
		/^##@/ { printf "\n  \033[1m%s\033[0m\n", substr($$0, 5); next } \
		/^[a-zA-Z0-9_.\/-]+:.*##/ { printf "    \033[36m%-14s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@printf '\n  overrides: DATA_DIR PACK PYTHON VERSION DEV_ADDR GO_TEST_FLAGS\n\n'

# --- internal targets -------------------------------------------------------

# Vite, vue-tsc and eslint all live here; install on first use so a fresh clone
# can go straight to `make dev`. `make deps` does the lockfile-exact npm ci.
$(WEB_DIR)/node_modules:
	$(call need,node,$(NODE_HINT))
	$(call need,npm,$(NODE_HINT))
	cd '$(WEB_DIR)' && npm install

# //go:embed all:dist does not compile when the directory is absent, which is
# every fresh clone — CI stubs it for the same reason (.github/workflows/ci.yml).
# Only writes a placeholder when there is no SPA at all, so a real `make spa`
# build is never clobbered.
.PHONY: ensure-spa
ensure-spa:
	@[ -f '$(EMBED_DIR)/index.html' ] || { mkdir -p '$(EMBED_DIR)'; printf '<!doctype html><title>tanulom</title>\n' > '$(EMBED_DIR)/index.html'; printf '  wrote a placeholder SPA to %s (make spa builds the real one)\n' '$(EMBED_DIR)'; }
