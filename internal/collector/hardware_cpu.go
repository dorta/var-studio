package collector

import (
	"fmt"
	"path/filepath"
	"strconv"
)

func cpuDisplayName(values map[string]string) string {
	if name := first(values["model name"], values["Processor"],
		values["Hardware"]); name != "Not available" {
		return name
	}
	parts := map[string]string{
		"0xd03": "Arm Cortex-A53", "0xd04": "Arm Cortex-A35",
			"0xd05": "Arm Cortex-A55",
		"0xd07": "Arm Cortex-A57", "0xd08": "Arm Cortex-A72",
			"0xd09": "Arm Cortex-A73",
		"0xd0a": "Arm Cortex-A75", "0xd0b": "Arm Cortex-A76",
			"0xd0d": "Arm Cortex-A77",
		"0xd41": "Arm Cortex-A78", "0xd44": "Arm Cortex-X1",
			"0xd46": "Arm Cortex-A510",
		"0xd47": "Arm Cortex-A710", "0xd4b": "Arm Cortex-A78C",
	}
	part := values["CPU part"]
	if name := parts[part]; name != "" {
		return name + " (" + part + ")"
	}
	return first(part, "Arm processor")
}

func formatCPUFrequency(raw string) string {
	khz, err := strconv.ParseFloat(clean(raw), 64)
	if err != nil || khz <= 0 {
		return ""
	}
	if khz >= 1000000 {
		return fmt.Sprintf("%.2f GHz", khz/1000000)
	}
	return fmt.Sprintf("%.0f MHz", khz/1000)
}

func (c *Collector) cpuCacheDetails() HardwareTable {
	rows := [][]string{}
	for _, cache := range glob(filepath.Join(c.paths.Sys,
		"devices/system/cpu/cpu0/cache/index*")) {
		rows = append(rows, []string{
			filepath.Base(
				cache,
			), clean(read(filepath.Join(cache, "level"))), clean(read(filepath.Join(
				cache, "type"))),
			clean(
				read(filepath.Join(cache, "size")),
			), clean(read(filepath.Join(cache, "coherency_line_size"))),
			clean(
				read(
					filepath.Join(
						cache,
						"ways_of_associativity",
					),
				),
			), clean(read(filepath.Join(cache, "number_of_sets"))),
		})
	}
	return HardwareTable{
		Title: "Caches",
		Columns: []string{
			"Index",
			"Level",
			"Type",
			"Size",
			"Line (bytes)",
			"Ways",
			"Sets",
		},
		Rows: rows,
	}
}
