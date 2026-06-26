package models

import "time"

type Asset struct {
	ID                 string
	DisplayName        string
	DiscoveredName     string
	AssetType          string
	Status             string
	PrimaryIP          string
	PrimaryMAC         string
	MACVendor          string
	Notes              string
	ManualDataJSON     string
	DiscoveredDataJSON string
	FirstSeenAt        time.Time
	LastSeenAt         time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AssetUpdate struct {
	ID          string
	DisplayName string
	AssetType   string
	Notes       string
}

type Service struct {
	ID          string
	AssetID     string
	DisplayName string
	IPAddress   string
	Port        int
	Protocol    string
	Transport   string
	ServiceName string
	Product     string
	Version     string
	URL         string
	Status      string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ObservedService struct {
	AssetID     string
	DisplayName string
	IPAddress   string
	Port        int
	Protocol    string
	Transport   string
	ServiceName string
	Product     string
	Version     string
	ObservedAt  time.Time
}

type DiscoveryRun struct {
	ID                string
	Profile           string
	Target            string
	Status            string
	CurrentPhase      string
	ProgressPercent   int
	HostsFound        int
	ServicesFound     int
	ObservationsCount int
	Error             string
	Logs              string
	RawOutputPath     string
	StartedAt         time.Time
	CompletedAt       time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type NetworkSettings struct {
	Label         string
	DefaultTarget string
}
