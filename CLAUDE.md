# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**LabPeek-Go** is a self-hosted home-lab CMDB and network discovery tool. Single Go binary + SQLite database. No external dependencies (no Redis, no Postgres, no queues). The project name is configurable via the `settings` table (`app.name` key).

## Build & Run Commands

```bash
# Build
make build
# or
go build -o bin/labpeek ./cmd/labpeek

# Run server
./bin/labpeek server

# Run tests
go test ./...

# Run a single test file or package
go test ./internal/discovery/...
go test ./internal/reconcile/...

# Run a single test by name
go test -run TestManualNamePreservation ./internal/reconcile/...

# Apply migrations
./bin/labpeek migrate

# Dev script
./scripts/dev.sh

# Docker
docker compose up -d
docker compose logs -f
```

## Architecture

### Core Design Principle

**Discovery data and curated CMDB data are separate.** Discovery writes raw `discovery_observations`. The reconciliation engine matches observations to curated `assets`. Manual fields (display_name, asset_type, role, location, rack, owner, criticality, notes, tags) must **never** be overwritten by discovery. This is enforced via `manual_data_json` on the asset — any field present in that JSON blob is locked from automatic updates.

### Binary Modes

`cmd/labpeek/main.go` dispatches to subcommands: `server`, `shell`, `discover`, `export`, `migrate`, `runs`, `assets`, `services`, `backup`.

### Package Layout

- `internal/config/` — config loading from `./data/config.yaml` and env vars (`LABPEEK_*`)
- `internal/db/` — SQLite connection setup (using `modernc.org/sqlite` for CGO-free builds)
- `internal/migrations/` — embedded SQL migration files, applied in order on startup
- `internal/models/` — struct definitions for all DB entities; thin repository layer with parameterised queries only
- `internal/discovery/` — discovery runner (goroutine-based background job), writes `data/discovery/<run-id>.xml`
  - `plugins/nmap/` — nmap XML parser; treats nmap as optional external binary
  - `plugins/arp/`, `mdns/`, `ssdp/`, `docker/`, `snmp/` — stub plugins for future use
  - `reconcile/` — reconciliation engine (see below)
- `internal/web/` — HTTP server (chi router preferred); server-rendered HTML + HTMX
  - `handlers/` — one file per page/feature area
  - `templates/` — Go HTML templates; escape all output
  - `static/` — minimal JS/CSS, no build step required
- `internal/shell/` — application command parser for the web shell; only executes approved LabPeek commands, never arbitrary OS commands
- `internal/export/` — YAML and Markdown exporters
- `internal/audit/` — writes to `changes` table

### Reconciliation Engine (`internal/discovery/reconcile/`)

Matching priority (high to low): serial → Docker ID → SSH host key → MAC → hostname+MAC → hostname → TLS fingerprint → IP only.

Rules:
1. MAC match → update discovered fields + `last_seen_at`, never touch manual fields.
2. Hostname strong match (no conflicting MAC) → update existing asset.
3. IP-only match → update only if no conflicting MAC/hostname history.
4. Low confidence → create a `suggestions` row, do not auto-apply.
5. No match → create new asset with auto-generated display_name.

**Manual name preservation test** (must always pass): create asset → user renames it → rediscovery with same MAC → `display_name` unchanged, `discovered_name` updated, `last_seen_at` updated.

### Discovery Profiles

| Profile | nmap flags |
|---|---|
| quick | `-sn -oX -` |
| normal | `-sV --top-ports 100 -oX -` |
| deep | `-sV -O -p- --max-retries 3 -oX -` |
| slow-safe | `-sV --top-ports 100 --scan-delay 100ms --max-rate 50 -oX -` |
| service-refresh | targets known IPs only, no subnet sweep |

### Network Safety

- Block all non-RFC1918 targets by default; only allow public scans when `LABPEEK_ALLOW_PUBLIC_SCAN=true`
- Private ranges: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `127.0.0.0/8`
- Only one active discovery run at a time by default
- All targets validated before nmap is invoked

### Data Paths

```
./data/labpeek.db          # SQLite database (single backup target)
./data/config.yaml         # optional config file
./data/discovery/<run-id>.xml   # raw nmap XML
./data/exports/            # YAML/Markdown exports
```

### Web Shell Security

`internal/shell/` parses a whitelist of LabPeek commands (`discover`, `assets`, `asset show`, `asset rename`, `asset type`, `services`, `runs`, `run show`, `export`, `backup`, `settings`, `status`, `help`, `clear`). No exec of arbitrary OS commands. Validate discovery targets before accepting.

### Display Name Generation Priority

1. SNMP sysName → 2. mDNS hostname → 3. DNS hostname → 4. Docker service name → 5. vendor + last IP octet → 6. `host-<ip-with-dashes>`

Names: lowercase, CLI-safe, unique, editable.

## Model Escalation

Use **Opus** for: data model design changes, reconciliation algorithm changes, security-sensitive code (shell parser, auth), concurrency/job design, difficult test failures, large refactors.

Use **Sonnet** for: routine implementation, template work, export formatting, CLI wiring.

## SQLite Tuning

Applied in `internal/db/` at connection open time:

```sql
PRAGMA journal_mode=WAL;      -- concurrent reads during discovery writes
PRAGMA busy_timeout=5000;     -- avoid "database is locked" errors
PRAGMA cache_size=-65536;     -- ~64 MB page cache (configurable)
```

**Indexing:** index `assets` on `(status, asset_type, last_seen_at, primary_ip, primary_mac, display_name)`; index `services` on `(asset_id, ip_address, port, protocol)`; index `discovery_runs` on `(status, created_at)`; composite unique on `asset_identities(identity_type, identity_value)` and `services(ip_address, port, protocol, transport)`.

**Write batching:** wrap each nmap parse batch and each reconciliation phase in a single transaction. Do not commit per-row.

**Hot-path queries:** never `SELECT *` in dashboard stats, asset/service list views, shell output, or reconciliation lookups — fetch only the columns needed.

**Raw output:** store nmap XML as files under `data/discovery/`; never as blobs in SQLite. Only parsed facts go into DB tables.

**JSON fields:** acceptable for noisy discovered metadata (`discovered_data_json`, `manual_data_json`, `raw_json`). Do not over-normalise early.

**VACUUM:** only after explicit retention cleanup runs, not after every scan.

**Backup:** use SQLite online backup API for hot backups; stop-container + copy for cold backups. DB must be on SSD-backed storage on TrueNAS.

**Slow queries:** use `EXPLAIN QUERY PLAN` on asset/service list, search, and reconciliation queries whenever they become slow.

## Key Constraints

- `modernc.org/sqlite` only (CGO-free); never add mattn/go-sqlite3 without a documented reason
- No PostgreSQL, Redis, external queues, or cloud dependencies
- UI: server-rendered HTML + HTMX; no React or heavy SPA
- All SQL queries must use parameterised arguments
- Docker: `cap_add: [NET_RAW, NET_ADMIN]` required for nmap; `privileged: true` is not required and must not be the default
