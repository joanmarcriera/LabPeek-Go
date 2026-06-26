package models_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/joanmarcriera/labpeek-go/internal/db"
	"github.com/joanmarcriera/labpeek-go/internal/migrations"
	"github.com/joanmarcriera/labpeek-go/internal/models"
)

func TestAssetRepositoryCreateListGetAndUpdateBasicFields(t *testing.T) {
	t.Parallel()

	repositories := setupRepositories(t)
	ctx := context.Background()

	asset := &models.Asset{
		ID:          "asset-1",
		DisplayName: "host-192-168-1-50",
		AssetType:   "unknown",
		Status:      "active",
		PrimaryIP:   "192.168.1.50",
		PrimaryMAC:  "AA:BB:CC:DD:EE:FF",
	}

	if err := repositories.assets.Create(ctx, asset); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	list, err := repositories.assets.List(ctx)
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("asset count = %d, want 1", len(list))
	}

	stored, err := repositories.assets.Get(ctx, asset.ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if stored.DisplayName != asset.DisplayName {
		t.Fatalf("display_name = %q, want %q", stored.DisplayName, asset.DisplayName)
	}

	if err := repositories.assets.UpdateBasicFields(ctx, models.AssetUpdate{
		ID:          asset.ID,
		DisplayName: "truenas-main",
		AssetType:   "nas",
		Notes:       "Manually curated",
	}); err != nil {
		t.Fatalf("update asset basics: %v", err)
	}

	updated, err := repositories.assets.Get(ctx, asset.ID)
	if err != nil {
		t.Fatalf("get updated asset: %v", err)
	}
	if updated.DisplayName != "truenas-main" {
		t.Fatalf("display_name = %q, want %q", updated.DisplayName, "truenas-main")
	}
	if updated.AssetType != "nas" {
		t.Fatalf("asset_type = %q, want %q", updated.AssetType, "nas")
	}
	if updated.Notes != "Manually curated" {
		t.Fatalf("notes = %q, want %q", updated.Notes, "Manually curated")
	}

	var manualData map[string]any
	if err := json.Unmarshal([]byte(updated.ManualDataJSON), &manualData); err != nil {
		t.Fatalf("parse manual_data_json: %v", err)
	}
	for _, field := range []string{"display_name", "asset_type", "notes"} {
		if _, ok := manualData[field]; !ok {
			t.Fatalf("manual_data_json missing %q override: %s", field, updated.ManualDataJSON)
		}
	}
}

func TestServiceRepositoryUpsertObservedAndListByAsset(t *testing.T) {
	t.Parallel()

	repositories := setupRepositories(t)
	ctx := context.Background()

	asset := &models.Asset{
		ID:          "asset-1",
		DisplayName: "appserver",
		AssetType:   "server",
		Status:      "active",
	}
	if err := repositories.assets.Create(ctx, asset); err != nil {
		t.Fatalf("create asset: %v", err)
	}

	firstSeen := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	if err := repositories.services.UpsertObserved(ctx, models.ObservedService{
		AssetID:     asset.ID,
		DisplayName: "ssh",
		IPAddress:   "192.168.1.60",
		Port:        22,
		Protocol:    "ssh",
		Transport:   "tcp",
		ServiceName: "ssh",
		Product:     "OpenSSH",
		Version:     "9.3",
		ObservedAt:  firstSeen,
	}); err != nil {
		t.Fatalf("upsert observed service: %v", err)
	}

	secondSeen := firstSeen.Add(2 * time.Hour)
	if err := repositories.services.UpsertObserved(ctx, models.ObservedService{
		AssetID:     asset.ID,
		DisplayName: "ssh",
		IPAddress:   "192.168.1.60",
		Port:        22,
		Protocol:    "ssh",
		Transport:   "tcp",
		ServiceName: "ssh",
		Product:     "OpenSSH",
		Version:     "9.4",
		ObservedAt:  secondSeen,
	}); err != nil {
		t.Fatalf("upsert observed service second pass: %v", err)
	}

	allServices, err := repositories.services.List(ctx)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(allServices) != 1 {
		t.Fatalf("service count = %d, want 1", len(allServices))
	}

	byAsset, err := repositories.services.ListByAsset(ctx, asset.ID)
	if err != nil {
		t.Fatalf("list services by asset: %v", err)
	}
	if len(byAsset) != 1 {
		t.Fatalf("services for asset = %d, want 1", len(byAsset))
	}
	if byAsset[0].Version != "9.4" {
		t.Fatalf("service version = %q, want %q", byAsset[0].Version, "9.4")
	}
	if byAsset[0].FirstSeenAt.UTC() != firstSeen {
		t.Fatalf("first_seen_at = %s, want %s", byAsset[0].FirstSeenAt.UTC(), firstSeen)
	}
	if byAsset[0].LastSeenAt.UTC() != secondSeen {
		t.Fatalf("last_seen_at = %s, want %s", byAsset[0].LastSeenAt.UTC(), secondSeen)
	}
}

