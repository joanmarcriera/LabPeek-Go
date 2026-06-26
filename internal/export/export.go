package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joanmarcriera/labpeek-go/internal/models"
	"gopkg.in/yaml.v3"
)

type Snapshot struct {
	GeneratedAt time.Time             `yaml:"generated_at"`
	Assets      []models.Asset        `yaml:"assets"`
	Services    []models.Service      `yaml:"services"`
	Metadata    map[string]string     `yaml:"metadata,omitempty"`
}

func MarshalYAML(snapshot Snapshot) ([]byte, error) {
	return yaml.Marshal(snapshot)
}

func RenderMarkdown(snapshot Snapshot) ([]byte, error) {
	var builder strings.Builder
	builder.WriteString("# LabPeek Inventory\n\n")
	builder.WriteString(fmt.Sprintf("Generated: %s\n\n", snapshot.GeneratedAt.UTC().Format(time.RFC3339)))

	builder.WriteString("## Assets\n\n")
	builder.WriteString("| Name | Type | IP |\n")
	builder.WriteString("| --- | --- | --- |\n")
	for _, asset := range snapshot.Assets {
		builder.WriteString(fmt.Sprintf("| %s | %s | %s |\n", asset.DisplayName, asset.AssetType, asset.PrimaryIP))
	}

	builder.WriteString("\n## Services\n\n")
	builder.WriteString("| Name | Asset ID | IP | Port | Protocol |\n")
	builder.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, service := range snapshot.Services {
		builder.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %s |\n", service.DisplayName, service.AssetID, service.IPAddress, service.Port, service.Protocol))
	}

	return []byte(builder.String()), nil
}

func WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
