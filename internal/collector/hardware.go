package collector

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func (c *Collector) Hardware() HardwareReport {
	var warnings []string
	system := c.systemInfo(&warnings)
	bsp := c.buildInfo(&warnings)
	cpuFields, cpuTable, cpuName := c.cpuDetails()
	memoryFields := c.memoryDetails()
	storageTable := c.storageDetails()
	thermals := c.thermals()
	networks := c.networks()

	sections := []HardwareSection{
		section(
			"summary",
			"Computer",
			"Summary",
			"overview",
			fields(
				"Operating system",
				first(
					system.OSName,
					bsp.Distro+" "+bsp.DistroVersion,
				),
				"Board",
				system.Model,
				"Processor",
				cpuName,
				"CPU",
				fmt.Sprintf(
					"%d cores · %s",
					system.Cores,
					runtime.GOARCH,
				),
				"RAM",
				fieldValue(memoryFields, "Total"),
				"Kernel",
				system.Kernel,
			),
		),
		section(
			"operating-system",
			"Computer",
			"Operating System",
			"system",
			fields(
				"Distribution",
				system.OSName,
				"Version",
				system.OSVersion,
				"Hostname",
				system.Hostname,
				"Architecture",
				system.Architecture,
				"Kernel",
				system.Kernel,
				"Uptime",
				formatDuration(system.UptimeSec),
				"Command line",
				clean(c.read(c.paths.Proc, "cmdline")),
			),
		),
		sectionWithTables(
			"cpu",
			"Devices",
			"Processor",
			"cpu",
			cpuFields,
			[]HardwareTable{cpuTable, c.cpuCacheDetails()},
		),
		section(
			"memory",
			"Devices",
			"Memory",
			"memory",
			memoryFields,
		),
		sectionWithTables(
			"storage",
			"Devices",
			"Storage",
			"storage",
			nil,
			[]HardwareTable{storageTable},
		),
		sectionWithTables(
			"usb",
			"Devices",
			"USB Devices",
			"usb",
			nil,
			[]HardwareTable{c.usbDetails()},
		),
		sectionWithTables(
			"pci",
			"Devices",
			"PCI Devices",
			"pci",
			nil,
			[]HardwareTable{c.pciDetails()},
		),
		sectionWithTables(
			"i2c",
			"Devices",
			"I²C Devices",
			"i2c",
			nil,
			[]HardwareTable{c.i2cDetails()},
		),
		sectionWithTables(
			"mmc",
			"Devices",
			"MMC Devices",
			"mmc",
			nil,
			[]HardwareTable{c.mmcDetails()},
		),
		sectionWithTables(
			"display",
			"Devices",
			"Display / DRM",
			"display",
			nil,
			[]HardwareTable{c.displayDetails()},
		),
		sectionWithTables(
			"input",
			"Devices",
			"Input Devices",
			"input",
			nil,
			[]HardwareTable{c.inputDetails()},
		),
		sectionWithTables(
			"sensors",
			"Devices",
			"Sensors",
			"sensor",
			nil,
			[]HardwareTable{thermalTable(thermals)},
		),
		sectionWithTables(
			"network-interfaces",
			"Network",
			"Interfaces",
			"network",
			nil,
			[]HardwareTable{networkTable(networks)},
		),
		sectionWithTables(
			"listening-ports",
			"Network",
			"Listening Ports",
			"ports",
			nil,
			[]HardwareTable{portTable(c.ports())},
		),
		sectionWithTables(
			"filesystems",
			"System",
			"Filesystems",
			"filesystem",
			nil,
			[]HardwareTable{c.filesystemDetails()},
		),
		sectionWithTables(
			"kernel-modules",
			"System",
			"Kernel Modules",
			"module",
			nil,
			[]HardwareTable{c.kernelModuleDetails()},
		),
		section(
			"device-tree",
			"System",
			"Device Tree",
			"tree",
			fields(
				"Model",
				system.Model,
				"Compatibility",
				strings.Join(system.Compatible, " · "),
				"Serial",
				cleanNUL(
					c.read(
						c.paths.Sys,
						"firmware/devicetree/base/serial-number",
					),
				),
				"Boot arguments",
				cleanNUL(
					c.read(
						c.paths.Sys,
						"firmware/devicetree/base/chosen/bootargs",
					),
				),
			),
		),
	}
	sections = append(
		sections,
		c.extendedHardwareSections(system)...)
	return HardwareReport{
		Timestamp: time.Now().UTC(),
		Sections:  sections,
	}
}

