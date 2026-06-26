package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/joanmarcriera/labpeek-go/internal/config"
	"github.com/joanmarcriera/labpeek-go/internal/db"
	"github.com/joanmarcriera/labpeek-go/internal/discovery"
	"github.com/joanmarcriera/labpeek-go/internal/export"
	"github.com/joanmarcriera/labpeek-go/internal/migrations"
	"github.com/joanmarcriera/labpeek-go/internal/models"
	"github.com/joanmarcriera/labpeek-go/internal/shell"
	"github.com/joanmarcriera/labpeek-go/internal/web"
	"github.com/spf13/cobra"
)

type appServices struct {
	cfg       *config.Config
	db        *sql.DB
	assets    *models.AssetRepository
	services  *models.ServiceRepository
	runs      *models.DiscoveryRunRepository
	settings  *models.SettingsRepository
	discovery *discovery.Service
	shell     *shell.Service
}

func main() {
	root := &cobra.Command{
		Use:   "labpeek",
		Short: "LabPeek — self-hosted home-lab CMDB and network discovery",
	}

	root.AddCommand(serverCmd())
	root.AddCommand(migrateCmd())
	root.AddCommand(discoverCmd())
	root.AddCommand(exportCmd())
	root.AddCommand(seedDemoCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start the web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			router := web.NewRouter(web.Dependencies{
				AppName:   app.cfg.AppName,
				DataDir:   app.cfg.DataDir,
				Assets:    app.assets,
				Services:  app.services,
				Runs:      app.runs,
				Settings:  app.settings,
				Discovery: app.discovery,
				Shell:     app.shell,
			})

			log.Printf("%s listening on %s (db: %s)", app.cfg.AppName, app.cfg.HTTPAddr, app.cfg.DBPath)
			return http.ListenAndServe(app.cfg.HTTPAddr, router)
		},
	}
}

func migrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Apply database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer database.Close()
			if err := migrations.Apply(database); err != nil {
				return err
			}
			log.Println("migrations applied")
			return nil
		},
	}
}

func discoverCmd() *cobra.Command {
	var profile string
	var target string

	command := &cobra.Command{
		Use:   "discover",
		Short: "Run discovery once",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			run, err := app.discovery.Run(context.Background(), profile, target)
			if err != nil {
				return err
			}
			fmt.Printf("run=%s status=%s hosts=%d services=%d raw=%s\n", run.ID, run.Status, run.HostsFound, run.ServicesFound, run.RawOutputPath)
			return nil
		},
	}

	command.Flags().StringVar(&profile, "profile", "quick", "Discovery profile")
	command.Flags().StringVar(&target, "target", "", "Target IP or CIDR")
	_ = command.MarkFlagRequired("target")
	return command
}

func exportCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "export",
		Short: "Export inventory",
	}

	command.AddCommand(exportFormatCmd("yaml"))
	command.AddCommand(exportFormatCmd("markdown"))
	return command
}

func exportFormatCmd(format string) *cobra.Command {
	var outputPath string

	command := &cobra.Command{
		Use:   format,
		Short: "Export " + format,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			snapshot, err := buildSnapshot(context.Background(), app.assets, app.services)
			if err != nil {
				return err
			}

			var data []byte
			switch format {
			case "yaml":
				data, err = export.MarshalYAML(snapshot)
			case "markdown":
				data, err = export.RenderMarkdown(snapshot)
			default:
				return fmt.Errorf("unsupported export format %q", format)
			}
			if err != nil {
				return err
			}

			if err := export.WriteFile(outputPath, data); err != nil {
				return err
			}
			fmt.Println(outputPath)
			return nil
		},
	}

	if format == "yaml" {
		outputPath = "inventory.yaml"
	} else {
		outputPath = "inventory.md"
	}
	command.Flags().StringVar(&outputPath, "output", outputPath, "Output file")
	return command
}

func seedDemoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed-demo",
		Short: "Seed demo assets and services for local testing/screenshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			return seedDemoData(context.Background(), app.assets, app.services)
		},
	}
}

