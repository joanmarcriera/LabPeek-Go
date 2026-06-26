package nmap

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type Result struct {
	Hosts []Host
}

type Host struct {
	IPAddresses []string
	MACAddress  string
	Vendor      string
	Hostnames   []string
	Ports       []Port
	// UpReason is the nmap probe type that determined the host is up
	// (e.g. "arp-response", "echo-reply", "syn-ack", "reset").
	// Used downstream to filter TCP-RST false positives.
	UpReason string
}

type Port struct {
	Port        int
	Protocol    string
	ServiceName string
	Product     string
	Version     string
}

type xmlNmapRun struct {
	Hosts []xmlHost `xml:"host"`
}

type xmlHostStatus struct {
	State  string `xml:"state,attr"`
	Reason string `xml:"reason,attr"`
}

type xmlHost struct {
	Status    xmlHostStatus `xml:"status"`
	Addresses xmlAddresses  `xml:",any"`
	Hostnames []xmlName     `xml:"hostnames>hostname"`
	Ports     []xmlPort     `xml:"ports>port"`
}

type xmlAddresses []xmlAddress

type xmlAddress struct {
	XMLName  xml.Name
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
	Vendor   string `xml:"vendor,attr"`
}

type xmlName struct {
	Name string `xml:"name,attr"`
}

type xmlPort struct {
	Protocol string     `xml:"protocol,attr"`
	PortID   string     `xml:"portid,attr"`
	State    xmlState   `xml:"state"`
	Service  xmlService `xml:"service"`
}

type xmlState struct {
	State string `xml:"state,attr"`
}

type xmlService struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
}

func Parse(data []byte) (*Result, error) {
	var raw xmlNmapRun
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse nmap xml: %w", err)
	}

	result := &Result{
		Hosts: make([]Host, 0, len(raw.Hosts)),
	}

	for _, rawHost := range raw.Hosts {
		if strings.ToLower(rawHost.Status.State) != "up" {
			continue
		}
		host, err := mapHost(rawHost)
		if err != nil {
			return nil, err
		}
		result.Hosts = append(result.Hosts, host)
	}

	return result, nil
}

func mapHost(raw xmlHost) (Host, error) {
	host := Host{
		Hostnames: make([]string, 0, len(raw.Hostnames)),
		Ports:     make([]Port, 0, len(raw.Ports)),
		UpReason:  strings.ToLower(raw.Status.Reason),
	}

	for _, address := range raw.Addresses {
		switch strings.ToLower(address.AddrType) {
		case "ipv4", "ipv6":
			host.IPAddresses = append(host.IPAddresses, address.Addr)
		case "mac":
			host.MACAddress = address.Addr
			host.Vendor = address.Vendor
		}
	}

	for _, hostname := range raw.Hostnames {
		if hostname.Name == "" {
			continue
		}
		host.Hostnames = append(host.Hostnames, hostname.Name)
	}

	for _, rawPort := range raw.Ports {
		if strings.ToLower(rawPort.State.State) != "open" {
			continue
		}

		portNumber, err := strconv.Atoi(rawPort.PortID)
		if err != nil {
			return Host{}, fmt.Errorf("parse nmap port %q: %w", rawPort.PortID, err)
		}

		host.Ports = append(host.Ports, Port{
			Port:        portNumber,
			Protocol:    rawPort.Protocol,
			ServiceName: rawPort.Service.Name,
			Product:     rawPort.Service.Product,
			Version:     rawPort.Service.Version,
		})
	}

	return host, nil
}
