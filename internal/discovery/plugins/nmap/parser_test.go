package nmap_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joanmarcriera/labpeek-go/internal/discovery/plugins/nmap"
)

func TestParseQuickScanFixture(t *testing.T) {
	t.Parallel()

	result := parseFixture(t, "quick_scan.xml")

	if len(result.Hosts) != 2 {
		t.Fatalf("host count = %d, want 2", len(result.Hosts))
	}

	first := result.Hosts[0]
	if len(first.IPAddresses) != 1 || first.IPAddresses[0] != "192.168.1.10" {
		t.Fatalf("first host IPs = %#v, want [192.168.1.10]", first.IPAddresses)
	}
	if first.MACAddress != "00:11:22:33:44:55" {
		t.Fatalf("first host MAC = %q, want %q", first.MACAddress, "00:11:22:33:44:55")
	}
	if first.Vendor != "Synology Inc." {
		t.Fatalf("first host vendor = %q, want %q", first.Vendor, "Synology Inc.")
	}
	if len(first.Hostnames) != 1 || first.Hostnames[0] != "nas.local" {
		t.Fatalf("first host hostnames = %#v, want [nas.local]", first.Hostnames)
	}
	if len(first.Ports) != 0 {
		t.Fatalf("first host open ports = %d, want 0", len(first.Ports))
	}

	second := result.Hosts[1]
	if len(second.IPAddresses) != 1 || second.IPAddresses[0] != "192.168.1.20" {
		t.Fatalf("second host IPs = %#v, want [192.168.1.20]", second.IPAddresses)
	}
	if second.MACAddress != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("second host MAC = %q, want %q", second.MACAddress, "AA:BB:CC:DD:EE:FF")
	}
	if second.Vendor != "" {
		t.Fatalf("second host vendor = %q, want empty string", second.Vendor)
	}
	if len(second.Hostnames) != 1 || second.Hostnames[0] != "printer.local" {
		t.Fatalf("second host hostnames = %#v, want [printer.local]", second.Hostnames)
	}
}

func TestParseNormalScanFixture(t *testing.T) {
	t.Parallel()

	result := parseFixture(t, "normal_scan.xml")

	if len(result.Hosts) != 1 {
		t.Fatalf("host count = %d, want 1", len(result.Hosts))
	}

	host := result.Hosts[0]
	if len(host.IPAddresses) != 1 || host.IPAddresses[0] != "192.168.1.30" {
		t.Fatalf("host IPs = %#v, want [192.168.1.30]", host.IPAddresses)
	}
	if host.MACAddress != "10:20:30:40:50:60" {
		t.Fatalf("host MAC = %q, want %q", host.MACAddress, "10:20:30:40:50:60")
	}
	if host.Vendor != "Intel Corporate" {
		t.Fatalf("host vendor = %q, want %q", host.Vendor, "Intel Corporate")
	}
	if len(host.Hostnames) != 1 || host.Hostnames[0] != "appserver.local" {
		t.Fatalf("host hostnames = %#v, want [appserver.local]", host.Hostnames)
	}
	if len(host.Ports) != 2 {
		t.Fatalf("open port count = %d, want 2", len(host.Ports))
	}

	ssh := host.Ports[0]
	if ssh.Port != 22 {
		t.Fatalf("ssh port = %d, want 22", ssh.Port)
	}
	if ssh.Protocol != "tcp" {
		t.Fatalf("ssh protocol = %q, want %q", ssh.Protocol, "tcp")
	}
	if ssh.ServiceName != "ssh" {
		t.Fatalf("ssh service = %q, want %q", ssh.ServiceName, "ssh")
	}
	if ssh.Product != "OpenSSH" {
		t.Fatalf("ssh product = %q, want %q", ssh.Product, "OpenSSH")
	}
	if ssh.Version != "9.3" {
		t.Fatalf("ssh version = %q, want %q", ssh.Version, "9.3")
	}

	https := host.Ports[1]
	if https.Port != 443 {
		t.Fatalf("https port = %d, want 443", https.Port)
	}
	if https.Protocol != "tcp" {
		t.Fatalf("https protocol = %q, want %q", https.Protocol, "tcp")
	}
	if https.ServiceName != "https" {
		t.Fatalf("https service = %q, want %q", https.ServiceName, "https")
	}
	if https.Product != "nginx" {
		t.Fatalf("https product = %q, want %q", https.Product, "nginx")
	}
	if https.Version != "1.24.0" {
		t.Fatalf("https version = %q, want %q", https.Version, "1.24.0")
	}
}

