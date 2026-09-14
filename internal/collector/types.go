package collector

import "time"

import "github.com/dorta/var-studio/internal/boards"

type Paths struct {
	Proc string
	Sys  string
	Etc  string
}

type Snapshot struct {
	Timestamp time.Time       `json:"timestamp"`
	System    SystemInfo      `json:"system"`
	Board     boards.Product  `json:"board"`
	BSP       BSPInfo         `json:"bsp"`
	Metrics   Metrics         `json:"metrics"`
	Thermals  []Thermal       `json:"thermals"`
	Networks  []Network       `json:"networks"`
	Ports     []ListeningPort `json:"ports"`
	Warnings  []string        `json:"warnings,omitempty"`
}

type SystemInfo struct {
	Hostname     string   `json:"hostname"`
	Model        string   `json:"model"`
	Compatible   []string `json:"compatible"`
	Architecture string   `json:"architecture"`
	Kernel       string   `json:"kernel"`
	OSName       string   `json:"os_name"`
	OSVersion    string   `json:"os_version"`
	UptimeSec    float64  `json:"uptime_seconds"`
	Cores        int      `json:"cores"`
}

type BSPInfo struct {
	Distro        string          `json:"distro"`
	DistroVersion string          `json:"distro_version"`
	Layers        []LayerRevision `json:"layers"`
}

type LayerRevision struct {
	Name          string `json:"name"`
	Revision      string `json:"revision"`
	Modified      bool   `json:"modified"`
	RepositoryURL string `json:"repository_url,omitempty"`
	RevisionURL   string `json:"revision_url,omitempty"`
}

type Metrics struct {
	CPUPercent   float64   `json:"cpu_percent"`
	PerCore      []float64 `json:"per_core"`
	Load         []float64 `json:"load"`
	MemoryTotal  uint64    `json:"memory_total_bytes"`
	MemoryUsed   uint64    `json:"memory_used_bytes"`
	StorageTotal uint64    `json:"storage_total_bytes"`
	StorageUsed  uint64    `json:"storage_used_bytes"`
}

type Thermal struct {
	Name    string  `json:"name"`
	Celsius float64 `json:"celsius"`
}

type Network struct {
	Name      string   `json:"name"`
	State     string   `json:"state"`
	MAC       string   `json:"mac"`
	Addresses []string `json:"addresses"`
	RXBytes   uint64   `json:"rx_bytes"`
	TXBytes   uint64   `json:"tx_bytes"`
	RXErrors  uint64   `json:"rx_errors"`
	TXErrors  uint64   `json:"tx_errors"`
}

type ListeningPort struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
}
