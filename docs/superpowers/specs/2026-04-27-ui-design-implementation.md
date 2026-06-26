# LabPeek-Go UI Design Implementation

**Date:** 2026-04-27  
**Source:** docs/Wireframes.html (IBM Plex Mono/Sans, dark-first design)

## Scope

Bring the existing server-rendered UI into fidelity with the wireframe design and ship a production-ready Docker image with TrueNAS SCALE deployment docs.

## Architecture

- No new dependencies. All changes are in `internal/web/router.go` and a new `internal/web/static/app.css`.
- CSS served as a static file via chi `r.Get("/static/*", ...)` — eliminates ~200 lines duplicated on every response.
- HTML generation stays inline (no template files); pattern is already established and tests pass.

## Components

### 1. Static file serving
- `internal/web/static/app.css` — extracted and updated CSS
- chi route: `r.Get("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))`
- Embed via `//go:embed static` in `router.go`

### 2. Pipeline bar (all pages)
- Current: rounded pill `<span>` with padding
- Target: 12px-wide flat colored blocks (6 segments), matching wireframe mini-bars
- CSS class `.phase-block` replaces `.phase span`

### 3. Dashboard
- Add time window query param `?w=12h` (default); tab strip renders 1h/4h/12h/24h/7d pills
- Alert banners: green banner for new devices count, amber banner for changed count (shown only if > 0)
- Activity timeline: replace "Changes Detected" panel with a flat timeline matching DashV2 wireframe (time · badge · IP · description)

### 4. Device list (`/assets`)
- Add status dot column (7px circle, green/red)
- Pipeline bar uses new flat block style
- Subnet filter pills styled to match wireframe tags

### 5. Discovery/Queue (`/discovery`)
- 3-col summary strip (Pending / Running / Done) matching QueueV2
- Running job card: target, type, host count, ETA placeholder, green highlight
- Pending jobs list with position numbers

### 6. Favicon
- Inline SVG favicon (data URI) in `<head>`: green "LP" monogram

### 7. Docker image
- Existing Dockerfile is correct (multi-stage, alpine, nmap included)
- Add `Makefile` targets: `docker-build` and `docker-push`
- Tag: `$(DOCKER_REPO)/labpeek:latest` (env var, default `labpeek/labpeek`)

### 8. README
- TrueNAS SCALE section: `docker run` one-liner, app template YAML snippet, capability notes (NET_RAW, NET_ADMIN), dataset setup

## Data Flow

No backend changes. All UI changes are presentation-only — same handler logic, same repositories.

## Error Handling

No new error paths. Static file 404s fall through to chi's default 404.

## Testing

- `go test ./...` must continue to pass
- Manual smoke: load /, /assets, /discovery, /shell in browser
