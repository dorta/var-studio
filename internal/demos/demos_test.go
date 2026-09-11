package demos

import (
	"strings"
	"testing"
)

func TestCatalogEntriesAreUniqueAndBounded(t *testing.T) {
	seen := make(map[string]bool)
	for _, demo := range Catalog {
		if demo.ID == "" || demo.Name == "" || demo.Kind == "" {
			t.Fatalf("incomplete catalog entry: %#v", demo)
		}
		if seen[demo.ID] {
			t.Fatalf("duplicate demo id: %s", demo.ID)
		}
		seen[demo.ID] = true
		if demo.DurationSec < 1 || demo.DurationSec > 300 {
			t.Fatalf(
				"unsafe duration for %s: %d",
				demo.ID,
				demo.DurationSec,
			)
		}
	}
}

func TestNativeDisplayDemosUseFixedInstallDirectory(t *testing.T) {
	found := 0
	for _, demo := range Catalog {
		if !strings.HasPrefix(demo.Kind, "display-") {
			continue
		}
		found++
		const prefix = "/opt/var-studio-demos/bin/"
		if !strings.HasPrefix(demo.command, prefix) {
			t.Fatalf("unsafe native demo command: %s", demo.command)
		}
	}
	if found != 2 {
		t.Fatalf("expected two native display demos, got %d", found)
	}
}
