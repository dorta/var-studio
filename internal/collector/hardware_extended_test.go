package collector

import "testing"

func TestHexIPv4(t *testing.T) {
	if got := hexIPv4("0100A8C0"); got != "192.168.0.1" {
		t.Fatalf("hexIPv4() = %q", got)
	}
}

func TestParseEnvironment(t *testing.T) {
	got := parseEnvironment(
		"LANG=pt_BR.UTF-8\n# comment\nNAME=\"VAR-Studio\"\n",
	)
	if len(got) != 2 || got[0][0] != "LANG" ||
		got[1][1] != "VAR-Studio" {
		t.Fatalf("parseEnvironment() = %#v", got)
	}
}

func TestResourceTable(t *testing.T) {
	got := resourceTable(
		"Memory",
		"1000-1fff : device\n",
		"Range",
		"Resource",
	)
	if len(got.Rows) != 1 ||
		got.Rows[0][0] != "1000-1fff" ||
		got.Rows[0][1] != "device" {
		t.Fatalf("resourceTable() = %#v", got.Rows)
	}
}
