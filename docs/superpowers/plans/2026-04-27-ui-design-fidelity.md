# LabPeek-Go UI Design Fidelity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the LabPeek-Go web UI into fidelity with the wireframe design (docs/Wireframes.html), ship a production-ready Docker image, and add TrueNAS SCALE deployment instructions.

**Architecture:** Extract the inline CSS from `renderPage` in `internal/web/router.go` into an embedded static file (`internal/web/static/app.css`), then update HTML generation functions in `router.go` to match the wireframe. No new dependencies, no template files.

**Tech Stack:** Go, `//go:embed`, chi, IBM Plex Mono/Sans (Google Fonts CDN)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/web/static/app.css` | CREATE | All UI styles (extracted + updated) |
| `internal/web/router.go` | MODIFY | Embed static dir, serve it, update HTML generation |
| `Makefile` | MODIFY | Add `docker-build` and `docker-push` targets |
| `README.md` | MODIFY | TrueNAS SCALE docker run + app template section |

---

### Task 1: Create static CSS file and wire up embedding

**Files:**
- Create: `internal/web/static/app.css`
- Modify: `internal/web/router.go` (top of file + `renderPage` function)

- [ ] **Step 1: Create `internal/web/static/app.css`** with the full extracted and updated stylesheet

```css
@import url('https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600;700&family=IBM+Plex+Sans:wght@400;500;600&display=swap');

:root {
  --bg:        #141414;
  --panel:     #1d1d1d;
  --panel-2:   #232323;
  --line:      #303030;
  --text:      #f3f3f3;
  --muted:     #a7a7a7;
  --xdim:      #555;
  --green:     #1a7a4a;
  --green-hi:  #3ecf7a;
  --green-bg:  #0d2e1a;
  --green-br:  #1a5c32;
  --amber:     #f0a030;
  --amber-bg:  #2a1e08;
  --amber-br:  #6a4010;
  --red:       #f05050;
  --red-bg:    #2a0e0e;
  --red-br:    #6a1c1c;
  --blue:      #5090e0;
  --blue-bg:   #0e1e3a;
  --blue-br:   #1a3a6a;
  --mono: 'IBM Plex Mono', ui-monospace, monospace;
  --sans: 'IBM Plex Sans', system-ui, sans-serif;
}

* { box-sizing: border-box; }

body {
  margin: 0;
  background: radial-gradient(circle at top right, rgba(26,122,74,0.12), transparent 28%), var(--bg);
  color: var(--text);
  font-family: var(--sans);
}

a { color: #a0c4ff; text-decoration: none; }

/* ── Layout ──────────────────────────────── */
.app { max-width: 1180px; margin: 0 auto; padding: 1rem 1rem 6rem; }

.topbar {
  display: flex; align-items: center; justify-content: space-between; gap: 1rem;
  padding: 1rem 0 1.5rem;
  position: sticky; top: 0;
  background: linear-gradient(180deg, rgba(20,20,20,.97), rgba(20,20,20,.84), rgba(20,20,20,0));
  backdrop-filter: blur(10px);
  z-index: 10;
}

.brand { font-family: var(--mono); font-weight: 700; font-size: 1.05rem; letter-spacing: -0.02em; }
.brand-accent { color: var(--green-hi); }

/* ── Network pill ────────────────────────── */
.network-pill {
  display: inline-flex; align-items: center; gap: .4rem;
  padding: .35rem .75rem;
  border: 1px solid var(--line); border-radius: 4px;
  background: rgba(255,255,255,.03);
  color: var(--text); font-family: var(--mono); font-size: .82rem; font-weight: 600;
  text-decoration: none; cursor: pointer;
}
.network-pill .dot-live { width: 7px; height: 7px; border-radius: 50%; background: var(--green-hi); flex-shrink: 0; }

/* ── Buttons ─────────────────────────────── */
.button, button {
  border: 1.5px solid var(--line);
  border-radius: 3px;
  padding: .4rem .85rem;
  background: var(--panel);
  color: var(--text);
  cursor: pointer;
  font-weight: 600;
  font-family: var(--mono);
  font-size: .82rem;
  white-space: nowrap;
  text-decoration: none;
  display: inline-flex; align-items: center;
}
.button.primary, button.primary {
  background: var(--text); color: var(--bg); border-color: var(--text);
}
.button.ghost, .ghost {
  background: transparent; border-color: var(--line); color: var(--muted);
}
.button.disabled, .disabled { opacity: .45; pointer-events: none; }
.inline { display: inline; }

/* ── KPI strip ───────────────────────────── */
.kpis { display: grid; gap: .6rem; grid-template-columns: repeat(auto-fit, minmax(110px, 1fr)); margin-bottom: 1rem; }
.kpi {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 4px;
  padding: .75rem .9rem;
  display: flex; flex-direction: column; align-items: center;
}
.kpi .value { font-family: var(--mono); font-size: 1.7rem; font-weight: 700; line-height: 1; margin-bottom: .25rem; }
.kpi .label { font-family: var(--mono); font-size: .7rem; color: var(--muted); text-transform: uppercase; letter-spacing: .05em; }
.kpi.good { border-color: var(--green-br); background: var(--green-bg); }
.kpi.good .value { color: var(--green-hi); }
.kpi.warn { border-color: var(--amber-br); background: var(--amber-bg); }
.kpi.warn .value { color: var(--amber); }
.kpi.bad  { border-color: var(--red-br);   background: var(--red-bg);   }
.kpi.bad  .value { color: var(--red); }
.kpi.info { border-color: var(--blue-br);  background: var(--blue-bg);  }
.kpi.info .value { color: var(--blue); }

/* ── Panels ──────────────────────────────── */
.grid.two { display: grid; gap: 1rem; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); }
.panel, .identity-card {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 4px;
  padding: 1rem;
}
.identity-card { margin-bottom: 1rem; }
.panel-head, .toolbar-row, .queue-line, .identity-main {
  display: flex; align-items: center; justify-content: space-between; gap: .75rem;
}
.toolbar-actions, .badge-row, .pill-row { display: flex; gap: .5rem; flex-wrap: wrap; align-items: center; }

