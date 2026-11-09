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
# ABSOLUTE, because the starter recipe runs from inside langpack/. As a
# relative path this resolved to langpack/backend/internal/lang/starter and
# `make starter` silently wrote the embedded pack into a shadow directory that
# nothing reads — so the binary went on shipping whatever was committed, which
# is the exact failure CLAUDE.md warns about. The media tarball on the next
# line is written from the repo root and was landing correctly, so the two
# halves of one target disagreed about where the pack lives.
STARTER_DIR  ?= $(CURDIR)/backend/internal/lang/starter
VOICES_DIR   := $(LANGPACK_DIR)/voices
MT_DIR        ?= $(CURDIR)/services/mt
# Absolute, and outside the repo: the weights are 600 MB of build output that a
# `git clean` has no business deciding about.
MT_MODEL_DIR  ?= $(HOME)/.cache/tanulom-mt
EMBED_DIR    := $(BACKEND_DIR)/internal/web/dist
BIN          := bin/tanulom

DATA_DIR      ?= $(CURDIR)/data
PACK          ?= $(DATA_DIR)/langpack-hu-v1.sqlite
# The pipeline's interpreter: the project virtualenv when `make venv` (part of
# `make deps`) has created it, else whatever python3 is on PATH. Piper lives in
# the venv, so a system python renders nothing — `make audio` once reported
# "rendered 0 clips" for exactly that reason.
VENV          := $(LANGPACK_DIR)/.venv
# ABSOLUTE: the pipeline recipes `cd langpack` first, so a relative venv path
# resolved against the wrong directory and `make refresh`/`make langpack` died
# with «No such file or directory» for anyone who had run `make deps`.
PYTHON        ?= $(if $(wildcard $(VENV)/bin/python),$(CURDIR)/$(VENV)/bin/python,python3)
# The ear trainer's three CC0 Hungarian talkers. Pinned to `main` because
# rhasspy/piper-voices publishes no tags; the .json voice cards beside each
# model are tracked, so a swapped model is visible in a diff.
PIPER_BASE    ?= https://huggingface.co/rhasspy/piper-voices/resolve/main/hu/hu_HU
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# The race detector needs a C toolchain (macOS: Xcode command-line tools).
# Without one: make test GO_TEST_FLAGS=-count=1
GO_TEST_FLAGS ?= -race -count=1

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

.PHONY: reset-password
reset-password: ensure-spa ## Forgot the password? Reset it in ./data (EMAIL=… PASSWORD=… optional; stop the server first)
	cd '$(BACKEND_DIR)' && go run ./cmd/tanulom reset-password --data-dir '$(abspath $(DATA_DIR))' $(if $(EMAIL),--email '$(EMAIL)') $(if $(PASSWORD),--password '$(PASSWORD)')

.PHONY: reset-learner
reset-learner: ## Start over: delete the learner database + config in ./data (keeps the language pack and audio). Asks first
	@printf '\n  This deletes ALL progress, accounts and settings in %s\n  (the language pack and media stay). Stop the server first. Type YES to go on: ' '$(abspath $(DATA_DIR))'; \
	read ans; [ "$$ans" = "YES" ] || { echo "  cancelled"; exit 1; }; \
	rm -f '$(DATA_DIR)'/tanulom.db '$(DATA_DIR)'/tanulom.db-wal '$(DATA_DIR)'/tanulom.db-shm '$(DATA_DIR)'/config.yaml; \
	printf '\n  done — the next start asks you to create a new account.\n\n'

##@ Quality

.PHONY: test
test: test-backend test-frontend docs-check ## Everything CI gates on: backend, frontend, docs

.PHONY: docs-check
docs-check: ## Research citations resolve, notes state their code impact, locales match
	@sh scripts/docs-check.sh

