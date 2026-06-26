package discovery_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joanmarcriera/labpeek-go/internal/db"
	"github.com/joanmarcriera/labpeek-go/internal/discovery"
	"github.com/joanmarcriera/labpeek-go/internal/discovery/plugins/nmap"
	"github.com/joanmarcriera/labpeek-go/internal/migrations"
	"github.com/joanmarcriera/labpeek-go/internal/models"
)

func TestValidateTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{name: "private cidr", target: "192.168.1.0/24"},
		{name: "private ip", target: "10.0.0.15"},
		{name: "loopback ip", target: "127.0.0.1"},
		{name: "public ip blocked", target: "8.8.8.8", wantErr: true},
		{name: "invalid target", target: "not-a-target", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := discovery.ValidateTarget(tt.target, false)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunFailsCleanlyWhenNmapIsMissing(t *testing.T) {
	t.Parallel()

	app := setupDiscoveryApp(t)
	service := discovery.NewService(
		app.dataDir,
		app.assets,
		app.services,
		app.runs,
		discovery.WithExecutor(stubExecutor{err: errors.New("nmap executable not found")}),
	)

	run, err := service.Run(context.Background(), "quick", "192.168.1.0/24")
	if err == nil {
		t.Fatal("expected discovery error, got nil")
	}
	if run.Status != "failed" {
		t.Fatalf("run status = %q, want %q", run.Status, "failed")
	}
	if !strings.Contains(strings.ToLower(run.Error), "nmap") {
		t.Fatalf("run error = %q, want nmap message", run.Error)
	}
}

func TestImportResultPreservesManualNameAndTypeOnMACMatch(t *testing.T) {
	t.Parallel()

	app := setupDiscoveryApp(t)

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	service := discovery.NewService(
		app.dataDir,
		app.assets,
		app.services,
		app.runs,
		discovery.WithNow(func() time.Time { return now }),
	)

	firstResult := &nmap.Result{
		Hosts: []nmap.Host{
			{
				IPAddresses: []string{"192.168.1.50"},
				MACAddress:  "AA:BB:CC:DD:EE:FF",
				Vendor:      "iXsystems",
				Hostnames:   []string{"host-192-168-1-50"},
				Ports: []nmap.Port{
					{Port: 22, Protocol: "tcp", ServiceName: "ssh", Product: "OpenSSH", Version: "9.3"},
				},
			},
		},
	}

	stats, err := service.ImportResult(context.Background(), firstResult)
	if err != nil {
		t.Fatalf("import first result: %v", err)
	}
	if stats.HostsFound != 1 || stats.ServicesFound != 1 {
		t.Fatalf("stats = %#v, want 1 host and 1 service", stats)
	}

	assets, err := app.assets.List(context.Background())
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("asset count = %d, want 1", len(assets))
	}

	asset := assets[0]
	if err := app.assets.UpdateBasicFields(context.Background(), models.AssetUpdate{
		ID:          asset.ID,
		DisplayName: "truenas-main",
		AssetType:   "nas",
	}); err != nil {
		t.Fatalf("manual asset update: %v", err)
	}

	secondNow := now.Add(30 * time.Minute)
	service = discovery.NewService(
		app.dataDir,
		app.assets,
		app.services,
		app.runs,
		discovery.WithNow(func() time.Time { return secondNow }),
	)

	secondResult := &nmap.Result{
		Hosts: []nmap.Host{
			{
				IPAddresses: []string{"192.168.1.50"},
				MACAddress:  "AA:BB:CC:DD:EE:FF",
				Vendor:      "iXsystems",
				Hostnames:   []string{"truenas-discovered"},
				Ports: []nmap.Port{
					{Port: 22, Protocol: "tcp", ServiceName: "ssh", Product: "OpenSSH", Version: "9.4"},
				},
			},
		},
	}

	stats, err = service.ImportResult(context.Background(), secondResult)
	if err != nil {
		t.Fatalf("import second result: %v", err)
	}
	if stats.HostsFound != 1 || stats.ServicesFound != 1 {
		t.Fatalf("stats = %#v, want 1 host and 1 service", stats)
	}

	updated, err := app.assets.Get(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("get updated asset: %v", err)
	}
	if updated.DisplayName != "truenas-main" {
		t.Fatalf("display_name = %q, want %q", updated.DisplayName, "truenas-main")
	}
	if updated.AssetType != "nas" {
		t.Fatalf("asset_type = %q, want %q", updated.AssetType, "nas")
	}
	if updated.DiscoveredName != "truenas-discovered" {
		t.Fatalf("discovered_name = %q, want %q", updated.DiscoveredName, "truenas-discovered")
	}
	if updated.LastSeenAt.UTC() != secondNow {
		t.Fatalf("last_seen_at = %s, want %s", updated.LastSeenAt.UTC(), secondNow)
	}

	services, err := app.services.ListByAsset(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("list services by asset: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("service count = %d, want 1", len(services))
	}
	if services[0].Version != "9.4" {
		t.Fatalf("service version = %q, want %q", services[0].Version, "9.4")
	}
}

func TestImportResultSkipsTCPRSTFalsePositives(t *testing.T) {
	t.Parallel()

	app := setupDiscoveryApp(t)
	service := discovery.NewService(app.dataDir, app.assets, app.services, app.runs)

	// Simulate what nmap produces when it falls back to TCP connect probes and
	// the router returns RST for non-existent IPs: all 256 hosts marked "up"
	// with reason="reset" and no MAC, hostname, or open ports.
	hosts := make([]nmap.Host, 0, 256)
	for i := 0; i < 254; i++ {
		hosts = append(hosts, nmap.Host{
			IPAddresses: []string{fmt.Sprintf("192.168.1.%d", i+1)},
			UpReason:    "reset",
		})
	}
	// Add two real devices among the noise: one with ARP (MAC), one with a port.
	hosts = append(hosts, nmap.Host{
		IPAddresses: []string{"192.168.1.10"},
		MACAddress:  "AA:BB:CC:DD:EE:FF",
		UpReason:    "arp-response",
	})
	hosts = append(hosts, nmap.Host{
		IPAddresses: []string{"192.168.1.20"},
		UpReason:    "syn-ack",
		Ports:       []nmap.Port{{Port: 80, Protocol: "tcp", ServiceName: "http"}},
	})

	result := &nmap.Result{Hosts: hosts}
	stats, err := service.ImportResult(context.Background(), result)
	if err != nil {
		t.Fatalf("ImportResult: %v", err)
	}
	if stats.HostsFound != 2 {
		t.Fatalf("HostsFound = %d, want 2 (only the two real devices)", stats.HostsFound)
	}

	assets, err := app.assets.List(context.Background())
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("asset count = %d, want 2; TCP-RST false positives must not create assets", len(assets))
	}
}

