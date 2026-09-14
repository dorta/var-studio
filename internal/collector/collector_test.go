package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeAddressIPv4(t *testing.T) {
	address, port, ok := decodeAddress(
		"0100007F:1F90",
		false,
	)
	if !ok || address != "127.0.0.1" || port != 8080 {
		t.Fatalf(
			"decodeAddress() = %q, %d, %v",
			address,
			port,
			ok,
		)
	}
}

func TestBuildInfo(t *testing.T) {
	etc := t.TempDir()
	data := `DISTRO = fsl-imx-xwayland
DISTRO_VERSION = 6.6-scarthgap
meta-variscite-bsp-imx = HEAD:e3b45413e41b232c466bf29eea0ed3f729a75459
meta-freescale = HEAD:b699d026506a6fcdd63d5f0929cb3a27b6fed -- modified
`
	if err := os.WriteFile(filepath.Join(etc, "buildinfo"), []byte(data), 0o600);
		err != nil {
		t.Fatal(err)
	}
	collector := New(Paths{Etc: etc})
	var warnings []string
	got := collector.buildInfo(&warnings)
	if got.Distro != "fsl-imx-xwayland" ||
		got.DistroVersion != "6.6-scarthgap" {
		t.Fatalf("unexpected distro: %+v", got)
	}
	if len(got.Layers) != 2 || got.Layers[0].Modified ||
		!got.Layers[1].Modified {
		t.Fatalf("unexpected layers: %+v", got.Layers)
	}
	if got.Layers[1].Revision != "b699d026506a6fcdd63d5f0929cb3a27b6fed" {
		t.Fatalf(
			"modified suffix was not removed: %q",
			got.Layers[1].Revision,
		)
	}
}