/* ── Section header (wireframe SecHd) ────── */
.sec-hd {
  font-family: var(--mono); font-size: .65rem; font-weight: 700;
  color: var(--xdim); padding: 4px 12px 3px;
  letter-spacing: .08em; text-transform: uppercase;
  background: var(--panel-2); border-bottom: 1px solid var(--line);
  border-top: 1px solid var(--line);
}

/* ── Alert banners ───────────────────────── */
.alert-banner {
  display: flex; align-items: center; gap: .75rem;
  padding: 6px 14px;
  border-bottom: 1px solid;
  font-family: var(--mono); font-size: .82rem;
}
.alert-banner.green { background: var(--green-bg); border-color: var(--green-br); color: var(--green-hi); }
.alert-banner.amber { background: var(--amber-bg); border-color: var(--amber-br); color: var(--amber); }
.alert-banner .ab-label { font-weight: 700; font-size: .85rem; }
.alert-banner .ab-link { margin-left: auto; font-size: .78rem; opacity: .8; text-decoration: underline; cursor: pointer; }

/* ── Time window strip ───────────────────── */
.window-row {
  display: flex; align-items: center; gap: .4rem;
  padding: 5px 14px; border-bottom: 1px solid var(--line);
  background: var(--panel-2); flex-shrink: 0;
}
.window-row .win-label { font-family: var(--mono); font-size: .65rem; color: var(--xdim); margin-right: 4px; letter-spacing: .06em; }
.win-tag {
  font-family: var(--mono); font-size: .7rem;
  padding: 2px 8px; border-radius: 2px;
  border: 1px solid var(--line);
  background: transparent; color: var(--muted);
  cursor: pointer; white-space: nowrap;
  text-decoration: none;
}
.win-tag.active { background: var(--text); color: var(--bg); border-color: var(--text); }