.PHONY: palette-check
palette-check: ## WCAG contrast of every design-token pair + illustration size limits
	@python3 scripts/validate_palette.py
	@python3 scripts/check_art.py

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
langpack: ## Rebuild data/langpack-hu-v1.sqlite (large source downloads and scratch space; duration not measured)
	$(call need,$(PYTHON),$(PY_HINT))
	@mkdir -p '$(DATA_DIR)'
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.build --out '$(PACK)'

# The REDISTRIBUTABLE pack: Tatoeba + Wiktionary only (--subtitle-sentences 0),
# because OPUS OpenSubtitles' licence does not cover publishing it
# (langpack/LICENSES.md). Written to dist/ - never over $(PACK) - and gzipped,
# which is the file a release carries and what Settings downloads. Put the
# printed sha256 in langpack.sha256 (config.DefaultLangPackSHA256) when the asset
# is published; the app refuses to download without it. Note the pipeline still
# FETCHES the subtitle archives (sources.fetch_all) even though none are used.
.PHONY: langpack-release
langpack-release: ## Build the publishable pack (no OpenSubtitles) to dist/langpack-hu-v1.sqlite.gz and print its sha256
	$(call need,$(PYTHON),$(PY_HINT))
	@mkdir -p '$(CURDIR)/dist'
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.build --out '$(CURDIR)/dist/langpack-hu-v1.sqlite' --subtitle-sentences 0
	gzip -9 -c '$(CURDIR)/dist/langpack-hu-v1.sqlite' > '$(CURDIR)/dist/langpack-hu-v1.sqlite.gz'
	@(sha256sum '$(CURDIR)/dist/langpack-hu-v1.sqlite.gz' 2>/dev/null || shasum -a 256 '$(CURDIR)/dist/langpack-hu-v1.sqlite.gz')

.PHONY: refresh-curriculum
.PHONY: refresh
refresh: refresh-curriculum ## Alias for refresh-curriculum

refresh-curriculum: ## Rewrite lessons/features/course in an existing pack (seconds, no downloads)
	$(call need,$(PYTHON),$(PY_HINT))
	@test -s '$(PACK)' || { echo 'need $(PACK) first — run: make langpack'; exit 1; }
# The lessons, the feature inventory, the minimal pairs and the course are pure
# Python literals: nothing about them needs Wiktionary or Tatoeba, so editing a
# lesson must not cost a multi-hour `make langpack`. Stop the server first — the
# app treats a pack as read-only and this is a build step writing to one.
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.refresh --pack '$(PACK)'

.PHONY: subtitles
subtitles: ## Add conversational sentences from OPUS to an existing pack (~40 min, no full rebuild)
	$(call need,$(PYTHON),$(PY_HINT))
	@test -s '$(PACK)' || { echo 'need $(PACK) first — run: make langpack'; exit 1; }
# The sentence stage never calls HuSpaCy — it links tokens through the pack's
# own wordform index — so adding sentences to a pack that already has a lexicon
# costs minutes of I/O, not a full `make langpack` rebuild. Stop the
# server first: the app treats a pack as read-only and this writes to one.
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.add_subtitles --pack '$(PACK)'

.PHONY: verbs
verbs: ## Backfill generated verb conjugations + corpus forms into an existing pack (seconds)
	$(call need,$(PYTHON),$(PY_HINT))
	@test -s '$(PACK)' || { echo 'need $(PACK) first — run: make langpack'; exit 1; }
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.verbs --pack '$(PACK)'

.PHONY: tutor-kb
tutor-kb: ## Regenerate the grammar tutor's reference corpus from its open sources (minutes)
	$(call need,$(PYTHON),$(PY_HINT))
# The tutor's `src-*.md` files (backend/internal/tutor/kb) are converted from
# Wikipedia, Wiktionary and the public-domain FSI course; the `ref-*.md` files
# beside them are written by hand and this never touches them. The pack is
# optional and only restores the diacritics the FSI scan lost — pass PACK= if
# yours lives elsewhere. Commit the result: the files are `//go:embed`ded.
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.tutor_kb \
		--out '$(CURDIR)/backend/internal/tutor/kb' --pack '$(PACK)'

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

