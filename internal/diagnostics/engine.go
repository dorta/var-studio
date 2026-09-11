package diagnostics

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dorta/var-studio/internal/collector"
	"github.com/dorta/var-studio/internal/support"
)

type Engine struct {
	paths collector.Paths
}

func New(
	paths collector.Paths,
) *Engine {
	return &Engine{paths: paths}
}

func (e *Engine) Build(
	snapshot collector.Snapshot,
	kernel support.KernelLog,
	events []Event,
) Dashboard {
	capabilities := e.capabilities(snapshot)
	return Dashboard{
		Health:       e.health(snapshot, kernel),
		Events:       events,
		Capabilities: capabilities,
		Guides:       e.guides(snapshot, kernel),
		Actions:      e.actions(capabilities),
	}
}

func (e *Engine) health(
	snapshot collector.Snapshot,
	kernel support.KernelLog,
) Health {
	checks := make([]Check, 0, 8)
	add := func(id, name, status, summary, evidence string, severity Severity) {
		checks = append(
			checks,
			Check{
				ID:       id,
				Name:     name,
				Status:   status,
				Summary:  summary,
				Evidence: evidence,
				Severity: severity,
			},
		)
	}
	maxTemp := 0.0
	hottest := ""
	for _, sensor := range snapshot.Thermals {
		if sensor.Celsius > maxTemp {
			maxTemp, hottest = sensor.Celsius, sensor.Name
		}
	}
	switch {
	case maxTemp >= 95:
		add(
			"thermal",
			"Thermal state",
			"critical",
			"Temperature is above the critical threshold.",
			fmt.Sprintf("%s %.1f°C", hottest, maxTemp),
			SeverityCritical,
		)
	case maxTemp >= 85:
		add(
			"thermal",
			"Thermal state",
			"warning",
			"Temperature is approaching the operating limit.",
			fmt.Sprintf("%s %.1f°C", hottest, maxTemp),
			SeverityWarning,
		)
	case maxTemp > 0:
		add(
			"thermal",
			"Thermal state",
			"passed",
			"All reported sensors are within the configured "+"threshold.",
			fmt.Sprintf(
				"Hottest: %s %.1f°C",
				hottest,
				maxTemp,
			),
			SeverityInfo,
		)
	default:
		add(
			"thermal",
			"Thermal state",
			"unavailable",
			"No thermal zones were reported by this image.",
			"",
			SeverityWarning,
		)
	}
	storage := percent(
		snapshot.Metrics.StorageUsed,
		snapshot.Metrics.StorageTotal,
	)
	switch {
	case storage >= 95:
		add(
			"storage",
			"Storage capacity",
			"critical",
			"Root storage is almost full.",
			fmt.Sprintf("%.1f%% used", storage),
			SeverityCritical,
		)
	case storage >= 85:
		add(
			"storage",
			"Storage capacity",
			"warning",
			"Root storage usage is high.",
			fmt.Sprintf("%.1f%% used", storage),
			SeverityWarning,
		)
	default:
		add(
			"storage",
			"Storage capacity",
			"passed",
			"Root storage has sufficient free space.",
			fmt.Sprintf("%.1f%% used", storage),
			SeverityInfo,
		)
	}
	memory := percent(
		snapshot.Metrics.MemoryUsed,
		snapshot.Metrics.MemoryTotal,
	)
	if memory >= 97 {
		add(
			"memory",
			"Memory pressure",
			"critical",
			"Memory usage is critically high.",
			fmt.Sprintf("%.1f%% used", memory),
			SeverityCritical,
		)
	} else if memory >= 90 {
		add("memory", "Memory pressure", "warning", "Memory usage is high.",
			fmt.Sprintf("%.1f%% used", memory), SeverityWarning)
	} else {
		add("memory", "Memory pressure", "passed",
			"Memory usage is within the configured threshold.", fmt.Sprintf(
				"%.1f%% used", memory), SeverityInfo)
	}
	up := 0
	errors := uint64(0)
	for _, network := range snapshot.Networks {
		if network.State == "up" && network.Name != "lo" {
			up++
		}
		errors += network.RXErrors + network.TXErrors
	}
	if up == 0 {
		add(
			"network",
			"Network connectivity",
			"warning",
			"No non-loopback network interface is up.",
			"",
			SeverityWarning,
		)
	} else if errors > 0 {
		add("network", "Network connectivity", "warning",
			"Network interfaces are active but errors were "+"recorded.", fmt.Sprintf(
				"%d interface errors", errors), SeverityWarning)
	} else {
		add("network", "Network connectivity", "passed",
			"At least one network interface is active without "+"errors.", fmt.Sprintf(
				"%d active interface(s)", up), SeverityInfo)
	}
	recentKernelErrors, historicalKernelErrors := 0, 0
	for _, message := range kernel.Messages {
		if message.Level != "EMERGENCY" &&
			message.Level != "ALERT" &&
			message.Level != "CRITICAL" &&
			message.Level != "ERROR" {
			continue
		}
		historicalKernelErrors++
		age := snapshot.System.UptimeSec - message.UptimeSeconds
		if age >= 0 && age <= 300 {
			recentKernelErrors++
		}
	}
	if recentKernelErrors > 0 {
		add(
			"kernel",
			"Kernel messages",
			"warning",
			"Recent error-level kernel messages were "+"captured.",
			fmt.Sprintf(
				"%d error(s) in the last 5 minutes",
				recentKernelErrors,
			),
			SeverityWarning,
		)
	} else if kernel.Available {
		add("kernel", "Kernel messages", "passed",
			"No recent error-level kernel messages were "+"captured.", fmt.Sprintf(
				"%d retained historical error(s) · %d messages "+"inspected",
					historicalKernelErrors, len(kernel.Messages)), SeverityInfo)
	} else {
		add("kernel", "Kernel messages", "unavailable",
			"Persistent kernel collection is unavailable.", "", SeverityWarning)
	}
	taint := strings.TrimSpace(
		read(
			filepath.Join(
				e.paths.Proc,
				"sys/kernel/tainted",
			),
		),
	)
	taintValue, _ := strconv.ParseUint(taint, 10, 64)
	severeTaintMask := uint64(
		1<<4 | 1<<5 | 1<<7 | 1<<9 | 1<<14,
	)
	switch {
	case taint == "" || taintValue == 0:
		add(
			"taint",
			"Kernel taint disclosure",
			"passed",
			"The kernel reports no taint flags.",
			"tainted=0",
			SeverityInfo,
		)
	case taintValue&severeTaintMask != 0:
		add(
			"taint",
			"Kernel taint disclosure",
			"warning",
			"The kernel reports a severe runtime taint flag.",
			"tainted="+taint,
			SeverityWarning,
		)
	default:
		add(
			"taint",
			"Kernel taint disclosure",
			"passed",
			"Only module or platform disclosure flags are "+"present.",
			"tainted="+taint,
			SeverityInfo,
		)
	}
	pstore, _ := filepath.Glob(
		filepath.Join(e.paths.Sys, "fs/pstore/*"),
	)
	if len(pstore) == 0 {
		add(
			"pstore",
			"Previous crashes",
			"passed",
			"No persistent crash records were found.",
			"",
			SeverityInfo,
		)
	} else {
		add("pstore", "Previous crashes", "warning",
			"Persistent crash records are present.", fmt.Sprintf("%d pstore record(s)",
				len(pstore)), SeverityWarning)
	}
	health := Health{
		Status:      "healthy",
		EvaluatedAt: time.Now().UTC(),
		Checks:      checks,
	}
	for _, check := range checks {
		switch check.Status {
		case "critical":
			health.Critical++
		case "warning", "unavailable":
			health.Warnings++
		default:
			health.Passed++
		}
	}
	if health.Critical > 0 {
		health.Status, health.Summary = "critical", fmt.Sprintf(
			"%d critical issue(s) require attention",
			health.Critical,
		)
	} else if health.Warnings > 0 {
		health.Status, health.Summary = "warning", fmt.Sprintf(
			"%d warning(s) detected", health.Warnings)
	} else {
		health.Summary = fmt.Sprintf("%d checks passed", health.Passed)
	}
	return health
}

