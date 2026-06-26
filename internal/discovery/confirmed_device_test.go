package discovery

import (
	"testing"

	"github.com/joanmarcriera/labpeek-go/internal/discovery/plugins/nmap"
)

func TestIsConfirmedDevice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host nmap.Host
		want bool
	}{
		{
			name: "ARP response with MAC - definitely real",
			host: nmap.Host{IPAddresses: []string{"192.168.1.1"}, MACAddress: "AA:BB:CC:DD:EE:FF", UpReason: "arp-response"},
			want: true,
		},
		{
			name: "ICMP echo reply, no MAC - router ICMP proxy false positive",
			host: nmap.Host{IPAddresses: []string{"192.168.1.2"}, UpReason: "echo-reply"},
			want: false,
		},
		{
			name: "network address 192.168.0.0 echo-reply - must never become an asset",
			host: nmap.Host{IPAddresses: []string{"192.168.0.0"}, UpReason: "echo-reply"},
			want: false,
		},
		{
			name: "SYN-ACK, no MAC, no ports - not enough evidence",
			host: nmap.Host{IPAddresses: []string{"192.168.1.3"}, UpReason: "syn-ack"},
			want: false,
		},
		{
			name: "has open ports - real host regardless of reason",
			host: nmap.Host{
				IPAddresses: []string{"192.168.1.4"},
				UpReason:    "reset",
				Ports:       []nmap.Port{{Port: 22, Protocol: "tcp", ServiceName: "ssh"}},
			},
			want: true,
		},
		{
			name: "has hostname - real host",
			host: nmap.Host{IPAddresses: []string{"192.168.1.5"}, Hostnames: []string{"nas.local"}, UpReason: "reset"},
			want: true,
		},
		{
			name: "TCP RST only, no MAC, no ports, no hostname - router false positive",
			host: nmap.Host{IPAddresses: []string{"192.168.1.100"}, UpReason: "reset"},
			want: false,
		},
		{
			name: "TCP RST for all 256 IPs in subnet - all should be filtered",
			host: nmap.Host{IPAddresses: []string{"192.168.1.200"}, UpReason: "reset"},
			want: false,
		},
		{
			name: "MAC present even with reset reason - real device",
			host: nmap.Host{IPAddresses: []string{"192.168.1.6"}, MACAddress: "11:22:33:44:55:66", UpReason: "reset"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConfirmedDevice(tt.host)
			if got != tt.want {
				t.Errorf("isConfirmedDevice(%+v) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}
