package discovery

import (
	"slices"
	"strings"
	"testing"
)

func TestNmapArgsContainNoDNSFlagForAllProfiles(t *testing.T) {
	t.Parallel()

	profiles := []string{"quick", "normal", "deep", "slow-safe"}
	for _, profile := range profiles {
		args, err := nmapArgs(profile, "192.168.1.0/24", "/tmp/out.xml")
		if err != nil {
			t.Fatalf("profile %q: unexpected error: %v", profile, err)
		}
		if !slices.Contains(args, "-n") {
			t.Errorf("profile %q: missing -n flag (DNS skipping); args = %v", profile, args)
		}
	}
}

func TestNmapArgsQuickProfileHasHostTimeout(t *testing.T) {
	t.Parallel()

	args, err := nmapArgs("quick", "192.168.1.0/24", "/tmp/out.xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Contains(args, "--host-timeout") {
		t.Errorf("quick profile missing --host-timeout; args = %v", args)
	}
}

func TestNmapArgsQuickProfileUsesTCPSYNDiscoveryWithMaxHostgroup1(t *testing.T) {
	t.Parallel()

	args, err := nmapArgs("quick", "192.168.1.0/24", "/tmp/out.xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must use TCP SYN host discovery so ICMP-proxy routers (which reply to
	// every ping for the entire /24) cannot produce false positives.
	hasTCPSYN := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-PS") {
			hasTCPSYN = true
			break
		}
	}
	if !hasTCPSYN {
		t.Errorf("quick profile missing -PS (TCP SYN host discovery); args = %v", args)
	}

	// --max-hostgroup 1 ensures nmap scans one host at a time, preventing
	// Docker NAT saturation that causes all port probes to appear filtered.
	if !slices.Contains(args, "--max-hostgroup") {
		t.Errorf("quick profile missing --max-hostgroup; args = %v", args)
	}
	maxHGIdx := slices.Index(args, "--max-hostgroup")
	if maxHGIdx < 0 || maxHGIdx+1 >= len(args) || args[maxHGIdx+1] != "1" {
		t.Errorf("quick profile --max-hostgroup must be 1; args = %v", args)
	}

	if !slices.Contains(args, "--top-ports") {
		t.Errorf("quick profile missing --top-ports; args = %v", args)
	}
	if slices.Contains(args, "-sn") {
		t.Errorf("quick profile must not use -sn (ICMP ping sweep); args = %v", args)
	}
	if slices.Contains(args, "-Pn") {
		t.Errorf("quick profile must not use -Pn (would scan all 256 IPs regardless); args = %v", args)
	}
}

func TestNmapArgsRejectsUnknownProfile(t *testing.T) {
	t.Parallel()

	if _, err := nmapArgs("unknown-profile", "192.168.1.0/24", "/tmp/out.xml"); err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
}
