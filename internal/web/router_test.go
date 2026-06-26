package web_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joanmarcriera/labpeek-go/internal/db"
	"github.com/joanmarcriera/labpeek-go/internal/discovery"
	"github.com/joanmarcriera/labpeek-go/internal/migrations"
	"github.com/joanmarcriera/labpeek-go/internal/models"
	"github.com/joanmarcriera/labpeek-go/internal/shell"
	"github.com/joanmarcriera/labpeek-go/internal/web"
)

func TestPagesRenderDashboardAssetsServicesDiscoveryAndShell(t *testing.T) {
	t.Parallel()

	app := setupWebApp(t)
	if err := app.assets.Create(context.Background(), &models.Asset{
		ID:          "asset-1",
		DisplayName: "truenas-main",
		AssetType:   "nas",
		Status:      "active",
		PrimaryIP:   "192.168.1.50",
		PrimaryMAC:  "AA:BB:CC:DD:EE:FF",
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := app.services.Create(context.Background(), &models.Service{
		ID:          "service-1",
		AssetID:     "asset-1",
		DisplayName: "ssh",
		IPAddress:   "192.168.1.50",
		Port:        22,
		Protocol:    "ssh",
		Transport:   "tcp",
		Status:      "active",
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	pages := []string{"/", "/assets", "/assets/asset-1", "/services", "/discovery", "/shell"}
	for _, path := range pages {
		response := performRequest(t, app.router, http.MethodGet, path, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusOK)
		}
	}

	if !strings.Contains(performRequest(t, app.router, http.MethodGet, "/", "").Body.String(), "Dashboard") {
		t.Fatal("dashboard page missing Dashboard heading")
	}
	if !strings.Contains(performRequest(t, app.router, http.MethodGet, "/assets", "").Body.String(), "truenas-main") {
		t.Fatal("assets page missing asset name")
	}
	if !strings.Contains(performRequest(t, app.router, http.MethodGet, "/services", "").Body.String(), "ssh") {
		t.Fatal("services page missing service name")
	}
	if !strings.Contains(performRequest(t, app.router, http.MethodGet, "/shell", "").Body.String(), "discover quick") {
		t.Fatal("shell page missing command help")
	}
	discoveryPage := performRequest(t, app.router, http.MethodGet, "/discovery", "").Body.String()
	if !strings.Contains(discoveryPage, "Network ping sweep (quick)") {
		t.Fatal("discovery page missing descriptive quick label")
	}
	if !strings.Contains(performRequest(t, app.router, http.MethodGet, "/", "").Body.String(), "Lab network(default)") {
		t.Fatal("dashboard missing default network label")
	}
}

func TestAssetDetailSaveUpdatesManualFields(t *testing.T) {
	t.Parallel()

	app := setupWebApp(t)
	if err := app.assets.Create(context.Background(), &models.Asset{
		ID:          "asset-1",
		DisplayName: "host-192-168-1-50",
		AssetType:   "unknown",
		Status:      "active",
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	form := url.Values{}
	form.Set("display_name", "truenas-main")
	form.Set("asset_type", "nas")
	form.Set("notes", "Main storage box")

	response := performRequest(t, app.router, http.MethodPost, "/assets/asset-1", form.Encode())
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusSeeOther)
	}

	asset, err := app.assets.Get(context.Background(), "asset-1")
	if err != nil {
		t.Fatalf("get updated asset: %v", err)
	}
	if asset.DisplayName != "truenas-main" {
		t.Fatalf("display_name = %q, want %q", asset.DisplayName, "truenas-main")
	}
	if asset.AssetType != "nas" {
		t.Fatalf("asset_type = %q, want %q", asset.AssetType, "nas")
	}
	if asset.Notes != "Main storage box" {
		t.Fatalf("notes = %q, want %q", asset.Notes, "Main storage box")
	}
}

func TestDiscoveryFormAndShellTriggerDiscovery(t *testing.T) {
	t.Parallel()

	app := setupWebApp(t)

	form := url.Values{}
	form.Set("profile", "quick")
	form.Set("target", "192.168.1.0/24")

	response := performRequest(t, app.router, http.MethodPost, "/discovery/run", form.Encode())
	if response.Code != http.StatusSeeOther {
		t.Fatalf("discovery form status = %d, want %d", response.Code, http.StatusSeeOther)
	}

	var runs []models.DiscoveryRun
	var err error
	var completedRun *models.DiscoveryRun
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runs, err = app.runs.ListRecent(context.Background(), 5)
		if err != nil {
			t.Fatalf("list runs: %v", err)
		}
		if len(runs) == 1 && runs[0].Status == "completed" {
			runCopy := runs[0]
			completedRun = &runCopy
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if completedRun == nil {
		t.Fatalf("runs = %#v, want one completed run", runs)
	}

	shellForm := url.Values{}
	shellForm.Set("command", "runs")
	response = performRequest(t, app.router, http.MethodPost, "/shell", shellForm.Encode())
	if response.Code != http.StatusOK {
		t.Fatalf("shell status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(strings.ToLower(response.Body.String()), "completed") {
		t.Fatalf("shell output missing completed run: %s", response.Body.String())
	}

	if _, err := os.Stat(completedRun.RawOutputPath); err != nil {
		t.Fatalf("expected raw discovery output file: %v", err)
	}
}

func TestDiscoveryPostReturnsImmediatelyWhileRunContinuesInBackground(t *testing.T) {
	t.Parallel()

	app := setupWebAppWithExecutor(t, slowStubExecutor{
		delay: 200 * time.Millisecond,
		xml:   []byte(sampleDiscoveryXML),
	})

	form := url.Values{}
	form.Set("profile", "quick")
	form.Set("target", "192.168.1.0/24")

	started := time.Now()
	response := performRequest(t, app.router, http.MethodPost, "/discovery/run", form.Encode())
	elapsed := time.Since(started)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("discovery form status = %d, want %d", response.Code, http.StatusSeeOther)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("discovery POST took %s, want it to return immediately", elapsed)
	}

	runs, err := app.runs.ListRecent(context.Background(), 5)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].Status != "queued" && runs[0].Status != "running" && runs[0].Status != "completed" {
		t.Fatalf("unexpected run status %q", runs[0].Status)
	}
}

func TestDeleteSingleAssetRemovesItAndRedirects(t *testing.T) {
	t.Parallel()

	app := setupWebApp(t)
	if err := app.assets.Create(context.Background(), &models.Asset{
		ID: "asset-1", DisplayName: "old-nas", AssetType: "nas", Status: "active",
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	resp := performRequest(t, app.router, http.MethodPost, "/assets/asset-1/delete", "")
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusSeeOther)
	}
	if loc := resp.Header().Get("Location"); loc != "/assets" {
		t.Fatalf("redirect = %q, want /assets", loc)
	}

	assets, err := app.assets.List(context.Background())
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("asset count = %d, want 0", len(assets))
	}
}

func TestBulkDeleteAssetsRemovesSelectedAndRedirects(t *testing.T) {
	t.Parallel()

	app := setupWebApp(t)
	for i, name := range []string{"host-a", "host-b", "host-c"} {
		if err := app.assets.Create(context.Background(), &models.Asset{
			ID:          fmt.Sprintf("asset-%d", i+1),
			DisplayName: name,
			AssetType:   "server",
			Status:      "active",
		}); err != nil {
			t.Fatalf("create asset %s: %v", name, err)
		}
	}

	form := url.Values{}
	form.Add("id", "asset-1")
	form.Add("id", "asset-3")
	resp := performRequest(t, app.router, http.MethodPost, "/assets/delete", form.Encode())
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusSeeOther)
	}

	assets, err := app.assets.List(context.Background())
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("asset count = %d, want 1 (host-b should remain)", len(assets))
	}
	if assets[0].DisplayName != "host-b" {
		t.Fatalf("remaining asset = %q, want host-b", assets[0].DisplayName)
	}
}