func (e *Engine) capabilities(
	snapshot collector.Snapshot,
) []Capability {
	type definition struct {
		id, name, category, pattern, availableDetail, missingDetail string
	}
	definitions := []definition{
		{
			"gpio",
			"GPIO controllers",
			"Buses & I/O",
			filepath.Join(
				e.paths.Sys,
				"class/gpio/gpiochip*",
			),
			"GPIO controllers exposed by the kernel",
			"No GPIO controllers are exposed",
		},
		{
			"i2c",
			"I²C buses",
			"Buses & I/O",
			filepath.Join(
				e.paths.Sys,
				"class/i2c-adapter/i2c-*",
			),
			"I²C adapters available",
			"No I²C adapters are exposed",
		},
		{
			"mmc",
			"MMC / SD hosts",
			"Storage",
			filepath.Join(
				e.paths.Sys,
				"class/mmc_host/mmc*",
			),
			"MMC hosts available",
			"No MMC hosts are exposed",
		},
		{
			"drm",
			"Display / DRM connectors",
			"Multimedia",
			filepath.Join(e.paths.Sys, "class/drm/card*-*"),
			"DRM connectors detected",
			"No DRM connectors are exposed",
		},
		{
			"camera",
			"V4L2 devices",
			"Multimedia",
			filepath.Join(
				e.paths.Sys,
				"class/video4linux/video*",
			),
			"Video devices detected",
			"No V4L2 devices are exposed",
		},
		{
			"usb",
			"USB devices",
			"Connectivity",
			filepath.Join(
				e.paths.Sys,
				"bus/usb/devices/*/product",
			),
			"USB products enumerated",
			"No USB product descriptors are visible",
		},
		{
			"pwm",
			"PWM controllers",
			"Buses & I/O",
			filepath.Join(
				e.paths.Sys,
				"class/pwm/pwmchip*",
			),
			"PWM controllers exposed by the kernel",
			"No PWM controllers are exposed",
		},
		{
			"remoteproc",
			"Remote processors",
			"Compute",
			filepath.Join(
				e.paths.Sys,
				"class/remoteproc/remoteproc*",
			),
			"Remote processors detected",
			"No remote processors are exposed",
		},
		{
			"gpu",
			"GPU engines",
			"Compute",
			filepath.Join(
				e.paths.Sys,
				"class/drm/card[0-9]*",
			),
			"GPU/DRM devices detected",
			"No GPU/DRM device is exposed",
		},
		{
			"npu",
			"NPU / ML accelerators",
			"Compute",
			filepath.Join(e.paths.Sys, "class/misc/*npu*"),
			"ML accelerator devices detected",
			"No supported NPU interface is exposed",
		},
		{
			"audio",
			"ALSA audio devices",
			"Multimedia",
			filepath.Join(e.paths.Sys, "class/sound/card*"),
			"ALSA cards detected",
			"No ALSA cards are exposed",
		},
		{
			"bluetooth",
			"Bluetooth controllers",
			"Connectivity",
			filepath.Join(
				e.paths.Sys,
				"class/bluetooth/hci*",
			),
			"Bluetooth controllers detected",
			"No Bluetooth controller is exposed",
		},
		{
			"rtc",
			"Real-time clocks",
			"Platform",
			filepath.Join(e.paths.Sys, "class/rtc/rtc*"),
			"RTC devices detected",
			"No RTC device is exposed",
		},
		{
			"spi",
			"SPI controllers",
			"Buses & I/O",
			filepath.Join(
				e.paths.Sys,
				"class/spi_master/spi*",
			),
			"SPI masters detected",
			"No SPI master is exposed",
		},
		{
			"pcie",
			"PCIe devices",
			"Connectivity",
			filepath.Join(e.paths.Sys, "bus/pci/devices/*"),
			"PCIe devices detected",
			"No PCIe devices are enumerated",
		},
		{
			"tpm",
			"TPM devices",
			"Security",
			filepath.Join(e.paths.Sys, "class/tpm/tpm*"),
			"TPM devices detected",
			"No TPM device is exposed",
		},
		{
			"input",
			"Input devices",
			"Multimedia",
			filepath.Join(
				e.paths.Sys,
				"class/input/input*",
			),
			"input devices detected",
			"No input devices are exposed",
		},
		{
			"backlight",
			"Backlight controllers",
			"Multimedia",
			filepath.Join(e.paths.Sys, "class/backlight/*"),
			"backlight controllers detected",
			"No backlight controller is exposed",
		},
		{
			"tee",
			"Trusted Execution Environment",
			"Security",
			filepath.Join(e.paths.Sys, "class/tee/tee*"),
			"TEE devices detected",
			"No TEE device is exposed",
		},
		{
			"watchdog",
			"Hardware watchdogs",
			"Platform",
			filepath.Join(
				e.paths.Sys,
				"class/watchdog/watchdog*",
			),
			"watchdog devices detected",
			"No watchdog device is exposed",
		},
		{
			"iio",
			"IIO / ADC devices",
			"Buses & I/O",
			filepath.Join(
				e.paths.Sys,
				"bus/iio/devices/iio:device*",
			),
			"IIO devices detected",
			"No IIO device is exposed",
		},
		{
			"suspend",
			"Suspend states",
			"Platform",
			filepath.Join(e.paths.Sys, "power/state"),
			"suspend interface exposed",
			"No suspend interface is exposed",
		},
		{
			"crypto",
			"Kernel crypto API",
			"Security",
			filepath.Join(e.paths.Proc, "crypto"),
			"kernel crypto inventory available",
			"Kernel crypto inventory is unavailable",
		},
	}
	result := make([]Capability, 0, len(definitions)+1)
	for _, definition := range definitions {
		matches, _ := filepath.Glob(definition.pattern)
		available, count := len(matches) > 0, len(matches)
		detail := definition.missingDetail
		if available {
			detail = fmt.Sprintf(
				"%d %s",
				count,
				definition.availableDetail,
			)
		}
		if definition.id == "npu" &&
			productHasNPU(snapshot) {
			available = true
			if count == 0 {
				count = 1
			}
			detail = fmt.Sprintf(
				"Integrated NPU identified from the %s product "+"profile",
				snapshot.Board.SoC,
			)
		}
		result = append(
			result,
			Capability{
				ID:        definition.id,
				Name:      definition.name,
				Category:  definition.category,
				Available: available,
				Count:     count,
				Detail:    detail,
			},
		)
	}
	canCount, wifiCount := 0, 0
	for _, network := range snapshot.Networks {
		if strings.HasPrefix(network.Name, "can") {
			canCount++
		}
		if strings.HasPrefix(network.Name, "wlan") ||
			strings.HasPrefix(network.Name, "wl") {
			wifiCount++
		}
	}
	result = append(
		result,
		Capability{
			ID:        "can",
			Name:      "CAN interfaces",
			Category:  "Connectivity",
			Available: canCount > 0,
			Count:     canCount,
			Detail: fmt.Sprintf(
				"%d CAN interface(s) detected",
				canCount,
			),
		},
	)
	result = append(
		result,
		Capability{
			ID:        "wifi",
			Name:      "Wi-Fi interfaces",
			Category:  "Connectivity",
			Available: wifiCount > 0,
			Count:     wifiCount,
			Detail: fmt.Sprintf(
				"%d Wi-Fi interface(s) detected",
				wifiCount,
			),
		},
	)
	sort.SliceStable(
		result,
		func(i, j int) bool { return result[i].Category < result[j].Category },
	)
	return result
}