func section(
	id, group, label, icon string,
	values []HardwareField,
) HardwareSection {
	return HardwareSection{
		ID:     id,
		Group:  group,
		Label:  label,
		Icon:   icon,
		Fields: nonNilFields(values),
		Tables: []HardwareTable{},
	}
}

func sectionWithTables(
	id, group, label, icon string,
	values []HardwareField,
	tables []HardwareTable,
) HardwareSection {
	return HardwareSection{
		ID:     id,
		Group:  group,
		Label:  label,
		Icon:   icon,
		Fields: nonNilFields(values),
		Tables: tables,
	}
}

func fields(values ...string) []HardwareField {
	result := []HardwareField{}
	for i := 0; i+1 < len(values); i += 2 {
		if strings.TrimSpace(values[i+1]) != "" {
			result = append(
				result,
				HardwareField{
					Label: values[i],
					Value: values[i+1],
				},
			)
		}
	}
	return result
}

func nonNilFields(values []HardwareField) []HardwareField {
	if values == nil {
		return []HardwareField{}
	}
	return values
}

func fieldValue(
	values []HardwareField,
	label string,
) string {
	for _, value := range values {
		if value.Label == label {
			return value.Value
		}
	}
	return ""
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "Not available"
}

func (c *Collector) cpuDetails() ([]HardwareField, HardwareTable, string) {
	blocks := strings.Split(
		strings.TrimSpace(c.read(c.paths.Proc, "cpuinfo")),
		"\n\n",
	)
	rows := [][]string{}
	var firstCPU map[string]string
	for _, block := range blocks {
		values := parseColonBlock(block)
		if values["processor"] == "" {
			continue
		}
		if firstCPU == nil {
			firstCPU = values
		}
		model := cpuDisplayName(values)
		current := formatCPUFrequency(
			c.read(
				c.paths.Sys,
				"devices/system/cpu/cpu"+values["processor"]+"/cpufreq/scaling_cur_freq",
			),
		)
		maximum := formatCPUFrequency(
			c.read(
				c.paths.Sys,
				"devices/system/cpu/cpu"+values["processor"]+"/cpufreq/cpuinfo_max_freq",
			),
		)
		rows = append(
			rows,
			[]string{
				values["processor"],
				model,
				current,
				maximum,
				values["BogoMIPS"],
			},
		)
	}
	if firstCPU == nil {
		firstCPU = parseColonBlock(
			c.read(c.paths.Proc, "cpuinfo"),
		)
	}
	name := cpuDisplayName(firstCPU)
	values := fields(
		"Model", name,
		"Implementer", firstCPU["CPU implementer"],
		"Architecture", firstCPU["CPU architecture"],
		"Variant", firstCPU["CPU variant"],
		"Part number", firstCPU["CPU part"],
		"Revision", firstCPU["CPU revision"],
		"Features", firstCPU["Features"],
	)
	return values, HardwareTable{
		Title: "Logical cores",
		Columns: []string{
			"Core",
			"Model",
			"Current clock",
			"Maximum clock",
			"BogoMIPS",
		},
		Rows: rows,
	}, name
}

func parseColonBlock(raw string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			result[strings.TrimSpace(key)] = strings.TrimSpace(
				value,
			)
		}
	}
	return result
}

func (c *Collector) memoryDetails() []HardwareField {
	result := []HardwareField{}
	for _, line := range strings.Split(c.read(c.paths.Proc, "meminfo"), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		parts := strings.Fields(value)
		if len(parts) == 0 {
			continue
		}
		n, err := strconv.ParseUint(parts[0], 10, 64)
		if err == nil &&
			(len(parts) == 1 || parts[1] == "kB") {
			value = humanBytes(n * 1024)
		}
		labels := map[string]string{
			"MemTotal":     "Total",
			"MemAvailable": "Available",
			"MemFree":      "Free",
			"Buffers":      "Buffers",
			"Cached":       "Cache",
			"SwapTotal":    "Swap total",
			"SwapFree":     "Swap free",
		}
		if label := labels[key]; label != "" {
			result = append(
				result,
				HardwareField{
					Label: label,
					Value: strings.TrimSpace(value),
				},
			)
		}
	}
	return result
}