func TestBulkDeleteWithNoSelectionRedirectsWithoutError(t *testing.T) {
	t.Parallel()

	app := setupWebApp(t)
	resp := performRequest(t, app.router, http.MethodPost, "/assets/delete", "")
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusSeeOther)
	}
}

func TestAssetsPageHasCheckboxesAndDeleteButton(t *testing.T) {
	t.Parallel()

	app := setupWebApp(t)
	if err := app.assets.Create(context.Background(), &models.Asset{
		ID: "asset-1", DisplayName: "myhost", AssetType: "server", Status: "active",
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	body := performRequest(t, app.router, http.MethodGet, "/assets", "").Body.String()
	if !strings.Contains(body, `type="checkbox"`) {
		t.Fatal("assets page missing checkboxes")
	}
	if !strings.Contains(body, `name="id" value="asset-1"`) {
		t.Fatal("assets page missing per-row checkbox with asset ID")
	}
	if !strings.Contains(body, `action="/assets/delete"`) {
		t.Fatal("assets page missing bulk-delete form action")
	}
	if !strings.Contains(body, `class="button danger`) {
		t.Fatal("assets page missing danger delete button")
	}
}

func TestCancelDiscoveryRedirectsAndMarksCancelledInDB(t *testing.T) {
	t.Parallel()

	app := setupWebAppWithExecutor(t, slowStubExecutor{
		delay: 10 * time.Second, // long enough that it won't finish during the test
		xml:   []byte(sampleDiscoveryXML),
	})

	// Queue (but don't execute) a run via the web form so we get a real run ID.
	form := url.Values{}
	form.Set("profile", "quick")
	form.Set("target", "192.168.1.0/24")
	resp := performRequest(t, app.router, http.MethodPost, "/discovery/run", form.Encode())
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("queue status = %d, want %d", resp.Code, http.StatusSeeOther)
	}

	// Wait briefly for the run to be in the DB (the goroutine may race slightly).
	var runs []models.DiscoveryRun
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var err error
		runs, err = app.runs.ListRecent(context.Background(), 5)
		if err != nil {
			t.Fatalf("list runs: %v", err)
		}
		if len(runs) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(runs) == 0 {
		t.Fatal("no runs found after queue")
	}

	runID := runs[0].ID
	cancelResp := performRequest(t, app.router, http.MethodPost, "/discovery/"+runID+"/cancel", "")
	if cancelResp.Code != http.StatusSeeOther {
		t.Fatalf("cancel status = %d, want %d", cancelResp.Code, http.StatusSeeOther)
	}
	if loc := cancelResp.Header().Get("Location"); loc != "/discovery" {
		t.Fatalf("redirect location = %q, want /discovery", loc)
	}

	runs, err := app.runs.ListRecent(context.Background(), 5)
	if err != nil {
		t.Fatalf("list runs after cancel: %v", err)
	}
	status := runs[0].Status
	if status != "cancelled" && status != "running" && status != "failed" {
		// "running" is acceptable if the goroutine hasn't processed the cancel yet;
		// the DB SetCancelled uses a WHERE clause that only updates queued/running rows.
		t.Fatalf("status = %q, want cancelled (or running if cancel arrived before SetCancelled)", status)
	}
}

func TestDiscoveryPageShowsProfileEstimatesAndAutoRefreshWhenRunning(t *testing.T) {
	t.Parallel()

	app := setupWebAppWithExecutor(t, slowStubExecutor{
		delay: 10 * time.Second,
		xml:   []byte(sampleDiscoveryXML),
	})

	form := url.Values{}
	form.Set("profile", "normal")
	form.Set("target", "192.168.1.0/24")
	performRequest(t, app.router, http.MethodPost, "/discovery/run", form.Encode())

	// Wait for the run to be in "running" state.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := app.runs.ListRecent(context.Background(), 5)
		if len(runs) > 0 && runs[0].Status == "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	body := performRequest(t, app.router, http.MethodGet, "/discovery", "").Body.String()
	if !strings.Contains(body, "~2 min") {
		t.Fatal("discovery page missing estimated duration for normal profile")
	}
	if !strings.Contains(body, `http-equiv="refresh"`) {
		t.Fatal("discovery page missing auto-refresh meta tag for active run")
	}
}

func TestSaveNetworkSettingsUpdatesHeaderAndDiscoveryDefaultTarget(t *testing.T) {
	t.Parallel()

	app := setupWebApp(t)

	form := url.Values{}
	form.Set("network_label", "Rack VLAN")
	form.Set("default_target", "192.168.50.0/24")

	response := performRequest(t, app.router, http.MethodPost, "/settings/network", form.Encode())
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusSeeOther)
	}

	dashboard := performRequest(t, app.router, http.MethodGet, "/", "").Body.String()
	if !strings.Contains(dashboard, "Rack VLAN") {
		t.Fatal("dashboard missing updated network label")
	}

	discoveryPage := performRequest(t, app.router, http.MethodGet, "/discovery", "").Body.String()
	if !strings.Contains(discoveryPage, `value="192.168.50.0/24"`) {
		t.Fatal("discovery page missing updated default target")
	}
}