/* ── Activity timeline ───────────────────── */
.tl-item {
  display: flex; align-items: center; gap: 8px;
  padding: 5px 14px;
  border-bottom: 1px solid var(--line);
}
.tl-time  { font-family: var(--mono); font-size: .72rem; color: var(--xdim); width: 48px; flex-shrink: 0; }
.tl-ip    { font-family: var(--mono); font-size: .72rem; color: var(--muted); white-space: nowrap; }
.tl-text  { font-family: var(--sans); font-size: .82rem; color: var(--text); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tl-ago   { font-family: var(--mono); font-size: .72rem; color: var(--xdim); white-space: nowrap; }
.tl-chev  { color: var(--xdim); }

/* ── Badges ──────────────────────────────── */
.badge {
  display: inline-flex; align-items: center;
  border-radius: 2px; padding: 1px 5px;
  font-family: var(--mono); font-size: .65rem; font-weight: 600;
  text-transform: uppercase; border: 1px solid transparent;
  white-space: nowrap;
}
.badge.good, .badge.green  { color: var(--green-hi); background: var(--green-bg); border-color: var(--green-br); }
.badge.warn, .badge.amber  { color: var(--amber);    background: var(--amber-bg); border-color: var(--amber-br); }
.badge.bad,  .badge.red    { color: var(--red);      background: var(--red-bg);   border-color: var(--red-br);   }
.badge.info, .badge.blue   { color: var(--blue);     background: var(--blue-bg);  border-color: var(--blue-br);  }
.badge.gray                { color: var(--muted);    background: rgba(255,255,255,.04); border-color: var(--line); }

/* ── Status dot ──────────────────────────── */
.status-dot {
  width: 7px; height: 7px; border-radius: 50%;
  display: inline-block; flex-shrink: 0;
  background: var(--green-hi);
}
.status-dot.bad   { background: var(--red); }
.status-dot.warn  { background: var(--amber); }
.status-dot.gray  { background: var(--xdim); }

/* ── Pipeline phase bar ──────────────────── */
.phase { display: flex; gap: 2px; margin-top: .45rem; }
.phase-seg {
  flex: 1; height: 5px; border-radius: 1px;
  background: #2b2b2b;
}
.phase-seg.done   { background: var(--green-hi); }
.phase-seg.active { background: var(--amber); }
.phase-labels {
  display: flex; gap: 2px; margin-top: 2px;
}
.phase-labels span {
  flex: 1; font-family: var(--mono); font-size: .55rem;
  color: var(--xdim); text-align: center;
}
.phase-labels span.done   { color: var(--green-hi); }
.phase-labels span.active { color: var(--amber); }

/* ── Tables ──────────────────────────────── */
table { width: 100%; border-collapse: collapse; margin-top: .75rem; }
th, td { padding: .6rem .4rem; border-bottom: 1px solid var(--line); text-align: left; vertical-align: middle; }
th { font-family: var(--mono); color: var(--muted); font-size: .7rem; font-weight: 700; letter-spacing: .05em; text-transform: uppercase; }
td { font-family: var(--mono); font-size: .82rem; }

/* ── Asset cards (dashboard) ─────────────── */
.asset-card, .timeline-item, .queue-card {
  border-top: 1px solid var(--line); padding: .85rem 0;
}
.asset-card:first-of-type, .timeline-item:first-of-type, .queue-card:first-of-type { border-top: 0; padding-top: 0; }
.asset-line { display: flex; justify-content: space-between; gap: .75rem; align-items: flex-start; }

/* ── Queue / running card ────────────────── */
.kpi-row-3 { display: grid; grid-template-columns: repeat(3, 1fr); border-bottom: 1.5px solid var(--line); margin-bottom: .75rem; }
.kpi-col {
  display: flex; flex-direction: column; align-items: center; padding: .55rem 0;
  border-right: 1px solid var(--line);
}
.kpi-col:last-child { border-right: 0; }
.kpi-col .n { font-family: var(--mono); font-size: 1.4rem; font-weight: 700; line-height: 1; }
.kpi-col .l { font-family: var(--mono); font-size: .65rem; color: var(--muted); text-transform: uppercase; letter-spacing: .05em; }
.kpi-col.running-col { background: var(--green-bg); }
.kpi-col.running-col .n { color: var(--green-hi); }

.running-card {
  margin: 0 0 .75rem;
  border: 1.5px solid var(--green-br);
  border-radius: 3px;
  padding: .75rem;
  background: var(--green-bg);
}
.running-card .rc-head { display: flex; align-items: center; gap: .6rem; margin-bottom: .4rem; }
.running-card .rc-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--green-hi); flex-shrink: 0; }
.running-card .rc-target { font-family: var(--mono); font-size: .92rem; font-weight: 700; color: var(--green-hi); flex: 1; }
.progress-bar { height: 4px; background: rgba(255,255,255,.1); border-radius: 2px; overflow: hidden; margin-bottom: .3rem; }
.progress-fill { height: 100%; background: var(--green-hi); }
.running-card .rc-meta { font-family: var(--mono); font-size: .72rem; color: var(--green-hi); opacity: .85; }

/* ── Form fields ─────────────────────────── */
.field { display: grid; gap: .3rem; margin-bottom: .85rem; color: var(--muted); font-size: .82rem; }
input, select, textarea {
  width: 100%; border: 1px solid var(--line); border-radius: 3px; padding: .6rem .8rem;
  background: rgba(255,255,255,.03); color: var(--text);
  font-family: var(--mono); font-size: .85rem;
}
.stack-form { display: flex; flex-direction: column; gap: 0; }
pre {
  background: rgba(255,255,255,.03); border: 1px solid var(--line);
  border-radius: 3px; padding: 1rem; overflow: auto; color: var(--text);
  font-family: var(--mono); font-size: .82rem;
}
.queue-note, .profile-help { margin-top: .75rem; color: var(--muted); font-size: .82rem; }
.profile-help p { margin: .3rem 0; }

/* ── Identity card ───────────────────────── */
.identity-card .id-meta { font-family: var(--mono); font-size: .78rem; color: var(--muted); line-height: 1.8; }

/* ── Misc ────────────────────────────────── */
.muted     { color: var(--muted); }
.mono      { font-family: var(--mono); }
.good      { color: var(--green-hi); }
.warn      { color: var(--amber); }
.bad       { color: var(--red); }
.pill      { display: inline-flex; align-items: center; gap: .35rem; padding: .3rem .7rem; border: 1px solid var(--line); border-radius: 3px; color: var(--muted); font-family: var(--mono); font-size: .75rem; }
.pill.active { color: var(--text); border-color: var(--green-br); background: rgba(26,122,74,.12); }