func TestAssetRepositoryDeleteRemovesAssetAndItsServices(t *testing.T) {
	t.Parallel()

	repos := setupRepositories(t)
	ctx := context.Background()

	asset := &models.Asset{ID: "asset-1", DisplayName: "nas", AssetType: "nas", Status: "active"}
	if err := repos.assets.Create(ctx, asset); err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := repos.services.UpsertObserved(ctx, models.ObservedService{
		AssetID: "asset-1", DisplayName: "ssh", IPAddress: "192.168.1.1",
		Port: 22, Protocol: "ssh", Transport: "tcp", ObservedAt: time.Now(),
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}

	if err := repos.assets.Delete(ctx, "asset-1"); err != nil {
		t.Fatalf("delete asset: %v", err)
	}

	list, err := repos.assets.List(ctx)
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("asset count = %d, want 0", len(list))
	}

	services, err := repos.services.List(ctx)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("service count = %d, want 0 (services must be removed with asset)", len(services))
	}
}

func TestAssetRepositoryDeleteBatchRemovesMultipleAssetsAndTheirServices(t *testing.T) {
	t.Parallel()

	repos := setupRepositories(t)
	ctx := context.Background()

	for i, name := range []string{"host-a", "host-b", "host-c"} {
		asset := &models.Asset{
			ID: fmt.Sprintf("asset-%d", i+1), DisplayName: name, AssetType: "server", Status: "active",
		}
		if err := repos.assets.Create(ctx, asset); err != nil {
			t.Fatalf("create asset %s: %v", name, err)
		}
		if err := repos.services.UpsertObserved(ctx, models.ObservedService{
			AssetID: asset.ID, DisplayName: "ssh", IPAddress: fmt.Sprintf("192.168.1.%d", i+1),
			Port: 22, Protocol: "ssh", Transport: "tcp", ObservedAt: time.Now(),
		}); err != nil {
			t.Fatalf("create service for %s: %v", name, err)
		}
	}

	// Delete two of the three assets.
	if err := repos.assets.DeleteBatch(ctx, []string{"asset-1", "asset-3"}); err != nil {
		t.Fatalf("delete batch: %v", err)
	}

	list, err := repos.assets.List(ctx)
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("asset count = %d, want 1 (host-b should remain)", len(list))
	}
	if list[0].DisplayName != "host-b" {
		t.Fatalf("remaining asset = %q, want host-b", list[0].DisplayName)
	}

	services, err := repos.services.List(ctx)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("service count = %d, want 1", len(services))
	}
}

func TestAssetRepositoryDeleteBatchNoOpsOnEmptySlice(t *testing.T) {
	t.Parallel()

	repos := setupRepositories(t)
	if err := repos.assets.DeleteBatch(context.Background(), nil); err != nil {
		t.Fatalf("DeleteBatch(nil) = %v, want nil", err)
	}
	if err := repos.assets.DeleteBatch(context.Background(), []string{}); err != nil {
		t.Fatalf("DeleteBatch([]) = %v, want nil", err)
	}
}

