package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joanmarcriera/labpeek-go/internal/discovery/plugins/nmap"
	"github.com/joanmarcriera/labpeek-go/internal/models"
)

type Executor interface {
	Run(ctx context.Context, profile string, target string, outputPath string) (string, error)
}

type Option func(*Service)

type Service struct {
	dataDir     string
	assets      *models.AssetRepository
	services    *models.ServiceRepository
	runs        *models.DiscoveryRunRepository
	executor    Executor
	now         func() time.Time
	allowPublic bool
	cancels     sync.Map // map[runID string]context.CancelFunc
}

type ImportStats struct {
	HostsFound        int
	ServicesFound     int
	ObservationsCount int
}

func NewService(
	dataDir string,
	assets *models.AssetRepository,
	services *models.ServiceRepository,
	runs *models.DiscoveryRunRepository,
	options ...Option,
) *Service {
	service := &Service{
		dataDir:  dataDir,
		assets:   assets,
		services: services,
		runs:     runs,
		executor: nmapExecutor{},
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func WithExecutor(executor Executor) Option {
	return func(service *Service) {
		service.executor = executor
	}
}

func WithNow(now func() time.Time) Option {
	return func(service *Service) {
		service.now = now
	}
}

func WithAllowPublic(allow bool) Option {
	return func(service *Service) {
		service.allowPublic = allow
	}
}

func ValidateTarget(target string, allowPublic bool) error {
	if ip := net.ParseIP(target); ip != nil {
		if isAllowedIP(ip, allowPublic) {
			return nil
		}
		return fmt.Errorf("target %s is not in an allowed private range", target)
	}

	ip, _, err := net.ParseCIDR(target)
	if err != nil {
		return fmt.Errorf("invalid target %q", target)
	}
	if isAllowedIP(ip, allowPublic) {
		return nil
	}
	return fmt.Errorf("target %s is not in an allowed private range", target)
}

func (s *Service) Run(ctx context.Context, profile string, target string) (*models.DiscoveryRun, error) {
	run, err := s.Queue(ctx, profile, target)
	if err != nil {
		return nil, err
	}
	return s.ExecuteQueued(ctx, run)
}

func (s *Service) Queue(ctx context.Context, profile string, target string) (*models.DiscoveryRun, error) {
	if err := ValidateTarget(target, s.allowPublic); err != nil {
		return nil, err
	}

	now := s.now()
	run := &models.DiscoveryRun{
		Profile:       profile,
		Target:        target,
		Status:        "queued",
		CurrentPhase:  "queued",
		CreatedAt:     now,
		UpdatedAt:     now,
		RawOutputPath: filepath.Join(s.dataDir, "discovery", fmt.Sprintf("%d.xml", now.UnixNano())),
	}
	if err := s.runs.Create(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Service) Cancel(ctx context.Context, runID string) error {
	if cancel, ok := s.cancels.LoadAndDelete(runID); ok {
		cancel.(context.CancelFunc)()
	}
	return s.runs.SetCancelled(ctx, runID)
}

func (s *Service) ExecuteQueued(ctx context.Context, run *models.DiscoveryRun) (*models.DiscoveryRun, error) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancels.Store(run.ID, cancel)
	defer func() {
		cancel()
		s.cancels.Delete(run.ID)
	}()

	now := s.now()
	run.Status = "running"
	run.CurrentPhase = "scanning"
	run.StartedAt = now
	run.ProgressPercent = 10
	if err := s.runs.Update(ctx, run); err != nil {
		return nil, err
	}

	logs, err := s.executor.Run(ctx, run.Profile, run.Target, run.RawOutputPath)
	run.Logs = logs
	if err != nil {
		run.Status = "failed"
		run.CurrentPhase = "failed"
		run.CompletedAt = s.now()
		run.ProgressPercent = 100
		run.Error = fmt.Sprintf("nmap run failed: %v", err)
		_ = s.runs.Update(ctx, run)
		return run, err
	}

	rawXML, err := os.ReadFile(run.RawOutputPath)
	if err != nil {
		run.Status = "failed"
		run.CurrentPhase = "failed"
		run.CompletedAt = s.now()
		run.ProgressPercent = 100
		run.Error = fmt.Sprintf("read nmap output: %v", err)
		_ = s.runs.Update(ctx, run)
		return run, err
	}

	result, err := nmap.Parse(rawXML)
	if err != nil {
		run.Status = "failed"
		run.CurrentPhase = "failed"
		run.CompletedAt = s.now()
		run.ProgressPercent = 100
		run.Error = fmt.Sprintf("parse nmap output: %v", err)
		_ = s.runs.Update(ctx, run)
		return run, err
	}

	stats, err := s.ImportResult(ctx, result)
	if err != nil {
		run.Status = "failed"
		run.CurrentPhase = "failed"
		run.CompletedAt = s.now()
		run.ProgressPercent = 100
		run.Error = fmt.Sprintf("import discovery result: %v", err)
		_ = s.runs.Update(ctx, run)
		return run, err
	}

	run.Status = "completed"
	run.CurrentPhase = "completed"
	run.ProgressPercent = 100
	run.HostsFound = stats.HostsFound
	run.ServicesFound = stats.ServicesFound
	run.ObservationsCount = stats.ObservationsCount
	run.CompletedAt = s.now()
	run.Logs = strings.TrimSpace(run.Logs + "\nparsed discovery output")
	if err := s.runs.Update(ctx, run); err != nil {
		return nil, err
	}

	return run, nil
}

func (s *Service) ImportResult(ctx context.Context, result *nmap.Result) (ImportStats, error) {
	tx, err := s.assets.DB().BeginTx(ctx, nil)
	if err != nil {
		return ImportStats{}, fmt.Errorf("begin discovery import tx: %w", err)
	}
	defer tx.Rollback()

	assets := s.assets.WithTx(tx)
	services := s.services.WithTx(tx)
	now := s.now()
	stats := ImportStats{}

	for _, host := range result.Hosts {
		if !isConfirmedDevice(host) {
			continue
		}
		stats.HostsFound++
		asset, err := s.matchOrCreateAsset(ctx, assets, host, now)
		if err != nil {
			return ImportStats{}, err
		}
		for _, port := range host.Ports {
			stats.ServicesFound++
			stats.ObservationsCount++
			ip := firstIPAddress(host.IPAddresses)
			if err := services.UpsertObserved(ctx, models.ObservedService{
				AssetID:     asset.ID,
				DisplayName: displayNameForService(port),
				IPAddress:   ip,
				Port:        port.Port,
				Protocol:    port.Protocol,
				Transport:   "tcp",
				ServiceName: port.ServiceName,
				Product:     port.Product,
				Version:     port.Version,
				ObservedAt:  now,
			}); err != nil {
				return ImportStats{}, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return ImportStats{}, fmt.Errorf("commit discovery import tx: %w", err)
	}
	return stats, nil
}

func (s *Service) matchOrCreateAsset(ctx context.Context, assets *models.AssetRepository, host nmap.Host, observedAt time.Time) (*models.Asset, error) {
	if asset, err := assets.FindByPrimaryMAC(ctx, host.MACAddress); err != nil {
		return nil, err
	} else if asset != nil {
		return s.applyDiscoveredFields(ctx, assets, asset, host, observedAt)
	}

	if hostname := firstHostname(host.Hostnames); hostname != "" {
		matches, err := assets.FindByDiscoveredName(ctx, hostname)
		if err != nil {
			return nil, err
		}
		if len(matches) == 1 {
			return s.applyDiscoveredFields(ctx, assets, &matches[0], host, observedAt)
		}
	}

	if ip := firstIPAddress(host.IPAddresses); ip != "" {
		asset, err := assets.FindByPrimaryIP(ctx, ip)
		if err != nil {
			return nil, err
		}
		if asset != nil && (asset.PrimaryMAC == "" || asset.PrimaryMAC == host.MACAddress || host.MACAddress == "") {
			return s.applyDiscoveredFields(ctx, assets, asset, host, observedAt)
		}
	}

	discoveredName := firstHostname(host.Hostnames)
	asset := &models.Asset{
		DisplayName:        defaultAssetDisplayName(host),
		DiscoveredName:     discoveredName,
		AssetType:          "unknown",
		Status:             "active",
		PrimaryIP:          firstIPAddress(host.IPAddresses),
		PrimaryMAC:         host.MACAddress,
		MACVendor:          host.Vendor,
		ManualDataJSON:     "{}",
		DiscoveredDataJSON: discoveredDataJSON(host),
		FirstSeenAt:        observedAt,
		LastSeenAt:         observedAt,
		CreatedAt:          observedAt,
		UpdatedAt:          observedAt,
	}
	if err := assets.Create(ctx, asset); err != nil {
		return nil, err
	}
	return asset, nil
}

func (s *Service) applyDiscoveredFields(ctx context.Context, assets *models.AssetRepository, asset *models.Asset, host nmap.Host, observedAt time.Time) (*models.Asset, error) {
	asset.DiscoveredName = firstHostname(host.Hostnames)
	asset.PrimaryIP = firstIPAddress(host.IPAddresses)
	asset.PrimaryMAC = host.MACAddress
	asset.MACVendor = host.Vendor
	asset.DiscoveredDataJSON = discoveredDataJSON(host)
	if asset.FirstSeenAt.IsZero() {
		asset.FirstSeenAt = observedAt
	}
	asset.LastSeenAt = observedAt
	asset.UpdatedAt = observedAt
	if err := assets.SaveDiscoveredFields(ctx, asset); err != nil {
		return nil, err
	}
	return asset, nil
}

func firstIPAddress(addresses []string) string {
	for _, address := range addresses {
		if address != "" {
			return address
		}
	}
	return ""
}

func firstHostname(hostnames []string) string {
	for _, hostname := range hostnames {
		if hostname != "" {
			return hostname
		}
	}
	return ""
}

func defaultAssetDisplayName(host nmap.Host) string {
	if hostname := firstHostname(host.Hostnames); hostname != "" {
		return hostname
	}
	if ip := firstIPAddress(host.IPAddresses); ip != "" {
		return "host-" + strings.ReplaceAll(ip, ".", "-")
	}
	return "host"
}

func displayNameForService(port nmap.Port) string {
	if port.ServiceName != "" {
		return port.ServiceName
	}
	return fmt.Sprintf("%s-%d", port.Protocol, port.Port)
}

func discoveredDataJSON(host nmap.Host) string {
	payload, err := json.Marshal(host)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func isAllowedIP(ip net.IP, allowPublic bool) bool {
	if ip == nil {
		return false
	}
	if allowPublic {
		return true
	}
	if ip.IsLoopback() {
		return true
	}

	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}
	for _, cidr := range privateCIDRs {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// isConfirmedDevice returns true only when nmap evidence is strong enough to
// treat the host as a real device worth creating a CMDB asset for.
//
// We require at least one of:
//   - MAC address present → device answered an ARP request on L2 (cannot be forged by a router)
//   - Open ports → a real service probe returned data
//   - Hostname resolved → DNS confirmed
//
// ICMP-only responses ("echo-reply", "timestamp-reply") are intentionally
// NOT trusted on their own. Home routers and ISP modems commonly act as ICMP
// proxies, replying to every address in the /24 on behalf of unassigned IPs —
// including the network address (e.g. 192.168.0.0). This produces 256 false
// positives on a ping sweep. The same applies to "arp-response" without a MAC
// (proxy-ARP) and "syn-ack"/"reset" without open ports.
//
// A device that can only be reached via ICMP and has no open ports will not
// appear in a quick ping sweep, but WILL appear in a normal/deep scan once
// it exposes a service.
func isConfirmedDevice(host nmap.Host) bool {
	if host.MACAddress != "" {
		return true
	}
	if len(host.Ports) > 0 {
		return true
	}
	if len(host.Hostnames) > 0 {
		return true
	}
	return false
}

type nmapExecutor struct{}

func (nmapExecutor) Run(ctx context.Context, profile string, target string, outputPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return "", err
	}

	if _, err := exec.LookPath("nmap"); err != nil {
		return "", errors.New("nmap is not installed or not on PATH")
	}

	args, err := nmapArgs(profile, target, outputPath)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "nmap", args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func nmapArgs(profile string, target string, outputPath string) ([]string, error) {
	switch profile {
	case "quick":
		// Scanning a /24 in parallel saturates Docker's NAT connection-tracking
		// table, causing the router to reply with ICMP instead of TCP SYN-ACK.
		// nmap then writes <hosthint> (not <host>) with reason="unknown-response"
		// and the subsequent port scan gets all-filtered → zero results.
		//
		// --max-hostgroup 1 forces nmap to finish one host before starting the
		// next, keeping the concurrent probe count low enough that NAT state
		// is never overwhelmed. This matches the behaviour of a manual single-IP
		// scan (which always works correctly).
		//
		// -PS uses TCP SYN for host discovery, not ICMP, so ICMP-proxy routers
		// that reply to every ping in the /24 cannot produce false positives.
		// --max-retries 2 limits retransmissions to keep the scan fast.
		return []string{"-T4", "-n", "-PS22,80,443,445,8080,8443", "--max-hostgroup", "1", "--max-retries", "2", "--top-ports", "20", "--open", "--host-timeout", "10s", "-oX", outputPath, target}, nil
	case "normal":
		return []string{"-sV", "-n", "--top-ports", "100", "-oX", outputPath, target}, nil
	case "deep":
		return []string{"-sV", "-O", "-n", "-p-", "--max-retries", "3", "-oX", outputPath, target}, nil
	case "slow-safe":
		return []string{"-sV", "-n", "--top-ports", "100", "--scan-delay", "100ms", "--max-rate", "50", "-oX", outputPath, target}, nil
	default:
		return nil, fmt.Errorf("unsupported profile %q", profile)
	}
}
