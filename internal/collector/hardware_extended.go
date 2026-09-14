package collector

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (c *Collector) extendedHardwareSections(
	system SystemInfo,
) []HardwareSection {
	return []HardwareSection{
		section(
			"boot",
			"Computer",
			"Boot",
			"boot",
			c.bootDetails(system),
		),
		sectionWithTables(
			"languages",
			"Computer",
			"Languages and Locale",
			"language",
			nil,
			[]HardwareTable{c.languageDetails()},
		),
		sectionWithTables(
			"environment",
			"Computer",
			"Environment",
			"environment",
			nil,
			[]HardwareTable{c.environmentDetails()},
		),
		sectionWithTables(
			"users",
			"Computer",
			"Users",
			"users",
			nil,
			[]HardwareTable{c.userDetails()},
		),
		sectionWithTables(
			"groups",
			"Computer",
			"Groups",
			"groups",
			nil,
			[]HardwareTable{c.groupDetails()},
		),
		sectionWithTables(
			"power",
			"Devices",
			"Power / Battery",
			"power",
			nil,
			[]HardwareTable{c.powerDetails()},
		),
		sectionWithTables(
			"resources",
			"Devices",
			"Resources",
			"resource",
			nil,
			c.resourceDetails(),
		),
		sectionWithTables(
			"connections",
			"Network",
			"IP Connections",
			"connections",
			nil,
			[]HardwareTable{c.connectionDetails()},
		),
		sectionWithTables(
			"routing",
			"Network",
			"Routing Table",
			"route",
			nil,
			[]HardwareTable{c.routingDetails()},
		),
		sectionWithTables(
			"arp",
			"Network",
			"ARP Table",
			"arp",
			nil,
			[]HardwareTable{c.arpDetails()},
		),
		sectionWithTables(
			"dns",
			"Network",
			"DNS Servers",
			"dns",
			nil,
			[]HardwareTable{c.dnsDetails()},
		),
		sectionWithTables(
			"network-stats",
			"Network",
			"Statistics",
			"statistics",
			nil,
			[]HardwareTable{c.networkStatistics()},
		),
		sectionWithTables(
			"shared",
			"Network",
			"Shared Directories",
			"shared",
			nil,
			[]HardwareTable{c.sharedDirectories()},
		),
	}
}

func (c *Collector) bootDetails(
	system SystemInfo,
) []HardwareField {
	var bootTime string
	for _, line := range strings.Split(c.read(c.paths.Proc, "stat"), "\n") {
		if strings.HasPrefix(line, "btime ") {
			seconds, _ := strconv.ParseInt(
				strings.TrimSpace(
					strings.TrimPrefix(line, "btime "),
				),
				10,
				64,
			)
			if seconds > 0 {
				bootTime = time.Unix(seconds, 0).
					Format(time.RFC3339)
			}
		}
	}
	return fields(
		"Current boot",
		bootTime,
		"Uptime",
		formatDuration(system.UptimeSec),
		"Kernel",
		system.Kernel,
		"Boot arguments",
		clean(c.read(c.paths.Proc, "cmdline")),
	)
}

func (c *Collector) languageDetails() HardwareTable {
	rows := [][]string{}
	for _, entry := range parseEnvironment(c.read(c.paths.Etc, "environment")) {
		if entry[0] == "LANG" || entry[0] == "LANGUAGE" ||
			strings.HasPrefix(entry[0], "LC_") {
			rows = append(rows, entry)
		}
	}
	return HardwareTable{
		Title:   "Locale configuration",
		Columns: []string{"Variable", "Value"},
		Rows:    rows,
	}
}

func (c *Collector) environmentDetails() HardwareTable {
	rows := [][]string{}
	for _, entry := range parseEnvironment(c.read(c.paths.Etc, "environment")) {
		upper := strings.ToUpper(entry[0])
		if strings.Contains(upper, "PASSWORD") ||
			strings.Contains(upper, "TOKEN") ||
			strings.Contains(upper, "SECRET") ||
			strings.Contains(upper, "CREDENTIAL") ||
			strings.HasSuffix(upper, "_KEY") {
			entry[1] = "[redacted]"
		}
		rows = append(rows, entry)
	}
	return HardwareTable{
		Title:   "Safe persistent variables",
		Columns: []string{"Variable", "Value"},
		Rows:    rows,
	}
}

