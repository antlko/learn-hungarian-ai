# Development

A Go backend (`backend/`) serving a Vue 3 SPA (`web-ui/`). In production the built SPA is
embedded into the binary; in development the two run side by side with Vite proxying to Go.

## Prerequisites

- **Go** 1.26+
- **Node** 22+ (24 recommended — see `web-ui/package.json` `engines`)
- **Python** 3.11+ *only if* you rebuild the language pack
- **Docker** for image builds

## Two toolchains, two directories

Run `npm` from `web-ui/`. Run `go` from `backend/`. They are separate projects that ship as one
binary.

## Run both dev servers

**Terminal 1 — backend** (API on `:8080`, writes to `./data`):

```bash
cd backend && go run ./cmd/tanulom --data-dir ../data --addr :8080
```

**Terminal 2 — frontend** (Vite on `:5173`):

```bash
cd web-ui && npm install && npm run dev
```

Open **http://localhost:5173**. Vite proxies `/api` and `/media` to the backend (see
`web-ui/vite.config.ts`), so cookies and requests work end to end. Both sides hot-reload.

## Build the single binary

```bash
# 1. Build the SPA
cd web-ui && npm ci && npm run build && cd ..

# 2. Embed it
rm -rf backend/internal/web/dist && cp -r web-ui/dist backend/internal/web/dist

# 3. Static binary — pure Go, no CGO
cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../tanulom ./cmd/tanulom

# 4. Run it — UI + API on :8080
cd .. && ./tanulom --data-dir ./data
```

The `Dockerfile` automates exactly these steps across build stages.

## Quality gates

**Frontend** (from `web-ui/`):

```bash
npm run type-check   # vue-tsc, no emit
npm run lint         # oxlint → eslint, both --fix
npm run build        # type-check + production build
```

**Backend** (from `backend/`):

```bash
go vet ./...
go test ./...
go build ./...
```

Both run in `.github/workflows/ci.yml` on every PR.

## Language pack

You do **not** need to build the pack to develop. Download a release asset into `./data/`, or let
the setup wizard fetch it. To rebuild — see [`LANGPACK.md`](LANGPACK.md) — expect hours and tens
of GB of scratch space; it runs in CI on tag, not on a laptop.

## Optional sidecars

```bash
docker compose --profile nlp --profile speech up -d
```

The app probes them at startup and via `GET /api/capabilities`. **Develop with them stopped at
least some of the time** — the invariant that the core works alone is easy to break by accident
and hard to notice.

## Testing conventions

- Backend uses stdlib `testing`: table-driven, `httptest` for handlers, a temp-file SQLite per
  test. Add tests beside the package.
- **Golden tests** for morphology live in `internal/morph/testdata/` against a held-out UD
  Hungarian sample. These are the product's quality floor — if they regress, `n+1` and the
  heatmap are wrong.
- **Property test:** replaying a `review_log` through FSRS must reproduce stored card state
  exactly. If this fails, review history is not trustworthy.
- **No frontend test framework is wired.** Verify UI changes by running the dev servers. Do not
  invent an `npm test` command.

## Conventions that bite

- **Icons: `@lucide/vue`**, not `lucide-vue-next` (deprecated). Some names changed in v1
  (`Home`→`House`); verify with `type-check`.
- **Tailwind v4 resets buttons to `cursor: default`.** Reuse `buttonVariants`, `TabsTrigger`,
  `SelectTrigger`, `DropdownMenuItem` rather than hand-rolling a bare `<button>`.
- **shadcn-vue primitives in `src/components/ui` are single-word by design** — `eslint.config.ts`
  disables `vue/multi-word-component-names` for that directory only.
- **Migrations:** add `internal/db/migrations/NNNNN_name.sql` with `-- +goose Up` / `-- +goose
  Down`, dialect `sqlite3`. **Never edit an applied migration.** They are embedded and run inside
  `db.Open`; there is no migrate step.
- **`config.yaml` is mutated only through `Server.updateConfig`.** New `Config` fields must also
  be added to `config/clone.go`.
- **Frontend types mirror `dto.go`** (camelCase). Change both together.
- **Fiber handlers** return `fiber.NewError(code, msg)`; the central `errorHandler` renders
  `{"error": msg}` — the shape the frontend expects. Success via `c.JSON`.
- **Both locales, always.** A PR adding an `en` key without `ru` fails review.
- **Pedagogical code cites its note:**
  `// [R5] blocked-first ... see .claude/research/05-interleaving.md`
- **Behaviour change → doc change in the same commit.** `docs/` is the contract.
