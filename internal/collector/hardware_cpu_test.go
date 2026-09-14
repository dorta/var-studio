package collector

import "testing"

func TestCPUDisplayNameARMPart(t *testing.T) {
	got := cpuDisplayName(
		map[string]string{"CPU part": "0xd04"},
	)
	if got != "Arm Cortex-A35 (0xd04)" {
		t.Fatalf("cpuDisplayName() = %q", got)
	}
}

func TestCPUDisplayNamePrefersModelName(t *testing.T) {
	got := cpuDisplayName(
		map[string]string{
			"model name": "Example CPU",
			"CPU part":   "0xd04",
		},
	)
	if got != "Example CPU" {
		t.Fatalf("cpuDisplayName() = %q", got)
	}
}

func TestFormatCPUFrequency(t *testing.T) {
	tests := map[string]string{
		"1200000": "1.20 GHz",
		"800000":  "800 MHz",
		"invalid": "",
	}
	for raw, want := range tests {
		if got := formatCPUFrequency(raw); got != want {
			t.Errorf(
				"formatCPUFrequency(%q) = %q, want %q",
				raw,
				got,
				want,
			)
		}
	}
}