func productHasNPU(snapshot collector.Snapshot) bool {
	if snapshot.Board.Confidence != "exact" &&
		snapshot.Board.Confidence != "compatible" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(snapshot.Board.SoC)) {
	case "i.mx 8m plus", "i.mx 93", "i.mx 95":
		return true
	default:
		return false
	}
}

func (e *Engine) guides(
	snapshot collector.Snapshot,
	kernel support.KernelLog,
) []Guide {
	findNetwork := func() Guide {
		steps := []GuideStep{}
		active := 0
		for _, network := range snapshot.Networks {
			if network.State == "up" {
				active++
			}
		}
		steps = append(
			steps,
			step(
				active > 0,
				"Network link",
				fmt.Sprintf(
					"%d active interface(s)",
					active,
				),
			),
		)
		steps = append(
			steps,
			step(
				len(snapshot.Ports) > 0,
				"Listening services",
				fmt.Sprintf(
					"%d listening port(s)",
					len(snapshot.Ports),
				),
			),
		)
		conclusion := "Network interfaces and local services are " + "available."
		if active == 0 {
			conclusion = "No active network link was detected. Check the " +
				"cable, PHY configuration, and Device Tree."
		}
		return Guide{
			ID:          "network",
			Name:        "Network is unavailable",
			Description: "Check link state, addressing, and listening " + "services.",
			Conclusion:  conclusion,
			Steps:       steps,
		}
	}
	cameraMatches, _ := filepath.Glob(
		filepath.Join(
			e.paths.Sys,
			"class/video4linux/video*",
		),
	)
	displayMatches, _ := filepath.Glob(
		filepath.Join(e.paths.Sys, "class/drm/card*-*"),
	)
	errorCount := 0
	for _, message := range kernel.Messages {
		if message.Level == "ERROR" ||
			message.Level == "CRITICAL" {
			errorCount++
		}
	}
	return []Guide{
		findNetwork(),
		Guide{
			ID:          "camera",
			Name:        "Camera is not detected",
			Description: "Check V4L2 enumeration and media-device " + "exposure.",
			Conclusion: choose(
				len(
					cameraMatches,
				) > 0,
				"Video devices are enumerated. Continue in Camera "+
					"Lab to validate capture.",
				"No V4L2 device is exposed. Check sensor power, "+
					"clock, overlays, and media drivers.",
			),
			Steps: []GuideStep{
				step(
					len(cameraMatches) > 0,
					"V4L2 enumeration",
					fmt.Sprintf(
						"%d video device(s)",
						len(cameraMatches),
					),
				),
			},
		},
		Guide{
			ID:          "display",
			Name:        "Display is not working",
			Description: "Check DRM connector enumeration and graphics " + "support.",
			Conclusion: choose(
				len(displayMatches) > 0,
				"DRM connectors are exposed by the kernel.",
				"No DRM connectors were found. Check display "+
					"overlays and kernel configuration.",
			),
			Steps: []GuideStep{
				step(
					len(displayMatches) > 0,
					"DRM connectors",
					fmt.Sprintf(
						"%d connector(s)",
						len(displayMatches),
					),
				),
			},
		},
		Guide{
			ID:          "kernel",
			Name:        "System appears unstable",
			Description: "Inspect retained error-level kernel messages and " +
				"crash records.",
			Conclusion: choose(
				errorCount == 0,
				"No retained error-level kernel messages were "+"found.",
				"Kernel errors were found. Open Support to "+
					"inspect and export the evidence.",
			),
			Steps: []GuideStep{
				step(
					errorCount == 0,
					"Kernel errors",
					fmt.Sprintf(
						"%d error-level message(s)",
						errorCount,
					),
				),
			},
		},
	}
}

