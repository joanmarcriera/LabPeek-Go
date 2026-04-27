package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joanmarcriera/labpeek-go/internal/discovery"
	"github.com/joanmarcriera/labpeek-go/internal/models"
	"github.com/joanmarcriera/labpeek-go/internal/shell"
	"github.com/joanmarcriera/labpeek-go/internal/web/handlers"
)

//go:embed static
var staticFiles embed.FS

type Dependencies struct {
	AppName   string
	DataDir   string
	Assets    *models.AssetRepository
	Services  *models.ServiceRepository
	Runs      *models.DiscoveryRunRepository
	Settings  *models.SettingsRepository
	Discovery *discovery.Service
	Shell     *shell.Service
}

type server struct {
	deps Dependencies
}

type pageMeta struct {
	Title   string
	Active  string
	Network string
	Content string
}

func NewRouter(deps Dependencies) http.Handler {
	s := &server{deps: deps}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	sub, _ := fs.Sub(staticFiles, "static")
	r.Get("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(sub))).ServeHTTP)

	r.Get("/health", handlers.Health)
	r.Get("/", s.dashboard)
	r.Get("/assets", s.assetsPage)
	r.Get("/assets/{id}", s.assetDetail)
	r.Post("/assets/{id}", s.saveAsset)
	r.Get("/services", s.servicesPage)
	r.Get("/discovery", s.discoveryPage)
	r.Post("/discovery/run", s.startDiscovery)
	r.Get("/shell", s.shellPage)
	r.Post("/shell", s.runShellCommand)
	r.Post("/settings/network", s.saveNetworkSettings)

	return r
}

func (s *server) dashboard(w http.ResponseWriter, r *http.Request) {
	assets, _ := s.deps.Assets.List(r.Context())
	services, _ := s.deps.Services.List(r.Context())
	runs, _ := s.deps.Runs.ListRecent(r.Context(), 12)

	newCount := 0
	changedCount := 0
	offlineCount := 0
	inQueueCount := countRunsByStatus(runs, "queued") + countRunsByStatus(runs, "running")
	recentAssets := newestAssets(assets, 5)
	for _, asset := range assets {
		if isRecent(asset.CreatedAt, 24*time.Hour) || isRecent(asset.FirstSeenAt, 24*time.Hour) {
			newCount++
		}
		if !asset.UpdatedAt.IsZero() && asset.UpdatedAt.After(asset.CreatedAt.Add(time.Minute)) {
			changedCount++
		}
		if strings.EqualFold(asset.Status, "down") || strings.EqualFold(asset.Status, "offline") {
			offlineCount++
		}
	}

	serviceCounts := serviceCountByAsset(services)

	var body strings.Builder
	body.WriteString(`<section class="kpis">`)
	body.WriteString(kpiCard("Devices", fmt.Sprintf("%d", len(assets)), "neutral"))
	body.WriteString(kpiCard("New", fmt.Sprintf("%d", newCount), "good"))
	body.WriteString(kpiCard("Changed", fmt.Sprintf("%d", changedCount), "warn"))
	body.WriteString(kpiCard("Offline", fmt.Sprintf("%d", offlineCount), "bad"))
	body.WriteString(kpiCard("In queue", fmt.Sprintf("%d", inQueueCount), "neutral"))
	body.WriteString(`</section>`)

	body.WriteString(`<section class="grid two">`)
	body.WriteString(`<article class="panel"><div class="panel-head"><h2>New Devices</h2><span class="muted">Recent discoveries</span></div>`)
	if len(recentAssets) == 0 {
		body.WriteString(`<p class="muted">No devices discovered yet.</p>`)
	} else {
		for _, asset := range recentAssets {
			body.WriteString(assetRowCard(asset, serviceCounts[asset.ID]))
		}
	}
	body.WriteString(`</article>`)

	body.WriteString(`<article class="panel"><div class="panel-head"><h2>Changes Detected</h2><span class="muted">Recent run activity</span></div>`)
	if len(runs) == 0 {
		body.WriteString(`<p class="muted">No discovery activity yet.</p>`)
	} else {
		for _, run := range runs {
			body.WriteString(runTimelineItem(run))
		}
	}
	body.WriteString(`</article></section>`)

	renderPage(w, s.deps.AppName, pageMeta{
		Title:   "Dashboard",
		Active:  "home",
		Network: s.networkLabel(r.Context()),
		Content: body.String(),
	})
}

func (s *server) assetsPage(w http.ResponseWriter, r *http.Request) {
	assets, err := s.deps.Assets.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	serviceCounts := serviceCountByAssetMust(r.Context(), s.deps.Services)
	subnets := subnetCounts(assets)

	var body strings.Builder
	body.WriteString(`<section class="toolbar-row"><h1>Devices</h1><div class="toolbar-actions"><a class="button ghost" href="/services">Services</a><a class="button" href="/discovery">Scan All</a></div></section>`)
	body.WriteString(`<div class="pill-row">`)
	body.WriteString(fmt.Sprintf(`<span class="pill active">All (%d)</span>`, len(assets)))
	for _, subnet := range orderedSubnetKeys(subnets) {
		body.WriteString(fmt.Sprintf(`<span class="pill">%s (%d)</span>`, template.HTMLEscapeString(subnet), subnets[subnet]))
	}
	body.WriteString(`</div>`)

	body.WriteString(`<section class="panel"><table><thead><tr><th>IP</th><th>Host / Label</th><th>Vendor</th><th>Pipeline</th><th>Status</th></tr></thead><tbody>`)
	for _, asset := range assets {
		body.WriteString(`<tr>`)
		body.WriteString(fmt.Sprintf(`<td class="mono">%s</td>`, template.HTMLEscapeString(asset.PrimaryIP)))
		body.WriteString(fmt.Sprintf(`<td><a href="/assets/%s">%s</a><div class="muted">%s</div></td>`,
			template.HTMLEscapeString(asset.ID),
			template.HTMLEscapeString(asset.DisplayName),
			template.HTMLEscapeString(emptyFallback(asset.DiscoveredName, "pending...")),
		))
		body.WriteString(fmt.Sprintf(`<td>%s</td>`, template.HTMLEscapeString(emptyFallback(asset.MACVendor, "Unknown"))))
		body.WriteString(fmt.Sprintf(`<td>%s</td>`, phaseBar(asset, serviceCounts[asset.ID])))
		body.WriteString(fmt.Sprintf(`<td>%s %s</td>`, statusDot(asset.Status), assetBadges(asset)))
		body.WriteString(`</tr>`)
	}
	body.WriteString(`</tbody></table></section>`)

	renderPage(w, s.deps.AppName, pageMeta{
		Title:   "Devices",
		Active:  "devices",
		Network: s.networkLabel(r.Context()),
		Content: body.String(),
	})
}

func (s *server) assetDetail(w http.ResponseWriter, r *http.Request) {
	asset, err := s.deps.Assets.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	services, err := s.deps.Services.ListByAsset(r.Context(), asset.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var body strings.Builder
	body.WriteString(`<section class="toolbar-row"><div><a class="button ghost" href="/assets">Back</a></div><div class="toolbar-actions"><span class="button ghost disabled">Watch</span>`)
	body.WriteString(fmt.Sprintf(`<form method="post" action="/discovery/run" class="inline"><input type="hidden" name="profile" value="normal"><input type="hidden" name="target" value="%s"><button type="submit" class="button">Service Scan</button></form>`,
		template.HTMLEscapeString(asset.PrimaryIP)))
	body.WriteString(`</div></section>`)

	body.WriteString(`<section class="identity-card">`)
	body.WriteString(fmt.Sprintf(`<div class="identity-main"><h1>%s</h1><div class="badge-row">%s %s</div></div>`,
		template.HTMLEscapeString(emptyFallback(asset.PrimaryIP, asset.DisplayName)),
		statusBadge(asset.Status),
		assetBadges(*asset),
	))
	body.WriteString(fmt.Sprintf(`<p class="muted">MAC: %s · Vendor: %s · Hostname: %s</p>`,
		template.HTMLEscapeString(emptyFallback(asset.PrimaryMAC, "pending...")),
		template.HTMLEscapeString(emptyFallback(asset.MACVendor, "pending...")),
		template.HTMLEscapeString(emptyFallback(asset.DiscoveredName, "pending...")),
	))
	body.WriteString(phaseSection(*asset, len(services)))
	body.WriteString(`</section>`)

	body.WriteString(`<section class="grid two">`)
	body.WriteString(fmt.Sprintf(`<article class="panel"><h2>Identity & Notes</h2><form method="post" action="/assets/%s">`, template.HTMLEscapeString(asset.ID)))
	body.WriteString(`<label class="field">Label<input name="display_name" value="` + template.HTMLEscapeString(asset.DisplayName) + `"></label>`)
	body.WriteString(`<label class="field">Type<input name="asset_type" value="` + template.HTMLEscapeString(asset.AssetType) + `"></label>`)
	body.WriteString(`<label class="field">Notes<textarea name="notes" rows="5">` + template.HTMLEscapeString(asset.Notes) + `</textarea></label>`)
	body.WriteString(`<button type="submit" class="button">Save</button></form></article>`)

	body.WriteString(`<article class="panel"><h2>Open Ports</h2>`)
	body.WriteString(`<table><thead><tr><th>Port</th><th>Proto</th><th>Service</th><th>State</th><th>Version</th></tr></thead><tbody>`)
	for _, service := range services {
		version := strings.TrimSpace(strings.TrimSpace(service.Product + " " + service.Version))
		if version == "" {
			version = "pending..."
		}
		body.WriteString(fmt.Sprintf(`<tr><td>%d</td><td>%s</td><td>%s</td><td>open</td><td>%s</td></tr>`,
			service.Port,
			template.HTMLEscapeString(service.Protocol),
			template.HTMLEscapeString(service.DisplayName),
			template.HTMLEscapeString(version),
		))
	}
	if len(services) == 0 {
		body.WriteString(`<tr><td colspan="5" class="muted">No services discovered yet.</td></tr>`)
	}
	body.WriteString(`</tbody></table>`)
	body.WriteString(fmt.Sprintf(`<p class="queue-note">Discovery JSON summary</p><pre>%s</pre>`, template.HTMLEscapeString(emptyFallback(asset.DiscoveredDataJSON, "{}"))))
	body.WriteString(`</article></section>`)

	renderPage(w, s.deps.AppName, pageMeta{
		Title:   "Device Detail",
		Active:  "devices",
		Network: s.networkLabel(r.Context()),
		Content: body.String(),
	})
}

func (s *server) saveAsset(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.deps.Assets.UpdateBasicFields(r.Context(), models.AssetUpdate{
		ID:          chi.URLParam(r, "id"),
		DisplayName: r.FormValue("display_name"),
		AssetType:   r.FormValue("asset_type"),
		Notes:       r.FormValue("notes"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/assets/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}

func (s *server) servicesPage(w http.ResponseWriter, r *http.Request) {
	services, err := s.deps.Services.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	assets, _ := s.deps.Assets.List(r.Context())
	assetNames := map[string]string{}
	for _, asset := range assets {
		assetNames[asset.ID] = asset.DisplayName
	}

	var body strings.Builder
	body.WriteString(`<section class="toolbar-row"><h1>Services</h1><div class="toolbar-actions"><a class="button ghost" href="/assets">Devices</a><a class="button" href="/discovery">Scan Queue</a></div></section>`)
	body.WriteString(`<section class="panel"><table><thead><tr><th>Name</th><th>Asset</th><th>IP</th><th>Port</th><th>Protocol</th><th>Product / Version</th><th>Last Seen</th></tr></thead><tbody>`)
	for _, service := range services {
		body.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td class="mono">%s</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			template.HTMLEscapeString(service.DisplayName),
			template.HTMLEscapeString(emptyFallback(assetNames[service.AssetID], service.AssetID)),
			template.HTMLEscapeString(service.IPAddress),
			service.Port,
			template.HTMLEscapeString(service.Protocol),
			template.HTMLEscapeString(strings.TrimSpace(strings.TrimSpace(service.Product+" "+service.Version))),
			template.HTMLEscapeString(timeLabel(service.LastSeenAt)),
		))
	}
	body.WriteString(`</tbody></table></section>`)

	renderPage(w, s.deps.AppName, pageMeta{
		Title:   "Services",
		Active:  "devices",
		Network: s.networkLabel(r.Context()),
		Content: body.String(),
	})
}

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
	body.WriteString(`<section class="toolbar-row"><h1>Scan Queue</h1><div class="toolbar-actions"><span class="button ghost disabled">Schedule</span><a class="button" href="#run-form">+ Add IP</a></div></section>`)
	body.WriteString(`<section class="kpis">`)
	body.WriteString(kpiCard("Pending", fmt.Sprintf("%d", pending), "neutral"))
	body.WriteString(kpiCard("Running", fmt.Sprintf("%d", running), "good"))
	body.WriteString(kpiCard("Done", fmt.Sprintf("%d", done), "neutral"))
	body.WriteString(`</section>`)

	body.WriteString(`<section class="grid two">`)
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
	body.WriteString(`<button type="submit" class="button">Start discovery</button></form>`)
	if runningRun != nil {
		body.WriteString(`<div class="queue-card running">`)
		body.WriteString(fmt.Sprintf(`<strong>%s</strong><p>%s · %d hosts · %d services</p><p class="muted">%s</p>`,
			template.HTMLEscapeString(runningRun.Target),
			template.HTMLEscapeString(profileLabel(runningRun.Profile)),
			runningRun.HostsFound,
			runningRun.ServicesFound,
			template.HTMLEscapeString(emptyFallback(runningRun.Logs, "running…")),
		))
		body.WriteString(`</div>`)
	}
	body.WriteString(`</article>`)

	body.WriteString(`<article class="panel"><div class="panel-head"><h2>Recent Jobs</h2><span class="muted">Pending / running / completed</span></div>`)
	for _, run := range runs {
		body.WriteString(`<div class="queue-card">`)
		body.WriteString(fmt.Sprintf(`<div class="queue-line"><strong>%s</strong><span>%s</span></div>`,
			template.HTMLEscapeString(run.Target),
			statusBadge(run.Status),
		))
		body.WriteString(fmt.Sprintf(`<p>%s · hosts %d · services %d</p>`,
			template.HTMLEscapeString(profileLabel(run.Profile)),
			run.HostsFound,
			run.ServicesFound,
		))
		if run.Error != "" {
			body.WriteString(fmt.Sprintf(`<p class="bad">%s</p>`, template.HTMLEscapeString(run.Error)))
		} else if run.Logs != "" {
			body.WriteString(fmt.Sprintf(`<p class="muted">%s</p>`, template.HTMLEscapeString(run.Logs)))
		}
		body.WriteString(`</div>`)
	}
	body.WriteString(`<p class="queue-note">Auto-enrichment note: new hosts can be re-queued for deeper phases later. Vulnerability scans remain manual/scheduled work.</p>`)
	body.WriteString(`</article></section>`)

	renderPage(w, s.deps.AppName, pageMeta{
		Title:   "Scan Queue",
		Active:  "queue",
		Network: s.networkLabel(r.Context()),
		Content: body.String(),
	})
}

func (s *server) startDiscovery(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	run, err := s.deps.Discovery.Queue(r.Context(), r.FormValue("profile"), r.FormValue("target"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	runCopy := *run
	go func() {
		_, _ = s.deps.Discovery.ExecuteQueued(context.Background(), &runCopy)
	}()

	http.Redirect(w, r, "/discovery", http.StatusSeeOther)
}

func (s *server) shellPage(w http.ResponseWriter, r *http.Request) {
	s.renderShellPage(w, r.Context(), "", s.deps.Shell.HelpText())
}

func (s *server) runShellCommand(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	command := r.FormValue("command")
	output, err := s.deps.Shell.Execute(r.Context(), command)
	if err != nil {
		output = strings.TrimSpace(output + "\n" + err.Error())
	}
	s.renderShellPage(w, r.Context(), command, output)
}

func (s *server) renderShellPage(w http.ResponseWriter, ctx context.Context, command string, output string) {
	networkSettings := s.mustNetworkSettings(ctx)
	var body strings.Builder
	body.WriteString(`<section class="toolbar-row"><h1>Settings</h1><div class="toolbar-actions"><a class="button ghost" href="/services">Services</a><a class="button" href="/discovery">Scan</a></div></section>`)
	body.WriteString(`<section class="grid two">`)
	body.WriteString(`<article class="panel"><div class="panel-head"><h2>Application Shell</h2><span class="muted">Internal commands only</span></div>`)
	body.WriteString(`<form method="post" action="/shell" class="stack-form"><label class="field">Command<input name="command" value="` + template.HTMLEscapeString(command) + `" placeholder="discover quick 192.168.1.0/24"></label><button type="submit" class="button">Run</button></form>`)
	body.WriteString(fmt.Sprintf(`<pre>%s</pre>`, template.HTMLEscapeString(output)))
	body.WriteString(`</article>`)
	body.WriteString(`<article class="panel"><div class="panel-head"><h2>Quick Commands</h2><span class="muted">Supported shell commands</span></div>`)
	body.WriteString(fmt.Sprintf(`<pre>%s</pre>`, template.HTMLEscapeString(s.deps.Shell.HelpText())))
	body.WriteString(`</article></section>`)
	body.WriteString(`<section id="network-settings" class="panel">`)
	body.WriteString(`<div class="panel-head"><h2>Network Defaults</h2><span class="muted">Editable header label and discovery defaults</span></div>`)
	body.WriteString(`<form method="post" action="/settings/network" class="stack-form">`)
	body.WriteString(`<label class="field">Header network label<input name="network_label" value="` + template.HTMLEscapeString(networkSettings.Label) + `" placeholder="Lab network(default)"></label>`)
	body.WriteString(`<label class="field">Default discovery target<input name="default_target" value="` + template.HTMLEscapeString(networkSettings.DefaultTarget) + `" placeholder="192.168.1.0/24"></label>`)
	body.WriteString(`<p class="queue-note">This label appears in the top header. The default target pre-fills the discovery form on first load.</p>`)
	body.WriteString(`<button type="submit" class="button">Save network defaults</button></form></section>`)

	renderPage(w, s.deps.AppName, pageMeta{
		Title:   "Settings / Shell",
		Active:  "settings",
		Network: s.networkLabel(ctx),
		Content: body.String(),
	})
}

func (s *server) saveNetworkSettings(w http.ResponseWriter, r *http.Request) {
	if s.deps.Settings == nil {
		http.Error(w, "settings repository is not configured", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.deps.Settings.UpdateNetworkSettings(r.Context(), models.NetworkSettings{
		Label:         r.FormValue("network_label"),
		DefaultTarget: r.FormValue("default_target"),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/shell#network-settings", http.StatusSeeOther)
}

func renderPage(w http.ResponseWriter, appName string, meta pageMeta) {
	const favicon = `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='5' fill='%231a7a4a'/%3E%3Ctext x='50%25' y='22' font-family='monospace' font-size='13' font-weight='700' fill='%233ecf7a' text-anchor='middle'%3ELP%3C/text%3E%3C/svg%3E`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s · %s</title>
  <link rel="icon" type="image/svg+xml" href="%s">
  <link rel="stylesheet" href="/static/app.css">
</head>
<body>
  <div class="app">
    <header class="topbar">
      <span class="brand">%s<span class="brand-accent">-Go</span></span>
      <a class="network-pill" href="/shell#network-settings"><span class="dot-live"></span>%s ▾</a>
      <a class="button primary" href="/discovery">⟳ Scan</a>
    </header>
    %s
  </div>
  <nav class="tabbar">
    <div class="tabbar-inner">
      %s
      %s
      %s
      %s
    </div>
  </nav>
</body>
</html>`,
		template.HTMLEscapeString(meta.Title),
		template.HTMLEscapeString(appName),
		favicon,
		template.HTMLEscapeString(appName),
		template.HTMLEscapeString(meta.Network),
		meta.Content,
		tabLink("/", "⊞", "Home", meta.Active == "home"),
		tabLink("/assets", "⊡", "Devices", meta.Active == "devices"),
		tabLink("/discovery", "≡", "Queue", meta.Active == "queue"),
		tabLink("/shell", "⚙", "Settings", meta.Active == "settings"),
	)
}

func tabLink(href, icon, label string, active bool) string {
	cls := "tab"
	if active {
		cls += " active"
	}
	return fmt.Sprintf(
		`<a class="%s" href="%s"><span style="font-size:1.1rem">%s</span><span style="font-size:.72rem">%s</span><span class="tab-bar"></span></a>`,
		cls, href, icon, template.HTMLEscapeString(label),
	)
}

func kpiCard(label string, value string, tone string) string {
	return fmt.Sprintf(`<div class="kpi %s"><div class="value">%s</div><div class="muted">%s</div></div>`,
		tone,
		template.HTMLEscapeString(value),
		template.HTMLEscapeString(label),
	)
}

func assetRowCard(asset models.Asset, serviceCount int) string {
	return fmt.Sprintf(`<div class="asset-card"><div class="asset-line"><div><a href="/assets/%s"><strong>%s</strong></a><div class="muted">%s · %s</div></div><span class="muted">%s</span></div>%s</div>`,
		template.HTMLEscapeString(asset.ID),
		template.HTMLEscapeString(emptyFallback(asset.PrimaryIP, asset.DisplayName)),
		template.HTMLEscapeString(emptyFallback(asset.MACVendor, "Unknown vendor")),
		template.HTMLEscapeString(emptyFallback(asset.PrimaryMAC, "pending MAC")),
		template.HTMLEscapeString(timeLabel(nonZero(asset.FirstSeenAt, asset.CreatedAt))),
		phaseBar(asset, serviceCount),
	)
}

func runTimelineItem(run models.DiscoveryRun) string {
	tone := "warn"
	if run.Status == "completed" {
		tone = "good"
	} else if run.Status == "failed" {
		tone = "bad"
	}
	detail := run.Error
	if detail == "" {
		detail = emptyFallback(run.Logs, fmt.Sprintf("%d hosts, %d services", run.HostsFound, run.ServicesFound))
	}
	return fmt.Sprintf(`<div class="timeline-item"><div class="queue-line"><span class="badge %s">%s</span><span class="muted">%s</span></div><p><strong>%s</strong> · %s</p><p class="muted">%s</p></div>`,
		tone,
		template.HTMLEscapeString(strings.ToUpper(run.Status)),
		template.HTMLEscapeString(timeLabel(nonZero(run.CompletedAt, run.CreatedAt))),
		template.HTMLEscapeString(profileLabel(run.Profile)),
		template.HTMLEscapeString(run.Target),
		template.HTMLEscapeString(detail),
	)
}

func phaseBar(asset models.Asset, serviceCount int) string {
	segments := []struct {
		Label string
		Tone  string
	}{
		{Label: "Ping", Tone: "done"},
		{Label: "ARP", Tone: toneIf(asset.PrimaryMAC != "", "done", "active")},
		{Label: "Ports", Tone: toneIf(serviceCount > 0, "done", "active")},
		{Label: "OS", Tone: toneIf(asset.DiscoveredDataJSON != "" && strings.Contains(strings.ToLower(asset.DiscoveredDataJSON), "os"), "done", "")},
		{Label: "Svcs", Tone: toneIf(serviceCount > 0, "done", "active")},
		{Label: "Vuln", Tone: ""},
	}

	var out strings.Builder
	out.WriteString(`<div class="phase">`)
	for _, segment := range segments {
		className := segment.Tone
		if className != "" {
			className = " " + className
		}
		out.WriteString(fmt.Sprintf(`<span class="%s">%s</span>`, strings.TrimSpace(className), template.HTMLEscapeString(segment.Label)))
	}
	out.WriteString(`</div>`)
	return out.String()
}

func phaseSection(asset models.Asset, serviceCount int) string {
	return `<div class="panel-head"><h2>Enrichment Progress</h2><span class="muted">Phased discovery pipeline</span></div>` + phaseBar(asset, serviceCount)
}

func statusDot(status string) string {
	className := "status-dot"
	if strings.EqualFold(status, "down") || strings.EqualFold(status, "offline") {
		className += " bad"
	}
	return `<span class="` + className + `"></span>`
}

func statusBadge(status string) string {
	tone := "good"
	label := strings.ToUpper(emptyFallback(status, "up"))
	if strings.EqualFold(status, "queued") || strings.EqualFold(status, "running") {
		tone = "warn"
	}
	if strings.EqualFold(status, "failed") || strings.EqualFold(status, "offline") || strings.EqualFold(status, "down") {
		tone = "bad"
	}
	return fmt.Sprintf(`<span class="badge %s">%s</span>`, tone, template.HTMLEscapeString(label))
}

func assetBadges(asset models.Asset) string {
	var badges []string
	if isRecent(asset.CreatedAt, 24*time.Hour) || isRecent(asset.FirstSeenAt, 24*time.Hour) {
		badges = append(badges, `<span class="badge good">NEW</span>`)
	}
	if !asset.UpdatedAt.IsZero() && asset.UpdatedAt.After(asset.CreatedAt.Add(time.Minute)) {
		badges = append(badges, `<span class="badge warn">CHG</span>`)
	}
	return strings.Join(badges, " ")
}

func toneIf(condition bool, ifTrue string, ifFalse string) string {
	if condition {
		return ifTrue
	}
	return ifFalse
}

func emptyFallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func timeLabel(value time.Time) string {
	if value.IsZero() {
		return "pending..."
	}
	return relativeTime(value)
}

func relativeTime(value time.Time) string {
	if value.IsZero() {
		return "pending..."
	}
	delta := time.Since(value)
	if delta < time.Minute {
		return "just now"
	}
	if delta < time.Hour {
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	}
	if delta < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(delta.Hours()/24))
}

func newestAssets(assets []models.Asset, limit int) []models.Asset {
	cloned := append([]models.Asset(nil), assets...)
	sort.Slice(cloned, func(i, j int) bool {
		return nonZero(cloned[i].FirstSeenAt, cloned[i].CreatedAt).After(nonZero(cloned[j].FirstSeenAt, cloned[j].CreatedAt))
	})
	if len(cloned) > limit {
		cloned = cloned[:limit]
	}
	return cloned
}

func serviceCountByAsset(services []models.Service) map[string]int {
	counts := map[string]int{}
	for _, service := range services {
		counts[service.AssetID]++
	}
	return counts
}

func serviceCountByAssetMust(ctx context.Context, repository *models.ServiceRepository) map[string]int {
	services, err := repository.List(ctx)
	if err != nil {
		return map[string]int{}
	}
	return serviceCountByAsset(services)
}

func subnetCounts(assets []models.Asset) map[string]int {
	counts := map[string]int{}
	for _, asset := range assets {
		if asset.PrimaryIP == "" {
			continue
		}
		counts[subnetFromIP(asset.PrimaryIP)]++
	}
	return counts
}

func orderedSubnetKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func subnetFromIP(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) < 3 {
		return ip
	}
	return fmt.Sprintf("%s.%s.%s.x", parts[0], parts[1], parts[2])
}

func isRecent(value time.Time, window time.Duration) bool {
	if value.IsZero() {
		return false
	}
	return time.Since(value) <= window
}

func nonZero(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func countRunsByStatus(runs []models.DiscoveryRun, status string) int {
	total := 0
	for _, run := range runs {
		if strings.EqualFold(run.Status, status) {
			total++
		}
	}
	return total
}

func firstRunByStatus(runs []models.DiscoveryRun, status string) *models.DiscoveryRun {
	for _, run := range runs {
		if strings.EqualFold(run.Status, status) {
			runCopy := run
			return &runCopy
		}
	}
	return nil
}

func (s *server) networkLabel(ctx context.Context) string {
	if s.deps.Settings != nil {
		settings, err := s.deps.Settings.GetNetworkSettings(nonNilContext(ctx))
		if err == nil && strings.TrimSpace(settings.Label) != "" {
			return settings.Label
		}
	}
	return models.DefaultNetworkLabel
}

func (s *server) mustNetworkSettings(ctx context.Context) models.NetworkSettings {
	if s.deps.Settings == nil {
		return models.NetworkSettings{
			Label:         models.DefaultNetworkLabel,
			DefaultTarget: models.DefaultDiscoveryTarget,
		}
	}
	settings, err := s.deps.Settings.GetNetworkSettings(nonNilContext(ctx))
	if err != nil {
		return models.NetworkSettings{
			Label:         models.DefaultNetworkLabel,
			DefaultTarget: models.DefaultDiscoveryTarget,
		}
	}
	return settings
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type profileOption struct {
	Value       string
	Label       string
	Description string
}

func discoveryProfileOptions() []profileOption {
	return []profileOption{
		{Value: "quick", Label: "Network ping sweep (quick)", Description: "Checks which hosts respond on the network without probing services."},
		{Value: "normal", Label: "Service scan on running devices (normal)", Description: "Scans responsive devices for common open services and versions."},
		{Value: "slow-safe", Label: "Slow safe service scan", Description: "Like the normal scan, but throttled to be quieter on busy or fragile networks."},
		{Value: "deep", Label: "Deep full-port and OS scan", Description: "Scans all ports and attempts OS detection. This is the heaviest option."},
	}
}

func profileLabel(value string) string {
	for _, option := range discoveryProfileOptions() {
		if option.Value == value {
			return option.Label
		}
	}
	return value
}