func parseEnvironment(raw string) [][]string {
	rows := [][]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			rows = append(
				rows,
				[]string{
					strings.TrimSpace(key),
					strings.Trim(
						strings.TrimSpace(value),
						"\"",
					),
				},
			)
		}
	}
	return rows
}

func (c *Collector) userDetails() HardwareTable {
	rows := [][]string{}
	for _, line := range strings.Split(c.read(c.paths.Etc, "passwd"), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 7 {
			rows = append(
				rows,
				[]string{
					parts[0],
					parts[2],
					parts[3],
					strings.Split(parts[4], ",")[0],
					parts[5],
					parts[6],
				},
			)
		}
	}
	return HardwareTable{
		Title: "Local accounts",
		Columns: []string{
			"User",
			"UID",
			"GID",
			"Name",
			"Home",
			"Shell",
		},
		Rows: rows,
	}
}

func (c *Collector) groupDetails() HardwareTable {
	rows := [][]string{}
	for _, line := range strings.Split(c.read(c.paths.Etc, "group"), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 4 {
			rows = append(
				rows,
				[]string{parts[0], parts[2], parts[3]},
			)
		}
	}
	return HardwareTable{
		Title:   "Local groups",
		Columns: []string{"Group", "GID", "Members"},
		Rows:    rows,
	}
}

func (c *Collector) powerDetails() HardwareTable {
	rows := [][]string{}
	properties := []struct{ file, label string }{
		{"type", "Type"},
		{"status", "Status"},
		{"capacity", "Capacity (%)"},
		{"technology", "Technology"},
		{"manufacturer", "Manufacturer"},
		{"model_name", "Model"},
		{"serial_number", "Serial"},
		{"voltage_now", "Voltage (µV)"},
		{"current_now", "Current (µA)"},
	}
	for _, supply := range glob(filepath.Join(c.paths.Sys,
		"class/power_supply/*")) {
		for _, property := range properties {
			if value := clean(read(filepath.Join(supply, property.file))); value != "" {
				rows = append(
					rows,
					[]string{
						filepath.Base(supply),
						property.label,
						value,
					},
				)
			}
		}
	}
	return HardwareTable{
		Title:   "Power supplies",
		Columns: []string{"Device", "Property", "Value"},
		Rows:    rows,
	}
}

func (c *Collector) resourceDetails() []HardwareTable {
	return []HardwareTable{
		resourceTable(
			"Interrupts",
			c.read(c.paths.Proc, "interrupts"),
			"IRQ",
			"Controller and device",
		),
		resourceTable(
			"Memory regions",
			c.read(c.paths.Proc, "iomem"),
			"Address",
			"Resource",
		),
		resourceTable(
			"I/O ports",
			c.read(c.paths.Proc, "ioports"),
			"Address",
			"Resource",
		),
	}
}

func resourceTable(
	title, raw, firstColumn, secondColumn string,
) HardwareTable {
	rows := [][]string{}
	for _, line := range strings.Split(raw, "\n") {
		left, right, ok := strings.Cut(
			strings.TrimSpace(line),
			":",
		)
		if ok && strings.TrimSpace(left) != "" {
			rows = append(
				rows,
				[]string{
					strings.TrimSpace(left),
					strings.TrimSpace(right),
				},
			)
		}
	}
	return HardwareTable{
		Title:   title,
		Columns: []string{firstColumn, secondColumn},
		Rows:    rows,
	}
}

func (c *Collector) connectionDetails() HardwareTable {
	rows := [][]string{}
	files := []struct {
		path, protocol string
		ipv6           bool
	}{{"net/tcp", "tcp", false}, {"net/tcp6", "tcp6", true}, {"net/udp", "udp",
		false}, {"net/udp6", "udp6", true}}
	states := map[string]string{
		"01": "ESTABLISHED",
		"02": "SYN_SENT",
		"03": "SYN_RECV",
		"04": "FIN_WAIT1",
		"05": "FIN_WAIT2",
		"06": "TIME_WAIT",
		"07": "CLOSE",
		"08": "CLOSE_WAIT",
		"09": "LAST_ACK",
		"0A": "LISTEN",
		"0B": "CLOSING",
	}
	for _, source := range files {
		lines := strings.Split(
			c.read(c.paths.Proc, source.path),
			"\n",
		)
		for _, line := range lines[1:] {
			parts := strings.Fields(line)
			if len(parts) < 4 {
				continue
			}
			localAddress, localPort, localOK := decodeAddress(
				parts[1],
				source.ipv6,
			)
			remoteAddress, remotePort, remoteOK := decodeAddress(
				parts[2],
				source.ipv6,
			)
			if localOK && remoteOK {
				rows = append(
					rows,
					[]string{
						fmt.Sprintf(
							"%s:%d",
							localAddress,
							localPort,
						),
						fmt.Sprintf(
							"%s:%d",
							remoteAddress,
							remotePort,
						),
						states[parts[3]],
						source.protocol,
					},
				)
			}
		}
	}
	return HardwareTable{
		Title: "Host sockets",
		Columns: []string{
			"Local address",
			"Remote address",
			"State",
			"Protocol",
		},
		Rows: rows,
	}
}