func openApp() (*appServices, error) {
	cfg := config.Load()
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := migrations.Apply(database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	assets := models.NewAssetRepository(database)
	services := models.NewServiceRepository(database)
	runs := models.NewDiscoveryRunRepository(database)
	settings := models.NewSettingsRepository(database)
	discoveryService := discovery.NewService(
		cfg.DataDir,
		assets,
		services,
		runs,
		discovery.WithAllowPublic(cfg.AllowPublicScan),
	)
	shellService := shell.NewService(cfg.DataDir, assets, services, runs, discoveryService)

	return &appServices{
		cfg:       cfg,
		db:        database,
		assets:    assets,
		services:  services,
		runs:      runs,
		settings:  settings,
		discovery: discoveryService,
		shell:     shellService,
	}, nil
}

func (a *appServices) Close() error {
	return a.db.Close()
}

func buildSnapshot(ctx context.Context, assets *models.AssetRepository, services *models.ServiceRepository) (export.Snapshot, error) {
	assetList, err := assets.List(ctx)
	if err != nil {
		return export.Snapshot{}, err
	}
	serviceList, err := services.List(ctx)
	if err != nil {
		return export.Snapshot{}, err
	}
	return export.Snapshot{
		GeneratedAt: time.Now().UTC(),
		Assets:      assetList,
		Services:    serviceList,
		Metadata: map[string]string{
			"source": "cli",
		},
	}, nil
}

func seedDemoData(ctx context.Context, assets *models.AssetRepository, services *models.ServiceRepository) error {
	now := time.Now().UTC()
	seedAssets := []models.Asset{
		{ID: "asset-truenas", DisplayName: "truenas-main", AssetType: "nas", Status: "active", PrimaryIP: "192.168.1.50", PrimaryMAC: "AA:AA:AA:AA:AA:50", MACVendor: "iXsystems", CreatedAt: now, UpdatedAt: now},
		{ID: "asset-router", DisplayName: "router", AssetType: "router", Status: "active", PrimaryIP: "192.168.1.1", PrimaryMAC: "AA:AA:AA:AA:AA:01", MACVendor: "Ubiquiti", CreatedAt: now, UpdatedAt: now},
		{ID: "asset-docker", DisplayName: "docker-host-01", AssetType: "server", Status: "active", PrimaryIP: "192.168.1.60", PrimaryMAC: "AA:AA:AA:AA:AA:60", MACVendor: "Dell", CreatedAt: now, UpdatedAt: now},
	}
	for i := range seedAssets {
		existing, err := assets.Get(ctx, seedAssets[i].ID)
		if err == nil && existing != nil {
			continue
		}
		if err := assets.Create(ctx, &seedAssets[i]); err != nil {
			return err
		}
	}

	seedServices := []models.Service{
		{ID: uuid.NewString(), AssetID: "asset-truenas", DisplayName: "truenas-web", IPAddress: "192.168.1.50", Port: 443, Protocol: "https", Transport: "tcp", ServiceName: "https", Product: "nginx", Version: "1.24.0", Status: "active", FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.NewString(), AssetID: "asset-truenas", DisplayName: "ssh", IPAddress: "192.168.1.50", Port: 22, Protocol: "ssh", Transport: "tcp", ServiceName: "ssh", Product: "OpenSSH", Version: "9.3", Status: "active", FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.NewString(), AssetID: "asset-docker", DisplayName: "qbittorrent-web", IPAddress: "192.168.1.60", Port: 8080, Protocol: "http", Transport: "tcp", ServiceName: "http", Product: "qBittorrent", Version: "4.6", Status: "active", FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.NewString(), AssetID: "asset-docker", DisplayName: "ssh", IPAddress: "192.168.1.60", Port: 22, Protocol: "ssh", Transport: "tcp", ServiceName: "ssh", Product: "OpenSSH", Version: "9.3", Status: "active", FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now},
	}
	for i := range seedServices {
		if err := services.UpsertObserved(ctx, models.ObservedService{
			AssetID:     seedServices[i].AssetID,
			DisplayName: seedServices[i].DisplayName,
			IPAddress:   seedServices[i].IPAddress,
			Port:        seedServices[i].Port,
			Protocol:    seedServices[i].Protocol,
			Transport:   seedServices[i].Transport,
			ServiceName: seedServices[i].ServiceName,
			Product:     seedServices[i].Product,
			Version:     seedServices[i].Version,
			ObservedAt:  now,
		}); err != nil {
			return err
		}
	}

	fmt.Printf("seeded demo data in %s\n", filepath.Clean("."))
	return nil
}
