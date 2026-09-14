package gpu

import "testing"

func TestParseOccupancyPercent(t *testing.T) {
	tests := []struct {
		line string
		name string
		want float64
	}{
		{
			" MC (Memory Controller)            5.00%",
			"MC",
			5,
		},
		{
			"\x1b[H\x1b[J USAGE                    17.25%",
			"USAGE",
			17.25,
		},
	}
	for _, test := range tests {
		name, value, ok := parsePercent(test.line)
		if !ok || name != test.name || value != test.want {
			t.Fatalf(
				"parsePercent(%q) = %q, %v, %v",
				test.line,
				name,
				value,
				ok,
			)
		}
	}
}
