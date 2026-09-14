package collector

import (
	"bufio"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dorta/var-studio/internal/boards"
)

type Collector struct {
	paths Paths
	mu    sync.Mutex
	prev  []cpuTime
}

type cpuTime struct{ idle, total uint64 }

func New(
	paths Paths,
) *Collector {
	return &Collector{paths: paths}
}

func (c *Collector) Snapshot() Snapshot {
	s := Snapshot{Timestamp: time.Now().UTC()}
	s.System = c.systemInfo(&s.Warnings)
	s.Board = boards.Detect(
		s.System.Model,
		s.System.Compatible,
	)
	s.BSP = c.buildInfo(&s.Warnings)
	s.Metrics = c.metrics(&s.Warnings)
	s.Thermals = c.thermals()
	s.Networks = c.networks()
	s.Ports = c.ports()
	return s
}

func (c *Collector) systemInfo(
	warnings *[]string,
) SystemInfo {
	info := SystemInfo{Architecture: runtime.GOARCH}
	info.Hostname = clean(c.read(c.paths.Etc, "hostname"))
	info.Model = cleanNUL(
		c.read(
			c.paths.Sys,
			"firmware/devicetree/base/model",
		),
	)
	info.Compatible = splitNUL(
		c.read(
			c.paths.Sys,
			"firmware/devicetree/base/compatible",
		),
	)
	info.Kernel = clean(
		c.read(c.paths.Proc, "sys/kernel/osrelease"),
	)
	if value := strings.Fields(c.read(c.paths.Proc, "uptime")); len(
		value,
	) > 0 {
		info.UptimeSec, _ = strconv.ParseFloat(value[0], 64)
	}
	osRelease := c.read(c.paths.Etc, "os-release")
	if osRelease == "" {
		osRelease = read(
			filepath.Join(
				filepath.Dir(c.paths.Etc),
				"os-release",
			),
		)
	}
	for _, line := range strings.Split(osRelease, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, "\"")
		switch key {
		case "PRETTY_NAME":
			info.OSName = value
		case "VERSION_ID":
			info.OSVersion = value
		}
	}
	info.Cores = len(
		glob(
			filepath.Join(
				c.paths.Sys,
				"devices/system/cpu/cpu[0-9]*",
			),
		),
	)
	if info.Model == "" {
		*warnings = append(
			*warnings,
			"Device-tree model is unavailable",
		)
	}
	return info
}

func (c *Collector) buildInfo(warnings *[]string) BSPInfo {
	var result = BSPInfo{Layers: []LayerRevision{}}
	raw := c.read(c.paths.Etc, "buildinfo")
	if raw == "" {
		*warnings = append(
			*warnings,
			"/etc/buildinfo is unavailable",
		)
		return result
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			key, value = strings.TrimSpace(
				key,
			), strings.TrimSpace(
				value,
			)
			value = strings.Trim(value, "\"")
			switch key {
			case "DISTRO":
				result.Distro = value
			case "DISTRO_VERSION":
				result.DistroVersion = value
			default:
				if strings.Contains(value, "HEAD:") {
					revision := strings.TrimSpace(
						strings.TrimSuffix(
							strings.TrimPrefix(
								value,
								"HEAD:",
							),
							"-- modified",
						),
					)
					repositoryURL, revisionURL := layerLinks(
						key,
						revision,
					)
					result.Layers = append(
						result.Layers,
						LayerRevision{
							Name:     key,
							Revision: revision,
							Modified: strings.Contains(
								value,
								"-- modified",
							),
							RepositoryURL: repositoryURL,
							RevisionURL:   revisionURL,
						},
					)
				}
			}
		}
	}
	return result
}

