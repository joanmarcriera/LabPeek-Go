package shell

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/joanmarcriera/labpeek-go/internal/discovery"
	"github.com/joanmarcriera/labpeek-go/internal/export"
	"github.com/joanmarcriera/labpeek-go/internal/models"
)

type Service struct {
	dataDir   string
	assets    *models.AssetRepository
	services  *models.ServiceRepository
	runs      *models.DiscoveryRunRepository
	discovery *discovery.Service
}

func NewService(
	dataDir string,
	assets *models.AssetRepository,
	services *models.ServiceRepository,
	runs *models.DiscoveryRunRepository,
	discoveryService *discovery.Service,
) *Service {
	return &Service{
		dataDir:   dataDir,
		assets:    assets,
		services:  services,
		runs:      runs,
		discovery: discoveryService,
	}
}

func (s *Service) HelpText() string {
	return strings.Join([]string{
		"help",
		"status",
		"discover quick <target>",
		"discover normal <target>",
		"runs",
		"assets",
		"services",
		"export yaml",
		"export markdown",
	}, "\n")
}

func (s *Service) Execute(ctx context.Context, command string) (string, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return s.HelpText(), nil
	}

	switch fields[0] {
	case "help":
		return s.HelpText(), nil
	case "status":
		return s.status(ctx)
	case "discover":
		return s.discover(ctx, fields)
	case "runs":
		return s.runsOutput(ctx)
	case "assets":
		return s.assetsOutput(ctx)
	case "services":
		return s.servicesOutput(ctx)
	case "export":
		return s.export(ctx, fields)
	default:
		return "", fmt.Errorf("unknown command %q", command)
	}
}

func (s *Service) status(ctx context.Context) (string, error) {
	assetCount, err := s.assets.Count(ctx)
	if err != nil {
		return "", err
	}
	serviceCount, err := s.services.Count(ctx)
	if err != nil {
		return "", err
	}
	latestRun, err := s.runs.Latest(ctx)
	if err != nil {
		return "", err
	}

	runStatus := "none"
	if latestRun != nil {
		runStatus = latestRun.Status
	}

	return fmt.Sprintf(
		"assets: %d\nservices: %d\nlatest run: %s",
		assetCount,
		serviceCount,
		runStatus,
	), nil
}

func (s *Service) discover(ctx context.Context, fields []string) (string, error) {
	if len(fields) != 3 {
		return "", fmt.Errorf("usage: discover <quick|normal> <target>")
	}
	run, err := s.discovery.Run(ctx, fields[1], fields[2])
	if err != nil {
		if run != nil {
			return fmt.Sprintf("discovery %s: %s", run.Status, run.Error), err
		}
		return "", err
	}
	return fmt.Sprintf(
		"discovery completed\nrun: %s\nhosts: %d\nservices: %d",
		run.ID,
		run.HostsFound,
		run.ServicesFound,
	), nil
}

func (s *Service) runsOutput(ctx context.Context) (string, error) {
	runs, err := s.runs.ListRecent(ctx, 10)
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "no discovery runs", nil
	}

	lines := make([]string, 0, len(runs))
	for _, run := range runs {
		lines = append(lines, fmt.Sprintf("%s %s %s", run.Profile, run.Target, run.Status))
	}
	return strings.Join(lines, "\n"), nil
}

func (s *Service) assetsOutput(ctx context.Context) (string, error) {
	assets, err := s.assets.List(ctx)
	if err != nil {
		return "", err
	}
	if len(assets) == 0 {
		return "no assets", nil
	}

	lines := make([]string, 0, len(assets))
	for _, asset := range assets {
		lines = append(lines, fmt.Sprintf("%s %s %s", asset.DisplayName, asset.AssetType, asset.PrimaryIP))
	}
	return strings.Join(lines, "\n"), nil
}

func (s *Service) servicesOutput(ctx context.Context) (string, error) {
	services, err := s.services.List(ctx)
	if err != nil {
		return "", err
	}
	if len(services) == 0 {
		return "no services", nil
	}

	lines := make([]string, 0, len(services))
	for _, service := range services {
		lines = append(lines, fmt.Sprintf("%s %s:%d %s", service.DisplayName, service.IPAddress, service.Port, service.Protocol))
	}
	return strings.Join(lines, "\n"), nil
}

func (s *Service) export(ctx context.Context, fields []string) (string, error) {
	if len(fields) != 2 {
		return "", fmt.Errorf("usage: export <yaml|markdown>")
	}

	snapshot, err := s.snapshot(ctx)
	if err != nil {
		return "", err
	}

	switch fields[1] {
	case "yaml":
		data, err := export.MarshalYAML(snapshot)
		if err != nil {
			return "", err
		}
		path := filepath.Join(s.dataDir, "exports", "inventory.yaml")
		if err := export.WriteFile(path, data); err != nil {
			return "", err
		}
		return fmt.Sprintf("wrote %s", path), nil
	case "markdown":
		data, err := export.RenderMarkdown(snapshot)
		if err != nil {
			return "", err
		}
		path := filepath.Join(s.dataDir, "exports", "inventory.md")
		if err := export.WriteFile(path, data); err != nil {
			return "", err
		}
		return fmt.Sprintf("wrote %s", path), nil
	default:
		return "", fmt.Errorf("unsupported export format %q", fields[1])
	}
}

func (s *Service) snapshot(ctx context.Context) (export.Snapshot, error) {
	assets, err := s.assets.List(ctx)
	if err != nil {
		return export.Snapshot{}, err
	}
	services, err := s.services.List(ctx)
	if err != nil {
		return export.Snapshot{}, err
	}
	return export.Snapshot{
		GeneratedAt: time.Now().UTC(),
		Assets:      assets,
		Services:    services,
		Metadata: map[string]string{
			"source": "shell",
		},
	}, nil
}