func (e *Engine) actions(
	capabilities []Capability,
) []Action {
	available := map[string]bool{}
	for _, capability := range capabilities {
		available[capability.ID] = capability.Available
	}
	return []Action{
		{
			ID:          "system-sanity",
			Name:        "System sanity check",
			Description: "Evaluate thermals, memory, storage, networking, " +
				"kernel taint, and crash records.",
			Category:    "System",
			Available:   true,
		},
		{
			ID:          "network-sanity",
			Name:        "Network sanity check",
			Description: "Validate active links, addresses, counters, and " +
				"interface errors without generating traffic.",
			Category:    "Connectivity",
			Available:   true,
		},
		{
			ID:          "camera-enumeration",
			Name:        "Camera enumeration",
			Description: "Validate V4L2 device exposure without starting a " + "stream.",
			Category:    "Multimedia",
			Available:   available["camera"],
			Unavailable: choose(
				available["camera"],
				"",
				"No V4L2 devices are exposed",
			),
		},
		{
			ID:          "display-enumeration",
			Name:        "Display enumeration",
			Description: "Validate DRM connector exposure without changing " +
				"display state.",
			Category:    "Multimedia",
			Available:   available["drm"],
			Unavailable: choose(
				available["drm"],
				"",
				"No DRM connectors are exposed",
			),
		},
	}
}