func (c *Collector) storageDetails() HardwareTable {
	rows := [][]string{}
	for _, device := range glob(filepath.Join(c.paths.Sys, "class/block/*")) {
		name := filepath.Base(device)
		if strings.HasPrefix(name, "loop") ||
			strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "dm-") {
			continue
		}
		sectors, _ := strconv.ParseUint(
			clean(read(filepath.Join(device, "size"))),
			10,
			64,
		)
		rows = append(
			rows,
			[]string{
				name,
				clean(
					read(
						filepath.Join(
							device,
							"device/model",
						),
					),
				),
				humanBytes(sectors * 512),
				yesNo(
					read(
						filepath.Join(device, "removable"),
					),
				),
				yesNo(
					read(
						filepath.Join(
							device,
							"queue/rotational",
						),
					),
				),
			},
		)
	}
	return HardwareTable{
		Title: "Block devices",
		Columns: []string{
			"Device",
			"Model",
			"Size",
			"Removable",
			"Rotational",
		},
		Rows: rows,
	}
}

func (c *Collector) usbDetails() HardwareTable {
	rows := [][]string{}
	for _, device := range glob(filepath.Join(c.paths.Sys, "bus/usb/devices/*")) {
		vendor, product := clean(
			read(filepath.Join(device, "idVendor")),
		), clean(
			read(filepath.Join(device, "idProduct")),
		)
		if vendor == "" {
			continue
		}
		rows = append(
			rows,
			[]string{
				filepath.Base(device),
				vendor + ":" + product,
				clean(
					read(
						filepath.Join(
							device,
							"manufacturer",
						),
					),
				),
				clean(
					read(filepath.Join(device, "product")),
				),
				clean(
					read(filepath.Join(device, "speed")),
				) + " Mb/s",
			},
		)
	}
	return HardwareTable{
		Title: "USB bus",
		Columns: []string{
			"Port",
			"VID:PID",
			"Manufacturer",
			"Product",
			"Speed",
		},
		Rows: rows,
	}
}

func (c *Collector) pciDetails() HardwareTable {
	rows := [][]string{}
	for _, device := range glob(filepath.Join(c.paths.Sys, "bus/pci/devices/*")) {
		rows = append(
			rows,
			[]string{
				filepath.Base(device),
				clean(
					read(filepath.Join(device, "vendor")),
				),
				clean(
					read(filepath.Join(device, "device")),
				),
				clean(read(filepath.Join(device, "class"))),
			},
		)
	}
	return HardwareTable{
		Title: "PCI bus",
		Columns: []string{
			"Address",
			"Vendor",
			"Device",
			"Class",
		},
		Rows: rows,
	}
}

func (c *Collector) i2cDetails() HardwareTable {
	rows := [][]string{}
	for _, device := range glob(filepath.Join(c.paths.Sys, "bus/i2c/devices/*")) {
		name := clean(read(filepath.Join(device, "name")))
		if name != "" {
			rows = append(
				rows,
				[]string{
					filepath.Base(device),
					name,
					clean(
						read(
							filepath.Join(
								device,
								"modalias",
							),
						),
					),
				},
			)
		}
	}
	return HardwareTable{
		Title: "I²C bus",
		Columns: []string{
			"Address",
			"Name",
			"Driver / modalias",
		},
		Rows: rows,
	}
}

func (c *Collector) mmcDetails() HardwareTable {
	rows := [][]string{}
	for _, device := range glob(filepath.Join(c.paths.Sys, "bus/mmc/devices/*")) {
		rows = append(
			rows,
			[]string{
				filepath.Base(device),
				clean(read(filepath.Join(device, "name"))),
				clean(read(filepath.Join(device, "type"))),
				clean(
					read(filepath.Join(device, "manfid")),
				),
				clean(
					read(filepath.Join(device, "serial")),
				),
			},
		)
	}
	return HardwareTable{
		Title: "Cards and eMMC",
		Columns: []string{
			"Device",
			"Name",
			"Type",
			"Manufacturer",
			"Serial",
		},
		Rows: rows,
	}
}