func (c *Collector) routingDetails() HardwareTable {
	rows := [][]string{}
	lines := strings.Split(
		c.read(c.paths.Proc, "net/route"),
		"\n",
	)
	for _, line := range lines[1:] {
		parts := strings.Fields(line)
		if len(parts) >= 8 {
			rows = append(
				rows,
				[]string{
					hexIPv4(parts[1]),
					hexIPv4(parts[2]),
					parts[3],
					hexIPv4(parts[7]),
					parts[0],
					parts[6],
				},
			)
		}
	}
	return HardwareTable{
		Title: "IPv4 routes",
		Columns: []string{
			"Destination",
			"Gateway",
			"Flags",
			"Mask",
			"Interface",
			"Metric",
		},
		Rows: rows,
	}
}

func hexIPv4(value string) string {
	n, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return value
	}
	return fmt.Sprintf(
		"%d.%d.%d.%d",
		byte(n),
		byte(n>>8),
		byte(n>>16),
		byte(n>>24),
	)
}

func (c *Collector) arpDetails() HardwareTable {
	rows := [][]string{}
	lines := strings.Split(
		c.read(c.paths.Proc, "net/arp"),
		"\n",
	)
	for _, line := range lines[1:] {
		parts := strings.Fields(line)
		if len(parts) >= 6 {
			rows = append(
				rows,
				[]string{
					parts[0],
					parts[3],
					parts[5],
					parts[2],
				},
			)
		}
	}
	return HardwareTable{
		Title: "IPv4 neighbors",
		Columns: []string{
			"IP address",
			"MAC address",
			"Interface",
			"Flags",
		},
		Rows: rows,
	}
}

func (c *Collector) dnsDetails() HardwareTable {
	raw := c.read(c.paths.Etc, "resolv.conf")
	if raw == "" {
		raw = read(
			filepath.Join(
				filepath.Dir(c.paths.Etc),
				"resolv.conf",
			),
		)
	}
	rows := [][]string{}
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 &&
			(parts[0] == "nameserver" || parts[0] == "search" || parts[0] == "domain") {
			rows = append(
				rows,
				[]string{
					parts[0],
					strings.Join(parts[1:], " "),
				},
			)
		}
	}
	return HardwareTable{
		Title:   "Resolver",
		Columns: []string{"Type", "Value"},
		Rows:    rows,
	}
}

func (c *Collector) networkStatistics() HardwareTable {
	rows := [][]string{}
	lines := strings.Split(
		c.read(c.paths.Proc, "net/snmp"),
		"\n",
	)
	for i := 0; i+1 < len(lines); i += 2 {
		head, values := strings.Fields(
			lines[i],
		), strings.Fields(
			lines[i+1],
		)
		if len(head) < 2 || len(values) < 2 ||
			head[0] != values[0] {
			continue
		}
		protocol := strings.TrimSuffix(head[0], ":")
		for column := 1; column < len(head) && column < len(values); column++ {
			rows = append(
				rows,
				[]string{
					protocol,
					head[column],
					values[column],
				},
			)
		}
	}
	return HardwareTable{
		Title:   "Protocol counters",
		Columns: []string{"Protocol", "Metric", "Value"},
		Rows:    rows,
	}
}

func (c *Collector) sharedDirectories() HardwareTable {
	rows := [][]string{}
	for _, line := range strings.Split(c.read(c.paths.Proc, "mounts"), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 4 &&
			(strings.Contains(parts[2], "nfs") || parts[2] == "cifs" ||
				parts[2] == "smb3" || parts[2] == "fuse.sshfs") {
			rows = append(
				rows,
				[]string{
					parts[0],
					parts[1],
					parts[2],
					parts[3],
				},
			)
		}
	}
	return HardwareTable{
		Title: "Remote mounts",
		Columns: []string{
			"Source",
			"Mount point",
			"Type",
			"Options",
		},
		Rows: rows,
	}
}