func (c *Collector) metrics(warnings *[]string) Metrics {
	var m = Metrics{PerCore: []float64{}, Load: []float64{}}
	current := readCPUTimes(c.read(c.paths.Proc, "stat"))
	c.mu.Lock()
	if len(c.prev) == len(current) {
		for i := range current {
			dTotal := current[i].total - c.prev[i].total
			dIdle := current[i].idle - c.prev[i].idle
			usage := 0.0
			if dTotal > 0 {
				usage = 100 * float64(
					dTotal-dIdle,
				) / float64(
					dTotal,
				)
			}
			if i == 0 {
				m.CPUPercent = usage
			} else {
				m.PerCore = append(m.PerCore, usage)
			}
		}
	} else if len(current) > 1 {
		m.PerCore = make([]float64, len(current)-1)
	}
	c.prev = current
	c.mu.Unlock()
	loadFields := strings.Fields(
		c.read(c.paths.Proc, "loadavg"),
	)
	for _, field := range loadFields[:min(3, len(loadFields))] {
		value, _ := strconv.ParseFloat(field, 64)
		m.Load = append(m.Load, value)
	}
	mem := map[string]uint64{}
	for _, line := range strings.Split(c.read(c.paths.Proc, "meminfo"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, _ := strconv.ParseUint(fields[1], 10, 64)
		mem[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	m.MemoryTotal = mem["MemTotal"]
	m.MemoryUsed = m.MemoryTotal - mem["MemAvailable"]
	var stat syscall.Statfs_t
	if err := syscall.Statfs(c.paths.Etc, &stat); err == nil {
		m.StorageTotal = stat.Blocks * uint64(stat.Bsize)
		m.StorageUsed = (stat.Blocks - stat.Bavail) * uint64(
			stat.Bsize,
		)
	} else {
		*warnings = append(*warnings, "Root filesystem statistics are unavailable")
	}
	return m
}

func readCPUTimes(raw string) []cpuTime {
	var result []cpuTime
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 ||
			(fields[0] != "cpu" && !strings.HasPrefix(fields[0], "cpu")) {
			continue
		}
		if fields[0] != "cpu" {
			if _, err := strconv.Atoi(strings.TrimPrefix(fields[0], "cpu")); err != nil {
				continue
			}
		}
		var values []uint64
		for _, field := range fields[1:] {
			value, _ := strconv.ParseUint(field, 10, 64)
			values = append(values, value)
		}
		var total uint64
		for _, value := range values {
			total += value
		}
		idle := values[3]
		if len(values) > 4 {
			idle += values[4]
		}
		result = append(
			result,
			cpuTime{idle: idle, total: total},
		)
	}
	return result
}

func (c *Collector) thermals() []Thermal {
	var result = []Thermal{}
	for _, zone := range glob(filepath.Join(c.paths.Sys,
		"class/thermal/thermal_zone*")) {
		raw := clean(read(filepath.Join(zone, "temp")))
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		name := clean(read(filepath.Join(zone, "type")))
		if name == "" {
			name = filepath.Base(zone)
		}
		result = append(
			result,
			Thermal{Name: name, Celsius: value / 1000},
		)
	}
	return result
}

func (c *Collector) networks() []Network {
	addresses := map[string][]string{}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		list, _ := iface.Addrs()
		for _, addr := range list {
			addresses[iface.Name] = append(
				addresses[iface.Name],
				addr.String(),
			)
		}
	}
	var result = []Network{}
	for _, dir := range glob(filepath.Join(c.paths.Sys, "class/net/*")) {
		name := filepath.Base(dir)
		if name == "lo" {
			continue
		}
		n := Network{
			Name: name,
			State: clean(
				read(filepath.Join(dir, "operstate")),
			),
			MAC: clean(
				read(filepath.Join(dir, "address")),
			),
			Addresses: addresses[name],
		}
		n.RXBytes = uintFile(
			filepath.Join(dir, "statistics/rx_bytes"),
		)
		n.TXBytes = uintFile(
			filepath.Join(dir, "statistics/tx_bytes"),
		)
		n.RXErrors = uintFile(
			filepath.Join(dir, "statistics/rx_errors"),
		)
		n.TXErrors = uintFile(
			filepath.Join(dir, "statistics/tx_errors"),
		)
		result = append(result, n)
	}
	sort.Slice(
		result,
		func(i, j int) bool { return result[i].Name < result[j].Name },
	)
	return result
}

func (c *Collector) ports() []ListeningPort {
	var result = []ListeningPort{}
	for _, item := range []struct{ file, protocol string }{{"net/tcp", "tcp4"}, {
		"net/tcp6", "tcp6"}} {
		f, err := os.Open(
			filepath.Join(c.paths.Proc, item.file),
		)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Scan()
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 4 || fields[3] != "0A" {
				continue
			}
			addr, port, ok := decodeAddress(
				fields[1],
				item.protocol == "tcp6",
			)
			if ok {
				result = append(
					result,
					ListeningPort{
						Protocol: item.protocol,
						Address:  addr,
						Port:     port,
					},
				)
			}
		}
		_ = f.Close()
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Port == result[j].Port {
			return result[i].Protocol < result[j].Protocol
		}
		return result[i].Port < result[j].Port
	})
	return result
}

func decodeAddress(
	raw string,
	ipv6 bool,
) (string, int, bool) {
	host, p, ok := strings.Cut(raw, ":")
	if !ok {
		return "", 0, false
	}
	port64, err := strconv.ParseInt(p, 16, 32)
	if err != nil {
		return "", 0, false
	}
	bytes, err := hex.DecodeString(host)
	if err != nil {
		return "", 0, false
	}
	if ipv6 {
		if len(bytes) != 16 {
			return "", 0, false
		}
		for i := 0; i < 16; i += 4 {
			bytes[i], bytes[i+3] = bytes[i+3], bytes[i]
			bytes[i+1], bytes[i+2] = bytes[i+2], bytes[i+1]
		}
	} else {
		if len(bytes) != 4 {
			return "", 0, false
		}
		bytes[0], bytes[3] = bytes[3], bytes[0]
		bytes[1], bytes[2] = bytes[2], bytes[1]
	}
	return net.IP(bytes).String(), int(port64), true
}

func (c *Collector) read(
	root, relative string,
) string {
	return read(filepath.Join(root, relative))
}

func read(
	path string,
) string {
	value, _ := os.ReadFile(path)
	return string(value)
}

func clean(
	value string,
) string {
	return strings.TrimSpace(value)
}

func cleanNUL(
	value string,
) string {
	return strings.TrimSpace(
		strings.TrimRight(value, "\x00"),
	)
}
func splitNUL(value string) []string {
	var out = []string{}
	for _, v := range strings.Split(strings.TrimRight(value, "\x00"), "\x00") {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func glob(
	pattern string,
) []string {
	values, _ := filepath.Glob(pattern)
	return values
}
func uintFile(path string) uint64 {
	value, _ := strconv.ParseUint(clean(read(path)), 10, 64)
	return value
}