/* ── Tab bar (bottom nav) ────────────────── */
.tabbar {
  position: fixed; left: 0; right: 0; bottom: 0;
  display: flex; justify-content: center;
  background: rgba(20,20,20,.98); border-top: 1px solid var(--line);
  padding: .65rem 1rem calc(.65rem + env(safe-area-inset-bottom));
}
.tabbar-inner { width: min(1180px, 100%); display: grid; grid-template-columns: repeat(4, 1fr); gap: .5rem; }
.tab { text-align: center; padding: .5rem .3rem; border-radius: 3px; color: var(--muted); font-family: var(--mono); font-size: .8rem; text-decoration: none; }
.tab.active { background: rgba(62,207,122,.12); color: var(--text); }
.tab-active-bar { display: none; }
.tab.active .tab-active-bar { display: block; width: 20px; height: 2px; background: var(--green-hi); border-radius: 1px; margin: 2px auto 0; }

/* ── Responsive ──────────────────────────── */
@media (max-width: 720px) {
  .topbar, .toolbar-row, .panel-head, .asset-line, .identity-main { flex-direction: column; align-items: stretch; }
  .app { padding: 1rem .85rem 6.2rem; }
  .toolbar-actions { justify-content: flex-end; }
}
```

- [ ] **Step 2: Add embed directive and static serving to `router.go`**

At the top of `internal/web/router.go`, after the `package web` line, add:

```go
import "embed"