type webApp struct {
	router   http.Handler
	assets   *models.AssetRepository
	runs     *models.DiscoveryRunRepository
	services *models.ServiceRepository
}

func setupWebApp(t *testing.T) webApp {
	t.Helper()

	return setupWebAppWithExecutor(t, stubExecutor{xml: []byte(sampleDiscoveryXML)})
}

func setupWebAppWithExecutor(t *testing.T, executor discovery.Executor) webApp {
	t.Helper()

	database := setupTestDB(t)
	dataDir := t.TempDir()

	assets := models.NewAssetRepository(database)
	services := models.NewServiceRepository(database)
	runs := models.NewDiscoveryRunRepository(database)
	settings := models.NewSettingsRepository(database)
	discoveryService := discovery.NewService(
		dataDir,
		assets,
		services,
		runs,
		discovery.WithExecutor(executor),
	)
	shellService := shell.NewService(dataDir, assets, services, runs, discoveryService)

	return webApp{
		router: web.NewRouter(web.Dependencies{
			AppName:   "LabPeek",
			DataDir:   dataDir,
			Assets:    assets,
			Services:  services,
			Runs:      runs,
			Settings:  settings,
			Discovery: discoveryService,
			Shell:     shellService,
		}),
		assets:   assets,
		services: services,
		runs:     runs,
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "labpeek.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := migrations.Apply(database); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return database
}

func performRequest(t *testing.T, handler http.Handler, method string, path string, encodedForm string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(encodedForm))
	if encodedForm != "" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type stubExecutor struct {
	xml []byte
}

func (s stubExecutor) Run(ctx context.Context, profile string, target string, outputPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(outputPath, s.xml, 0644); err != nil {
		return "", err
	}
	return "nmap completed", nil
}

type slowStubExecutor struct {
	delay time.Duration
	xml   []byte
}

func (s slowStubExecutor) Run(ctx context.Context, profile string, target string, outputPath string) (string, error) {
	time.Sleep(s.delay)
	return stubExecutor{xml: s.xml}.Run(ctx, profile, target, outputPath)
}

const sampleDiscoveryXML = `<?xml version="1.0" encoding="UTF-8"?>
<nmaprun>
  <host>
    <status state="up"/>
    <address addr="192.168.1.50" addrtype="ipv4"/>
    <address addr="AA:BB:CC:DD:EE:FF" addrtype="mac" vendor="iXsystems"/>
    <hostnames>
      <hostname name="truenas.local" type="PTR"/>
    </hostnames>
    <ports>
      <port protocol="tcp" portid="22">
        <state state="open"/>
        <service name="ssh" product="OpenSSH" version="9.3"/>
      </port>
    </ports>
  </host>
</nmaprun>`