func TestDiscoveryRunRepositoryCreateListRecentAndSetCancelled(t *testing.T) {
	t.Parallel()

	repos := setupRepositories(t)
	ctx := context.Background()

	run := &models.DiscoveryRun{
		Profile: "quick",
		Target:  "192.168.1.0/24",
	}
	if err := repos.runs.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.ID == "" {
		t.Fatal("run ID must be set after create")
	}
	if run.Status != "queued" {
		t.Fatalf("status = %q, want queued", run.Status)
	}

	runs, err := repos.runs.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].Profile != "quick" {
		t.Fatalf("profile = %q, want quick", runs[0].Profile)
	}

	latest, err := repos.runs.Latest(ctx)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest == nil || latest.ID != run.ID {
		t.Fatalf("latest run ID = %v, want %q", latest, run.ID)
	}

	if err := repos.runs.SetCancelled(ctx, run.ID); err != nil {
		t.Fatalf("set cancelled: %v", err)
	}

	runs, err = repos.runs.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("list recent after cancel: %v", err)
	}
	if runs[0].Status != "cancelled" {
		t.Fatalf("status after cancel = %q, want cancelled", runs[0].Status)
	}
	if runs[0].CompletedAt.IsZero() {
		t.Fatal("completed_at must be set after cancel")
	}
}

func TestDiscoveryRunRepositorySetCancelledIgnoresCompletedRuns(t *testing.T) {
	t.Parallel()

	repos := setupRepositories(t)
	ctx := context.Background()

	run := &models.DiscoveryRun{
		Profile: "quick",
		Target:  "192.168.1.1",
		Status:  "completed",
	}
	if err := repos.runs.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := repos.runs.SetCancelled(ctx, run.ID); err != nil {
		t.Fatalf("set cancelled: %v", err)
	}

	runs, err := repos.runs.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if runs[0].Status != "completed" {
		t.Fatalf("status = %q, want completed (cancel must not affect finished runs)", runs[0].Status)
	}
}

type repositorySet struct {
	assets   *models.AssetRepository
	services *models.ServiceRepository
	settings *models.SettingsRepository
	runs     *models.DiscoveryRunRepository
}

func setupRepositories(t *testing.T) repositorySet {
	t.Helper()

	database := setupTestDB(t)

	return repositorySet{
		assets:   models.NewAssetRepository(database),
		services: models.NewServiceRepository(database),
		settings: models.NewSettingsRepository(database),
		runs:     models.NewDiscoveryRunRepository(database),
	}
}

func TestSettingsRepositoryNetworkDefaultsAndUpdate(t *testing.T) {
	t.Parallel()

	repositories := setupRepositories(t)
	ctx := context.Background()

	settings, err := repositories.settings.GetNetworkSettings(ctx)
	if err != nil {
		t.Fatalf("get network settings: %v", err)
	}
	if settings.Label != models.DefaultNetworkLabel {
		t.Fatalf("label = %q, want %q", settings.Label, models.DefaultNetworkLabel)
	}
	if settings.DefaultTarget != models.DefaultDiscoveryTarget {
		t.Fatalf("default_target = %q, want %q", settings.DefaultTarget, models.DefaultDiscoveryTarget)
	}

	if err := repositories.settings.UpdateNetworkSettings(ctx, models.NetworkSettings{
		Label:         "Rack VLAN",
		DefaultTarget: "192.168.50.0/24",
	}); err != nil {
		t.Fatalf("update network settings: %v", err)
	}

	updated, err := repositories.settings.GetNetworkSettings(ctx)
	if err != nil {
		t.Fatalf("get updated network settings: %v", err)
	}
	if updated.Label != "Rack VLAN" {
		t.Fatalf("label = %q, want %q", updated.Label, "Rack VLAN")
	}
	if updated.DefaultTarget != "192.168.50.0/24" {
		t.Fatalf("default_target = %q, want %q", updated.DefaultTarget, "192.168.50.0/24")
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