func TestQueueCreatesRunWithQueuedStatus(t *testing.T) {
	t.Parallel()

	app := setupDiscoveryApp(t)
	service := discovery.NewService(app.dataDir, app.assets, app.services, app.runs)

	run, err := service.Queue(context.Background(), "quick", "192.168.1.0/24")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if run.ID == "" {
		t.Fatal("run ID must be set")
	}
	if run.Status != "queued" {
		t.Fatalf("status = %q, want queued", run.Status)
	}
	if run.Profile != "quick" {
		t.Fatalf("profile = %q, want quick", run.Profile)
	}
	if run.Target != "192.168.1.0/24" {
		t.Fatalf("target = %q, want 192.168.1.0/24", run.Target)
	}

	runs, err := app.runs.ListRecent(context.Background(), 5)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
}

func TestQueueRejectsInvalidTarget(t *testing.T) {
	t.Parallel()

	app := setupDiscoveryApp(t)
	service := discovery.NewService(app.dataDir, app.assets, app.services, app.runs)

	if _, err := service.Queue(context.Background(), "quick", "8.8.8.8"); err == nil {
		t.Fatal("expected error for public IP, got nil")
	}
	if _, err := service.Queue(context.Background(), "quick", "not-an-ip"); err == nil {
		t.Fatal("expected error for invalid target, got nil")
	}

	runs, err := app.runs.ListRecent(context.Background(), 5)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("run count = %d, want 0 (no runs created for rejected targets)", len(runs))
	}
}

func TestCancelStopsRunningExecutorAndMarksCancelled(t *testing.T) {
	t.Parallel()

	app := setupDiscoveryApp(t)
	executorReady := make(chan struct{})
	executor := &blockingExecutor{ready: executorReady}

	service := discovery.NewService(app.dataDir, app.assets, app.services, app.runs,
		discovery.WithExecutor(executor),
	)

	run, err := service.Queue(context.Background(), "quick", "192.168.1.0/24")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := service.ExecuteQueued(context.Background(), run)
		done <- err
	}()

	// wait until the executor is actually running before cancelling
	select {
	case <-executorReady:
	case <-time.After(2 * time.Second):
		t.Fatal("executor did not start within 2s")
	}

	if err := service.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteQueued did not return after cancel")
	}

	runs, err := app.runs.ListRecent(context.Background(), 5)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("expected at least one run")
	}
	status := runs[0].Status
	if status != "cancelled" && status != "failed" {
		t.Fatalf("status = %q, want cancelled or failed", status)
	}
}

func TestCancelQueuedRunMarksCancelledWithoutExecuting(t *testing.T) {
	t.Parallel()

	app := setupDiscoveryApp(t)
	service := discovery.NewService(app.dataDir, app.assets, app.services, app.runs,
		discovery.WithExecutor(stubExecutor{err: errors.New("should not be called")}),
	)

	run, err := service.Queue(context.Background(), "quick", "192.168.1.0/24")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	if err := service.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	runs, err := app.runs.ListRecent(context.Background(), 5)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if runs[0].Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", runs[0].Status)
	}
}

// blockingExecutor blocks until its context is cancelled, simulating a long-running scan.
type blockingExecutor struct {
	ready chan struct{}
}

func (b *blockingExecutor) Run(ctx context.Context, profile, target, outputPath string) (string, error) {
	if b.ready != nil {
		select {
		case <-b.ready:
		default:
			close(b.ready)
		}
	}
	<-ctx.Done()
	return "", ctx.Err()
}

type discoveryApp struct {
	dataDir  string
	assets   *models.AssetRepository
	services *models.ServiceRepository
	runs     *models.DiscoveryRunRepository
}

func setupDiscoveryApp(t *testing.T) discoveryApp {
	t.Helper()

	database := setupTestDB(t)
	dataDir := t.TempDir()

	return discoveryApp{
		dataDir:  dataDir,
		assets:   models.NewAssetRepository(database),
		services: models.NewServiceRepository(database),
		runs:     models.NewDiscoveryRunRepository(database),
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

type stubExecutor struct {
	xml []byte
	err error
}

func (s stubExecutor) Run(ctx context.Context, profile string, target string, outputPath string) (string, error) {
	if s.err != nil {
		return "nmap failed", s.err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(outputPath, s.xml, 0644); err != nil {
		return "", err
	}
	return "nmap completed", nil
}
