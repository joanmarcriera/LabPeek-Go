# LabPeek-Go MVP Plan

**Goal:** Single Go binary + SQLite CMDB with network discovery, web UI, and web shell.

---

## Phase 1 — Skeleton (THIS SESSION)
- [ ] go.mod + dependencies
- [ ] internal/config, internal/db, internal/migrations
- [ ] Schema: settings, assets, services, discovery_runs (+ indexes)
- [ ] `labpeek server` starts on :8080, /health returns 200
- [ ] Makefile, docker-compose.yml, .env.example, README quick-start

## Phase 2 — Models + Repositories
- [ ] models: Asset, Service, DiscoveryRun, Observation, Suggestion
- [ ] Thin repo layer (parameterised queries only)
- [ ] Tests: CRUD round-trips

## Phase 3 — Discovery Engine
- [ ] Network safety validator (RFC1918 only by default)
- [ ] nmap XML parser + testdata fixtures
- [ ] Reconciliation engine (MAC → hostname → IP priority; manual-field protection via manual_data_json)
- [ ] Background runner: goroutine, writes run status + raw XML
- [ ] Tests: manual name preservation

## Phase 4 — Web UI
- [ ] chi router, server-rendered HTML + HTMX
- [ ] Pages: dashboard, assets list, asset detail+edit, services, discovery, suggestions
- [ ] Web shell (whitelist-only command parser, no arbitrary exec)

## Phase 5 — CLI + Export + Ops
- [ ] cobra subcommands: server, discover, runs, assets, services, export, backup, migrate
- [ ] YAML + Markdown export
- [ ] Backup command (tar.gz of data/)
- [ ] Optional HTTP Basic Auth (LABPEEK_ADMIN_PASSWORD)

## Phase 6 — Docs + Polish
- [ ] README with screenshots
- [ ] docs/truenas.md
- [ ] go test ./... passes
- [ ] docker compose up -d works end-to-end