# The managed-components manager embeds the sidecar sources (go:embed cannot reach
# outside its package), so services/<id>/ stays the source of truth and the copy
# under backend/internal/components/src/ is generated. A Go test fails on drift.
.PHONY: sync-components
sync-components: ## Copy services/{mt,nlp} sources into the Go package that embeds them (ADR-061)
	cp services/mt/app.py services/mt/requirements.txt services/mt/requirements-convert.txt backend/internal/components/src/mt/
	cp services/nlp/app.py services/nlp/requirements.txt backend/internal/components/src/nlp/

.PHONY: mt
mt: ## Run the local translator natively on :5003 (first run downloads ~2.4 GB)
	$(call need,$(PYTHON),$(PY_HINT))
# Its own venv, not the pipeline's: this needs ctranslate2 and transformers, and
# the first run additionally needs torch to convert the model. Putting that in
# the system interpreter is the kind of thing people only forgive once.
#
# MT_MODEL_DIR is ABSOLUTE and outside the repo. As a relative path it resolves
# against wherever the process started and 600 MB of weights land somewhere
# nobody expects — the same trap `make starter` documents two targets up.
	@if [ ! -x '$(MT_DIR)/.venv/bin/python' ]; then \
		printf '\n  creating %s/.venv — a few minutes, ~1 GB of wheels\n\n' '$(MT_DIR)'; \
		'$(PYTHON)' -m venv '$(MT_DIR)/.venv' || exit 1; \
		'$(MT_DIR)/.venv/bin/pip' install -q --upgrade pip || exit 1; \
		'$(MT_DIR)/.venv/bin/pip' install -q \
			-r '$(MT_DIR)/requirements.txt' \
			-r '$(MT_DIR)/requirements-convert.txt' || exit 1; \
	fi
	@printf '\n  translator  http://localhost:5003\n  model dir   %s\n' '$(MT_MODEL_DIR)'
	@printf '  Settings -> Services -> translation: provider local, endpoint http://localhost:5003\n\n'
	cd '$(MT_DIR)' && MT_MODEL_DIR='$(MT_MODEL_DIR)' ./.venv/bin/python app.py

.PHONY: audio
audio: voices ## Render the ear-trainer clips into data/media (needs piper + ffmpeg)
	$(call need,$(PYTHON),$(PY_HINT))
	$(call need,ffmpeg,Install ffmpeg (macOS: brew install ffmpeg) — the clips are encoded to Opus with it)
	@'$(PYTHON)' -c 'import piper' 2>/dev/null || { echo 'piper is not installed for $(PYTHON) — run: make deps'; exit 1; }
	cd '$(LANGPACK_DIR)' && '$(PYTHON)' -m pipeline.minimal_pairs \
		--pack '$(PACK)' --media '$(DATA_DIR)/media' --voices-dir ./voices

##@ Housekeeping

.PHONY: deps
deps: venv ## Fetch Go modules, npm packages and the pipeline's Python requirements (into langpack/.venv)
	$(call need,go,$(GO_HINT))
	$(call need,npm,$(NODE_HINT))
	cd '$(BACKEND_DIR)' && go mod download
	cd '$(WEB_DIR)' && npm ci

# The pipeline's own interpreter. Created once; `make deps` installs into it and
# every pipeline target finds it through PYTHON above. Nothing here touches the
# system Python. `make deps` after a requirements change re-installs.
.PHONY: venv
venv: $(VENV)/bin/python ## Create langpack/.venv and install the pipeline's requirements
	'$(VENV)/bin/python' -m pip install --quiet -r '$(LANGPACK_DIR)/requirements.txt'
$(VENV)/bin/python:
	$(call need,python3,$(PY_HINT))
	python3 -m venv '$(VENV)'

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
	@printf '\n  overrides: DATA_DIR PACK PYTHON VERSION DEV_ADDR GO_TEST_FLAGS MT_MODEL_DIR\n\n'

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