func (c *Collector) displayDetails() HardwareTable {
	rows := [][]string{}
	for _, connector := range glob(filepath.Join(c.paths.Sys,
		"class/drm/card*-*")) {
		status := clean(
			read(filepath.Join(connector, "status")),
		)
		if status == "" {
			continue
		}
		modes := strings.Join(
			strings.Fields(
				read(filepath.Join(connector, "modes")),
			),
			", ",
		)
		rows = append(
			rows,
			[]string{
				filepath.Base(connector),
				status,
				modes,
			},
		)
	}
	return HardwareTable{
		Title:   "DRM connectors",
		Columns: []string{"Connector", "Status", "Modes"},
		Rows:    rows,
	}
}

func (c *Collector) inputDetails() HardwareTable {
	rows := [][]string{}
	for _, block := range strings.Split(c.read(c.paths.Proc,
		"bus/input/devices"), "\n\n") {
		values := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			if len(line) > 3 && line[1] == ':' {
				key, value, ok := strings.Cut(line[3:], "=")
				if ok {
					values[key] = strings.Trim(
						strings.TrimSpace(value),
						"\"",
					)
				}
			}
		}
		if values["Name"] != "" {
			rows = append(
				rows,
				[]string{
					values["Name"],
					values["Phys"],
					values["Handlers"],
				},
			)
		}
	}
	return HardwareTable{
		Title: "Input subsystem",
		Columns: []string{
			"Name",
			"Physical path",
			"Handlers",
		},
		Rows: rows,
	}
}

func (c *Collector) filesystemDetails() HardwareTable {
	rows := [][]string{}
	for _, line := range strings.Split(c.read(c.paths.Proc, "mounts"), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 4 {
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
		Title: "Mount points",
		Columns: []string{
			"Device",
			"Mount point",
			"Type",
			"Options",
		},
		Rows: rows,
	}
}

func (c *Collector) kernelModuleDetails() HardwareTable {
	rows := [][]string{}
	for _, line := range strings.Split(c.read(c.paths.Proc, "modules"), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 6 {
			rows = append(
				rows,
				[]string{
					parts[0],
					parts[1],
					parts[2],
					strings.TrimSuffix(parts[3], ","),
					parts[4],
				},
			)
		}
	}
	return HardwareTable{
		Title: "Loaded modules",
		Columns: []string{
			"Module",
			"Size",
			"Usage",
			"Dependencies",
			"Status",
		},
		Rows: rows,
	}
}

func thermalTable(items []Thermal) HardwareTable {
	rows := [][]string{}
	for _, item := range items {
		rows = append(
			rows,
			[]string{
				item.Name,
				fmt.Sprintf("%.1f °C", item.Celsius),
			},
		)
	}
	return HardwareTable{
		Title:   "Thermal sensors",
		Columns: []string{"Sensor", "Temperature"},
		Rows:    rows,
	}
}

func networkTable(items []Network) HardwareTable {
	rows := [][]string{}
	for _, item := range items {
		rows = append(
			rows,
			[]string{
				item.Name,
				item.State,
				item.MAC,
				strings.Join(item.Addresses, ", "),
				humanBytes(item.RXBytes),
				humanBytes(item.TXBytes),
			},
		)
	}
	return HardwareTable{
		Title: "Network interfaces",
		Columns: []string{
			"Interface",
			"Status",
			"MAC",
			"Addresses",
			"Received",
			"Sent",
		},
		Rows: rows,
	}
}

func portTable(items []ListeningPort) HardwareTable {
	rows := [][]string{}
	for _, item := range items {
		rows = append(
			rows,
			[]string{
				item.Protocol,
				item.Address,
				strconv.Itoa(item.Port),
			},
		)
	}
	return HardwareTable{
		Title:   "TCP sockets",
		Columns: []string{"Protocol", "Address", "Port"},
		Rows:    rows,
	}
}

func formatDuration(seconds float64) string {
	duration := time.Duration(seconds) * time.Second
	return fmt.Sprintf(
		"%dd %02dh %02dm",
		int(duration.Hours())/24,
		int(duration.Hours())%24,
		int(duration.Minutes())%60,
	)
}

func humanBytes(value uint64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	n := float64(value)
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", value, units[i])
	}
	return fmt.Sprintf("%.1f %s", n, units[i])
}

func yesNo(raw string) string {
	if clean(raw) == "1" {
		return "Yes"
	}
	return "No"
}