func TestParsePopulatesUpReason(t *testing.T) {
	t.Parallel()

	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<nmaprun>
  <host>
    <status state="up" reason="arp-response" reason_ttl="0"/>
    <address addr="192.168.1.1" addrtype="ipv4"/>
    <address addr="AA:BB:CC:DD:EE:01" addrtype="mac"/>
  </host>
  <host>
    <status state="up" reason="echo-reply" reason_ttl="64"/>
    <address addr="192.168.1.2" addrtype="ipv4"/>
  </host>
  <host>
    <status state="up" reason="reset" reason_ttl="64"/>
    <address addr="192.168.1.3" addrtype="ipv4"/>
  </host>
</nmaprun>`)

	result, err := nmap.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Hosts) != 3 {
		t.Fatalf("host count = %d, want 3", len(result.Hosts))
	}
	if result.Hosts[0].UpReason != "arp-response" {
		t.Errorf("host[0] UpReason = %q, want arp-response", result.Hosts[0].UpReason)
	}
	if result.Hosts[1].UpReason != "echo-reply" {
		t.Errorf("host[1] UpReason = %q, want echo-reply", result.Hosts[1].UpReason)
	}
	if result.Hosts[2].UpReason != "reset" {
		t.Errorf("host[2] UpReason = %q, want reset", result.Hosts[2].UpReason)
	}
}

func TestParseFiltersOutDownHosts(t *testing.T) {
	t.Parallel()

	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<nmaprun>
  <host>
    <status state="up"/>
    <address addr="192.168.1.10" addrtype="ipv4"/>
    <address addr="AA:BB:CC:DD:EE:01" addrtype="mac"/>
    <hostnames><hostname name="up-host.local" type="PTR"/></hostnames>
  </host>
  <host>
    <status state="down"/>
    <address addr="192.168.1.11" addrtype="ipv4"/>
    <address addr="AA:BB:CC:DD:EE:02" addrtype="mac"/>
    <hostnames><hostname name="down-host.local" type="PTR"/></hostnames>
  </host>
  <host>
    <status state="up"/>
    <address addr="192.168.1.12" addrtype="ipv4"/>
    <address addr="AA:BB:CC:DD:EE:03" addrtype="mac"/>
  </host>
</nmaprun>`)

	result, err := nmap.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Hosts) != 2 {
		t.Fatalf("host count = %d, want 2 (only up hosts)", len(result.Hosts))
	}
	if result.Hosts[0].IPAddresses[0] != "192.168.1.10" {
		t.Fatalf("first host IP = %q, want 192.168.1.10", result.Hosts[0].IPAddresses[0])
	}
	if result.Hosts[1].IPAddresses[0] != "192.168.1.12" {
		t.Fatalf("second host IP = %q, want 192.168.1.12", result.Hosts[1].IPAddresses[0])
	}
}

func TestParseFiltersClosedPorts(t *testing.T) {
	t.Parallel()

	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<nmaprun>
  <host>
    <status state="up"/>
    <address addr="192.168.1.50" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="22"><state state="open"/><service name="ssh"/></port>
      <port protocol="tcp" portid="80"><state state="closed"/><service name="http"/></port>
      <port protocol="tcp" portid="443"><state state="filtered"/><service name="https"/></port>
    </ports>
  </host>
</nmaprun>`)

	result, err := nmap.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Hosts) != 1 {
		t.Fatalf("host count = %d, want 1", len(result.Hosts))
	}
	if len(result.Hosts[0].Ports) != 1 {
		t.Fatalf("port count = %d, want 1 (only open ports)", len(result.Hosts[0].Ports))
	}
	if result.Hosts[0].Ports[0].Port != 22 {
		t.Fatalf("port = %d, want 22", result.Hosts[0].Ports[0].Port)
	}
}

func parseFixture(t *testing.T, name string) *nmap.Result {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	result, err := nmap.Parse(data)
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}

	return result
}
