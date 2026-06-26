package export_test

import (
	"strings"
	"testing"
	"time"

	"github.com/joanmarcriera/labpeek-go/internal/export"
	"github.com/joanmarcriera/labpeek-go/internal/models"
)

func TestMarshalYAMLIncludesAssetsAndServices(t *testing.T) {
	t.Parallel()

	snapshot := export.Snapshot{
		GeneratedAt: time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC),
		Assets: []models.Asset{
			{ID: "asset-1", DisplayName: "truenas-main", AssetType: "nas", PrimaryIP: "192.168.1.50"},
		},
		Services: []models.Service{
			{ID: "service-1", AssetID: "asset-1", DisplayName: "ssh", IPAddress: "192.168.1.50", Port: 22, Protocol: "ssh", Transport: "tcp"},
		},
	}

	data, err := export.MarshalYAML(snapshot)
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}

	text := string(data)
	for _, needle := range []string{"truenas-main", "ssh", "192.168.1.50"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("yaml missing %q: %s", needle, text)
		}
	}
}

func TestRenderMarkdownIncludesAssetsAndServices(t *testing.T) {
	t.Parallel()

	snapshot := export.Snapshot{
		GeneratedAt: time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC),
		Assets: []models.Asset{
			{ID: "asset-1", DisplayName: "truenas-main", AssetType: "nas", PrimaryIP: "192.168.1.50"},
		},
		Services: []models.Service{
			{ID: "service-1", AssetID: "asset-1", DisplayName: "ssh", IPAddress: "192.168.1.50", Port: 22, Protocol: "ssh", Transport: "tcp"},
		},
	}

	data, err := export.RenderMarkdown(snapshot)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	text := string(data)
	for _, needle := range []string{"# LabPeek Inventory", "truenas-main", "ssh", "192.168.1.50"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("markdown missing %q: %s", needle, text)
		}
	}
}