//go:embed static
var staticFiles embed.FS
```

Then in `NewRouter`, before the existing routes, add:

```go
r.Get("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))).ServeHTTP)
```

Wait — `http.FS(staticFiles)` with an embed FS rooted at the package will serve files at `static/app.css`. We need `http.StripPrefix` to strip `/static/` and serve from the embedded FS. But the FS has a `static/` directory inside it. The correct approach is:

```go
sub, _ := fs.Sub(staticFiles, "static")
r.Get("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(sub))).ServeHTTP)
```

Add `"io/fs"` to imports.

- [ ] **Step 3: Update `renderPage` to use stylesheet link + favicon**

Replace the inline `<style>...</style>` block in `renderPage` with:

```html
<link rel="stylesheet" href="/static/app.css">
<link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='6' fill='%231a7a4a'/%3E%3Ctext x='50%%' y='22' font-family='monospace' font-size='14' font-weight='700' fill='white' text-anchor='middle'%3ELP%3C/text%3E%3C/svg%3E">
```

Also update the brand HTML from:
```go
template.HTMLEscapeString(appName)
```
to produce:
```html
<span class="brand">LabPeek<span class="brand-accent">-Go</span></span>
```

And update the network pill from a plain anchor to include the live dot:
```html
<a class="network-pill" href="/shell#network-settings"><span class="dot-live"></span>%s ▾</a>
```

- [ ] **Step 4: Run tests to verify embed compiles**

```bash
go build ./... && go test ./...
```

Expected: all tests pass, binary builds.

- [ ] **Step 5: Commit**

```bash
git add internal/web/static/app.css internal/web/router.go
git commit -m "feat: extract CSS to static file, add embed serving and favicon"
```

---

### Task 2: Update pipeline bar to flat blocks

**Files:**
- Modify: `internal/web/router.go` — `phaseBar()` and `phaseSection()` functions

- [ ] **Step 1: Replace `phaseBar()` function**

Find and replace the entire `phaseBar` function (lines ~666–690 in current file):

```go
func phaseBar(asset models.Asset, serviceCount int) string {
	type seg struct{ label, tone string }
	segs := []seg{
		{"Ping", "done"},
		{"ARP",  toneIf(asset.PrimaryMAC != "", "done", "active")},
		{"Ports", toneIf(serviceCount > 0, "done", "active")},
		{"OS",   toneIf(asset.DiscoveredDataJSON != "" && strings.Contains(strings.ToLower(asset.DiscoveredDataJSON), "os"), "done", "")},
		{"Svcs", toneIf(serviceCount > 0, "done", "active")},
		{"Vuln", ""},
	}

	var out strings.Builder
	out.WriteString(`<div class="phase">`)
	for _, s := range segs {
		cls := "phase-seg"
		if s.tone != "" {
			cls += " " + s.tone
		}
		out.WriteString(fmt.Sprintf(`<div class="%s" title="%s"></div>`, cls, template.HTMLEscapeString(s.label)))
	}
	out.WriteString(`</div>`)
	out.WriteString(`<div class="phase-labels">`)
	for _, s := range segs {
		cls := s.tone
		out.WriteString(fmt.Sprintf(`<span class="%s">%s</span>`, cls, template.HTMLEscapeString(s.label)))
	}
	out.WriteString(`</div>`)
	return out.String()
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./...
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/web/router.go
git commit -m "feat: update pipeline bar to flat segment blocks with labels"
```

---

### Task 3: Dashboard — time window + alert banners + activity timeline

**Files:**
- Modify: `internal/web/router.go` — `dashboard()` function

- [ ] **Step 1: Add `timeWindow` helper that reads query param**

Add this helper function anywhere in `router.go`:

```go
var validWindows = []string{"1h", "4h", "12h", "24h", "7d"}

func parseWindow(r *http.Request) (string, time.Duration) {
	w := r.URL.Query().Get("w")
	switch w {
	case "1h":
		return "1h", 1 * time.Hour
	case "4h":
		return "4h", 4 * time.Hour
	case "12h":
		return "12h", 12 * time.Hour
	case "7d":
		return "7d", 7 * 24 * time.Hour
	default:
		return "24h", 24 * time.Hour
	}
}
```

- [ ] **Step 2: Rewrite the `dashboard()` handler**

Replace the existing `dashboard()` function body with:

```go
func (s *server) dashboard(w http.ResponseWriter, r *http.Request) {
	assets, _ := s.deps.Assets.List(r.Context())
	services, _ := s.deps.Services.List(r.Context())
	runs, _ := s.deps.Runs.ListRecent(r.Context(), 30)

	winKey, winDur := parseWindow(r)

	newCount := 0
	changedCount := 0
	offlineCount := 0
	inQueueCount := countRunsByStatus(runs, "queued") + countRunsByStatus(runs, "running")
	serviceCounts := serviceCountByAsset(services)

	var newAssets []models.Asset
	for _, a := range assets {
		if isRecent(nonZero(a.FirstSeenAt, a.CreatedAt), winDur) {
			newAssets = append(newAssets, a)
			newCount++
		}
		if !a.UpdatedAt.IsZero() && a.UpdatedAt.After(a.CreatedAt.Add(time.Minute)) && isRecent(a.UpdatedAt, winDur) {
			changedCount++
		}
		if strings.EqualFold(a.Status, "down") || strings.EqualFold(a.Status, "offline") {
			offlineCount++
		}
	}

	var body strings.Builder

	// KPI strip
	body.WriteString(`<section class="kpis">`)
	body.WriteString(kpiCard("Devices", fmt.Sprintf("%d", len(assets)), "neutral"))
	body.WriteString(kpiCard("New", fmt.Sprintf("%d", newCount), "good"))
	body.WriteString(kpiCard("Changed", fmt.Sprintf("%d", changedCount), "warn"))
	body.WriteString(kpiCard("Offline", fmt.Sprintf("%d", offlineCount), "bad"))
	body.WriteString(kpiCard("In queue", fmt.Sprintf("%d", inQueueCount), "info"))
	body.WriteString(`</section>`)

	// Time window strip
	body.WriteString(`<div class="window-row"><span class="win-label">WINDOW</span>`)
	for _, win := range validWindows {
		cls := "win-tag"
		if win == winKey {
			cls += " active"
		}
		body.WriteString(fmt.Sprintf(`<a class="%s" href="/?w=%s">%s</a>`, cls, win, win))
	}
	body.WriteString(`</div>`)

	// Alert banners
	if newCount > 0 {
		body.WriteString(fmt.Sprintf(
			`<div class="alert-banner green"><span class="ab-label">%d NEW DEVICE%s</span><span>in last %s</span><a class="ab-link" href="/assets">View ›</a></div>`,
			newCount, pluralS(newCount), winKey,
		))
	}
	if changedCount > 0 {
		body.WriteString(fmt.Sprintf(
			`<div class="alert-banner amber"><span class="ab-label">%d CHANGE%s</span><span>in last %s</span><a class="ab-link" href="/assets">View ›</a></div>`,
			changedCount, pluralS(changedCount), winKey,
		))
	}

	// New devices section
	body.WriteString(`<div class="sec-hd">▲ New devices — last ` + winKey + `</div>`)
	if len(newAssets) == 0 {
		body.WriteString(`<div class="tl-item" style="color:var(--muted);font-family:var(--mono);font-size:.8rem">No new devices in this window.</div>`)
	} else {
		for _, a := range newAssets {
			body.WriteString(assetRowCard(a, serviceCounts[a.ID]))
		}
	}

	// Activity timeline (recent runs)
	body.WriteString(`<div class="sec-hd">⚡ Activity — recent scans &amp; events</div>`)
	if len(runs) == 0 {
		body.WriteString(`<div class="tl-item" style="color:var(--muted);font-family:var(--mono);font-size:.8rem">No activity yet.</div>`)
	} else {
		for _, run := range runs {
			body.WriteString(runTimelineItem(run))
		}
	}

	renderPage(w, s.deps.AppName, pageMeta{
		Title:   "Dashboard",
		Active:  "home",
		Network: s.networkLabel(r.Context()),
		Content: body.String(),
	})
}
```

- [ ] **Step 3: Add `pluralS` helper**

```go
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "S"
}
```

- [ ] **Step 4: Update `runTimelineItem` to use timeline classes**

Replace the existing `runTimelineItem` function:

```go
func runTimelineItem(run models.DiscoveryRun) string {
	tone := "warn"
	badgeLabel := "CHG"
	if run.Status == "completed" {
		tone = "good"
		badgeLabel = "SCAN"
	} else if run.Status == "running" {
		tone = "info"
		badgeLabel = "RUN"
	} else if run.Status == "failed" {
		tone = "bad"
		badgeLabel = "ERR"
	}
	detail := run.Error
	if detail == "" {
		if run.HostsFound > 0 || run.ServicesFound > 0 {
			detail = fmt.Sprintf("%d hosts, %d services", run.HostsFound, run.ServicesFound)
		} else {
			detail = emptyFallback(run.Logs, profileLabel(run.Profile))
		}
	}
	return fmt.Sprintf(
		`<div class="tl-item"><span class="tl-time">%s</span><span class="badge %s">%s</span><span class="tl-ip">%s</span><span class="tl-text">%s</span><span class="tl-ago">%s</span></div>`,
		timeLabel(nonZero(run.CreatedAt)),
		tone, badgeLabel,
		template.HTMLEscapeString(run.Target),
		template.HTMLEscapeString(detail),
		template.HTMLEscapeString(timeLabel(nonZero(run.CompletedAt, run.CreatedAt))),
	)
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./...
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/web/router.go
git commit -m "feat: dashboard time window selector, alert banners, activity timeline"
```

---

### Task 4: Device list — status dots and tighter layout

**Files:**
- Modify: `internal/web/router.go` — `assetsPage()` function and `statusDot()` helper

- [ ] **Step 1: Update `statusDot()` to emit a flex-compatible span**

Replace the existing `statusDot` function:

```go
func statusDot(status string) string {
	cls := "status-dot"
	switch {
	case strings.EqualFold(status, "down"), strings.EqualFold(status, "offline"):
		cls += " bad"
	case strings.EqualFold(status, "queued"), strings.EqualFold(status, "running"):
		cls += " warn"
	}
	return `<span class="` + cls + `"></span>`
}
```

- [ ] **Step 2: Update table in `assetsPage()` to include status dot in first column**

In `assetsPage()`, find:
```go
body.WriteString(`<section class="panel"><table><thead><tr><th>IP</th><th>Host / Label</th><th>Vendor</th><th>Pipeline</th><th>Status</th></tr></thead><tbody>`)
for _, asset := range assets {
    body.WriteString(`<tr>`)
    body.WriteString(fmt.Sprintf(`<td class="mono">%s</td>`, template.HTMLEscapeString(asset.PrimaryIP)))
    body.WriteString(fmt.Sprintf(`<td><a href="/assets/%s">%s</a><div class="muted">%s</div></td>`,
        ...
    ))
    body.WriteString(fmt.Sprintf(`<td>%s</td>`, template.HTMLEscapeString(emptyFallback(asset.MACVendor, "Unknown"))))
    body.WriteString(fmt.Sprintf(`<td>%s</td>`, phaseBar(asset, serviceCounts[asset.ID])))
    body.WriteString(fmt.Sprintf(`<td>%s %s</td>`, statusDot(asset.Status), assetBadges(asset)))
    body.WriteString(`</tr>`)
}
```

Replace the table loop with:

```go
body.WriteString(`<section class="panel"><table><thead><tr><th></th><th>IP</th><th>Host / Label</th><th>Vendor</th><th>Pipeline</th><th>Badges</th></tr></thead><tbody>`)
for _, asset := range assets {
    body.WriteString(`<tr>`)
    body.WriteString(fmt.Sprintf(`<td style="width:18px;padding-right:0">%s</td>`, statusDot(asset.Status)))
    body.WriteString(fmt.Sprintf(`<td class="mono" style="font-weight:700">%s</td>`, template.HTMLEscapeString(asset.PrimaryIP)))
    body.WriteString(fmt.Sprintf(`<td><a href="/assets/%s">%s</a><div class="muted" style="font-size:.72rem">%s</div></td>`,
        template.HTMLEscapeString(asset.ID),
        template.HTMLEscapeString(asset.DisplayName),
        template.HTMLEscapeString(emptyFallback(asset.DiscoveredName, "pending...")),
    ))
    body.WriteString(fmt.Sprintf(`<td style="color:var(--muted)">%s</td>`, template.HTMLEscapeString(emptyFallback(asset.MACVendor, "Unknown"))))
    body.WriteString(fmt.Sprintf(`<td>%s</td>`, phaseBar(asset, serviceCounts[asset.ID])))
    body.WriteString(fmt.Sprintf(`<td>%s</td>`, assetBadges(asset)))
    body.WriteString(`</tr>`)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./...
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/web/router.go
git commit -m "feat: device list status dot column and tighter table layout"
```

---

### Task 5: Discovery/Queue — 3-col summary strip + running card

**Files:**
- Modify: `internal/web/router.go` — `discoveryPage()` function

- [ ] **Step 1: Rewrite `discoveryPage()` to add 3-col strip and running job card**

Replace the existing `discoveryPage()` function body with:

```go
func (s *server) discoveryPage(w http.ResponseWriter, r *http.Request) {
	runs, err := s.deps.Runs.ListRecent(r.Context(), 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pending := countRunsByStatus(runs, "queued")
	running := countRunsByStatus(runs, "running")
	done := countRunsByStatus(runs, "completed")
	runningRun := firstRunByStatus(runs, "running")
	networkSettings := s.mustNetworkSettings(r.Context())

	var body strings.Builder
	body.WriteString(`<section class="toolbar-row"><h1>Scan Queue</h1><div class="toolbar-actions"><span class="button ghost disabled">Schedule</span><a class="button primary" href="#run-form">+ Add IP</a></div></section>`)

	// 3-col KPI strip
	body.WriteString(`<div class="kpi-row-3">`)
	body.WriteString(fmt.Sprintf(`<div class="kpi-col"><span class="n warn">%d</span><span class="l">Pending</span></div>`, pending))
	runningCls := "kpi-col"
	if running > 0 {
		runningCls += " running-col"
	}
	body.WriteString(fmt.Sprintf(`<div class="%s"><span class="n good">%d</span><span class="l">Running</span></div>`, runningCls, running))
	body.WriteString(fmt.Sprintf(`<div class="kpi-col"><span class="n" style="color:var(--muted)">%d</span><span class="l">Done</span></div>`, done))
	body.WriteString(`</div>`)

	// Running job card
	if runningRun != nil {
		hostPct := 0
		if runningRun.HostsFound > 0 {
			hostPct = 63 // placeholder until real progress is tracked
		}
		body.WriteString(`<div class="running-card">`)
		body.WriteString(fmt.Sprintf(`<div class="rc-head"><div class="rc-dot"></div><span class="rc-target">%s</span><span class="badge info">%s</span></div>`,
			template.HTMLEscapeString(runningRun.Target),
			template.HTMLEscapeString(profileLabel(runningRun.Profile)),
		))
		body.WriteString(fmt.Sprintf(`<div class="progress-bar"><div class="progress-fill" style="width:%d%%"></div></div>`, hostPct))
		body.WriteString(fmt.Sprintf(`<div class="rc-meta">%d hosts found · %d services · started %s</div>`,
			runningRun.HostsFound, runningRun.ServicesFound,
			template.HTMLEscapeString(timeLabel(runningRun.CreatedAt)),
		))
		body.WriteString(`</div>`)
	}

	body.WriteString(`<section class="grid two">`)

	// Add job form
	body.WriteString(`<article class="panel">`)
	body.WriteString(`<div class="panel-head"><h2>Run Discovery</h2><span class="muted">Manual queue entry</span></div>`)
	body.WriteString(`<form id="run-form" method="post" action="/discovery/run" class="stack-form">`)
	body.WriteString(`<label class="field">Target<input name="target" value="` + template.HTMLEscapeString(networkSettings.DefaultTarget) + `"></label>`)
	body.WriteString(`<label class="field">Scan type<select name="profile">`)
	for _, option := range discoveryProfileOptions() {
		body.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`,
			template.HTMLEscapeString(option.Value),
			template.HTMLEscapeString(option.Label),
		))
	}
	body.WriteString(`</select></label>`)
	body.WriteString(`<div class="profile-help">`)
	for _, option := range discoveryProfileOptions() {
		body.WriteString(fmt.Sprintf(`<p><strong>%s:</strong> %s</p>`,
			template.HTMLEscapeString(option.Label),
			template.HTMLEscapeString(option.Description),
		))
	}
	body.WriteString(`</div>`)
	body.WriteString(`<p class="queue-note">Starting a discovery only queues the work. The browser returns immediately, and the run continues in the background while you keep using the UI.</p>`)
	body.WriteString(`<button type="submit" class="button primary">Start discovery</button></form>`)
	body.WriteString(`</article>`)

	// Recent jobs
	body.WriteString(`<article class="panel"><div class="panel-head"><h2>Recent Jobs</h2><span class="muted">Pending / running / completed</span></div>`)
	for _, run := range runs {
		tone := "warn"
		if run.Status == "completed" { tone = "good" }
		if run.Status == "failed" { tone = "bad" }
		if run.Status == "running" { tone = "info" }
		detail := run.Error
		if detail == "" { detail = emptyFallback(run.Logs, fmt.Sprintf("%d hosts, %d services", run.HostsFound, run.ServicesFound)) }
		body.WriteString(`<div class="queue-card">`)
		body.WriteString(fmt.Sprintf(`<div class="queue-line"><strong class="mono">%s</strong><span class="badge %s">%s</span></div>`,
			template.HTMLEscapeString(run.Target), tone, template.HTMLEscapeString(strings.ToUpper(run.Status)),
		))
		body.WriteString(fmt.Sprintf(`<p class="muted" style="font-size:.78rem;margin:.25rem 0 0">%s · %s</p>`,
			template.HTMLEscapeString(profileLabel(run.Profile)),
			template.HTMLEscapeString(timeLabel(nonZero(run.CompletedAt, run.CreatedAt))),
		))
		if detail != "" {
			body.WriteString(fmt.Sprintf(`<p class="muted" style="font-size:.78rem;margin:.1rem 0 0">%s</p>`, template.HTMLEscapeString(detail)))
		}
		body.WriteString(`</div>`)
	}
	body.WriteString(`<p class="queue-note" style="font-size:.75rem">Auto-enrichment: new hosts discovered by ping/ARP are automatically queued for OS → Service detection. Vulnerability scans are manual only.</p>`)
	body.WriteString(`</article></section>`)

	renderPage(w, s.deps.AppName, pageMeta{
		Title:   "Scan Queue",
		Active:  "queue",
		Network: s.networkLabel(r.Context()),
		Content: body.String(),
	})
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./...
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/web/router.go
git commit -m "feat: queue page 3-col summary strip and running job card"
```

---

### Task 6: Makefile docker targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Read current Makefile**

Current content:
```makefile
.PHONY: build run test

build:
	go build -o bin/labpeek ./cmd/labpeek

run: build
	./bin/labpeek server

test:
	go test ./...
```

- [ ] **Step 2: Add docker targets**

Replace `Makefile` with:

```makefile
DOCKER_REPO ?= labpeek/labpeek
IMAGE       := $(DOCKER_REPO):latest

.PHONY: build run test docker-build docker-push

build:
	go build -o bin/labpeek ./cmd/labpeek

run: build
	./bin/labpeek server

test:
	go test ./...

docker-build:
	docker build -t $(IMAGE) .

docker-push: docker-build
	docker push $(IMAGE)
```

- [ ] **Step 3: Verify Makefile syntax**

```bash
make -n docker-build
```

Expected: prints `docker build -t labpeek/labpeek:latest .`

- [ ] **Step 4: Build the Docker image**

```bash
docker build -t labpeek/labpeek:latest .
```

Expected: image built successfully, `labpeek/labpeek:latest` appears in `docker images`.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "feat: add docker-build and docker-push Makefile targets"
```

---

### Task 7: README — TrueNAS SCALE deployment section

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Replace the TrueNAS SCALE section**

Find the existing `## TrueNAS SCALE Deployment` section and replace it with:

```markdown
## TrueNAS SCALE Deployment

### Docker Hub

The image is published to Docker Hub:

```
docker pull labpeek/labpeek:latest
```

### Quick docker run

```bash
docker run -d \
  --name labpeek \
  --cap-add NET_RAW \
  --cap-add NET_ADMIN \
  -p 8088:8080 \
  -v /mnt/tank/labpeek/data:/app/data \
  --restart unless-stopped \
  labpeek/labpeek:latest
```

Open `http://<your-truenas-ip>:8088`.

### TrueNAS SCALE Custom App

In TrueNAS SCALE → Apps → Discover → Custom App, paste this YAML:

```yaml
version: "3"
services:
  labpeek:
    image: labpeek/labpeek:latest
    container_name: labpeek
    restart: unless-stopped
    cap_add:
      - NET_RAW
      - NET_ADMIN
    ports:
      - "8088:8080"
    volumes:
      - /mnt/tank/labpeek/data:/app/data
    environment:
      LABPEEK_ADDR: ":8080"
      LABPEEK_DB: "/app/data/labpeek.db"
      LABPEEK_DATA_DIR: "/app/data"
      LABPEEK_APP_NAME: "LabPeek"
```

Replace `/mnt/tank/labpeek/data` with the path to your TrueNAS dataset. Create the dataset before starting the app.

### Notes

- `NET_RAW` and `NET_ADMIN` are required for nmap to run ping sweeps inside the container.
- `privileged: true` is **not** required and should not be used.
- Use host networking (`network_mode: host`) if you want nmap to see all subnets on your TrueNAS network. Without it, discovery is limited to the Docker bridge network.
- Back up by snapshotting or copying the `/mnt/tank/labpeek/data` dataset.
- The SQLite database is at `/app/data/labpeek.db`. Do not mount the file directly — mount the directory.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: TrueNAS SCALE deployment with docker run and app template YAML"
```

---

### Task 8: Final build verification

- [ ] **Step 1: Run full test suite**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 2: Build binary**

```bash
go build -o bin/labpeek ./cmd/labpeek
```

Expected: `bin/labpeek` created with no errors.

- [ ] **Step 3: Build Docker image**

```bash
docker build -t labpeek/labpeek:latest .
```

Expected: image builds successfully.

- [ ] **Step 4: Smoke test**

```bash
docker run --rm -d --name labpeek-test -p 18088:8080 labpeek/labpeek:latest
sleep 3
curl -s http://localhost:18088/health
docker stop labpeek-test
```

Expected: `{"status":"ok"}` (or similar health response).

---

## Self-Review

**Spec coverage:**
- ✅ CSS extracted to static file with embed serving
- ✅ Pipeline bar → flat 5px blocks
- ✅ Dashboard: time window, alert banners, activity timeline
- ✅ Device list: status dot column
- ✅ Queue: 3-col summary + running job card with progress bar
- ✅ Favicon: inline SVG data URI in `<head>`
- ✅ Makefile docker targets
- ✅ README TrueNAS SCALE section with docker run + app template YAML

**Placeholder scan:**
- Task 5 running card uses `hostPct = 63` as a placeholder. This is intentional — real progress tracking is out of scope and noted inline.
- All other steps have complete code.

**Type consistency:**
- `parseWindow` returns `(string, time.Duration)` — used correctly in `dashboard()`.
- `pluralS` returns `string` — used correctly in alert banners.
- `phaseBar` signature unchanged — all callers unaffected.
- `statusDot` signature unchanged — all callers unaffected.
