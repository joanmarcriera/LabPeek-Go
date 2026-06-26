package shell_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joanmarcriera/labpeek-go/internal/db"
	"github.com/joanmarcriera/labpeek-go/internal/discovery"
	"github.com/joanmarcriera/labpeek-go/internal/migrations"
	"github.com/joanmarcriera/labpeek-go/internal/models"
	"github.com/joanmarcriera/labpeek-go/internal/shell"
)

func TestExecuteHelpAndDiscoverCommands(t *testing.T) {
	t.Parallel()

	app := setupShellApp(t)
	output, err := app.shell.Execute(context.Background(), "help")
	if err != nil {
		t.Fatalf("help command: %v", err)
	}
	if !strings.Contains(output, "discover quick <target>") {
		t.Fatalf("help output missing discover command: %s", output)
	}

	output, err = app.shell.Execute(context.Background(), "discover quick 192.168.1.0/24")
	if err != nil {
		t.Fatalf("discover command: %v", err)
	}
	if !strings.Contains(strings.ToLower(output), "completed") {
		t.Fatalf("discover output missing completion status: %s", output)
	}

	runs, err := app.runs.ListRecent(context.Background(), 5)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].Status != "completed" {
		t.Fatalf("run status = %q, want %q", runs[0].Status, "completed")
	}
}

func TestExecuteExportCommands(t *testing.T) {
	t.Parallel()

	app := setupShellApp(t)
	if err := app.assets.Create(context.Background(), &models.Asset{
		ID:          "asset-1",
		DisplayName: "truenas-main",
		AssetType:   "nas",
		Status:      "active",
		PrimaryIP:   "192.168.1.50",
	}); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	yamlOutput, err := app.shell.Execute(context.Background(), "export yaml")
	if err != nil {
		t.Fatalf("export yaml: %v", err)
	}
	if !strings.Contains(yamlOutput, "inventory.yaml") {
		t.Fatalf("yaml export output missing file path: %s", yamlOutput)
	}

	markdownOutput, err := app.shell.Execute(context.Background(), "export markdown")
	if err != nil {
		t.Fatalf("export markdown: %v", err)
	}
	if !strings.Contains(markdownOutput, "inventory.md") {
		t.Fatalf("markdown export output missing file path: %s", markdownOutput)
	}

	for _, path := range []string{
		filepath.Join(app.dataDir, "exports", "inventory.yaml"),
		filepath.Join(app.dataDir, "exports", "inventory.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected export file %s: %v", path, err)
		}
	}
}

type shellApp struct {
	dataDir string
	assets  *models.AssetRepository
	runs    *models.DiscoveryRunRepository
	shell   *shell.Service
}

func setupShellApp(t *testing.T) shellApp {
	t.Helper()

	database := setupTestDB(t)
	dataDir := t.TempDir()

	assets := models.NewAssetRepository(database)
	services := models.NewServiceRepository(database)
	runs := models.NewDiscoveryRunRepository(database)
	discoveryService := discovery.NewService(
		dataDir,
		assets,
		services,
		runs,
		discovery.WithExecutor(stubExecutor{xml: []byte(sampleDiscoveryXML)}),
	)

	return shellApp{
		dataDir: dataDir,
		assets:  assets,
		runs:    runs,
		shell:   shell.NewService(dataDir, assets, services, runs, discoveryService),
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