func (e *Engine) Run(
	id string,
	snapshot collector.Snapshot,
	kernel support.KernelLog,
) (Result, bool) {
	started := time.Now().UTC()
	result := Result{
		ActionID:  id,
		Status:    "passed",
		StartedAt: started,
	}
	switch id {
	case "system-sanity":
		health := e.health(snapshot, kernel)
		result.Summary = health.Summary
		for _, check := range health.Checks {
			result.Checks = append(
				result.Checks,
				GuideStep{
					Name:     check.Name,
					Status:   check.Status,
					Evidence: check.Evidence,
				},
			)
			if check.Status == "critical" ||
				check.Status == "warning" {
				result.Status = "attention"
			}
		}
	case "network-sanity":
		for _, network := range snapshot.Networks {
			if network.Name == "lo" {
				continue
			}
			ok := network.State == "up" &&
				network.RXErrors+network.TXErrors == 0
			result.Checks = append(
				result.Checks,
				step(
					ok,
					network.Name,
					fmt.Sprintf(
						"state=%s, addresses=%d, errors=%d",
						network.State,
						len(network.Addresses),
						network.RXErrors+network.TXErrors,
					),
				),
			)
		}
		result.Summary = "Network interfaces inspected without generating " +
			"traffic."
	case "camera-enumeration", "display-enumeration":
		capabilityID := strings.TrimSuffix(
			id,
			"-enumeration",
		)
		for _, capability := range e.capabilities(snapshot) {
			if capability.ID == capabilityID {
				result.Checks = append(
					result.Checks,
					step(
						capability.Available,
						capability.Name,
						capability.Detail,
					),
				)
				result.Summary = capability.Detail
				if !capability.Available {
					result.Status = "attention"
				}
			}
		}
	default:
		return Result{}, false
	}
	result.Duration = time.Since(started).Milliseconds()
	return result, true
}

func step(ok bool, name, evidence string) GuideStep {
	status := "passed"
	if !ok {
		status = "attention"
	}
	return GuideStep{
		Name:     name,
		Status:   status,
		Evidence: evidence,
	}
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(used) / float64(total)
}

func read(path string) string {
	value, _ := os.ReadFile(path)
	return string(value)
}

func choose(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}
