package diagnostics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dorta/var-studio/internal/boards"
	"github.com/dorta/var-studio/internal/collector"
	"github.com/dorta/var-studio/internal/support"
)

func TestEngineBuildExplainsHealthAndCapabilities(
	t *testing.T,
) {
	root := t.TempDir()
	paths := collector.Paths{
		Proc: filepath.Join(root, "proc"),
		Sys:  filepath.Join(root, "sys"),
		Etc:  filepath.Join(root, "etc"),
	}
	for _, directory := range []string{
		filepath.Join(paths.Proc, "sys/kernel"),
		filepath.Join(paths.Sys, "class/video4linux/video0"),
		filepath.Join(paths.Sys, "class/drm/card0"),
		filepath.Join(paths.Sys, "class/sound/card0"),
		filepath.Join(paths.Sys, "class/gpio/gpiochip0"),
		filepath.Join(paths.Sys, "fs/pstore"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(paths.Proc, "sys/kernel/tainted"),
		[]byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := collector.Snapshot{
		Timestamp: time.Now().UTC(),
		Metrics: collector.Metrics{
			MemoryTotal:  100,
			MemoryUsed:   20,
			StorageTotal: 100,
			StorageUsed:  40,
		},
		Thermals: []collector.Thermal{
			{Name: "soc-thermal", Celsius: 96},
		},
		Networks: []collector.Network{
			{Name: "eth0", State: "up"},
		},
	}
	kernel := support.KernelLog{
		Available: true,
		Messages: []support.KernelMessage{
			{
				Sequence: 1,
				Level:    "ERROR",
				Message:  "test fault",
			},
		},
	}
	dashboard := New(paths).Build(snapshot, kernel, nil)
	if dashboard.Health.Status != "critical" {
		t.Fatalf(
			"health status = %q, want critical",
			dashboard.Health.Status,
		)
	}
	if dashboard.Health.Critical == 0 {
		t.Fatal("expected a critical health check")
	}
	assertCapability(
		t,
		dashboard.Capabilities,
		"camera",
		true,
	)
	assertCapability(t, dashboard.Capabilities, "gpu", true)
	assertCapability(
		t,
		dashboard.Capabilities,
		"audio",
		true,
	)
	assertCapability(
		t,
		dashboard.Capabilities,
		"npu",
		false,
	)
}

func TestEngineRecognizesIntegratedMPlusNPUFromExactProductProfile(
	t *testing.T,
) {
	root := t.TempDir()
	paths := collector.Paths{
		Proc: filepath.Join(root, "proc"),
		Sys:  filepath.Join(root, "sys"),
		Etc:  filepath.Join(root, "etc"),
	}
	snapshot := collector.Snapshot{
		Board: boards.Product{
			SoC:        "i.MX 8M Plus",
			Confidence: "exact",
		},
	}
	dashboard := New(
		paths,
	).Build(snapshot, support.KernelLog{}, nil)
	assertCapability(t, dashboard.Capabilities, "npu", true)
}

func TestTrackerRecordsTransitionsAndPersists(
	t *testing.T,
) {
	directory := t.TempDir()
	tracker := NewTracker(directory, "boot/test")
	start := time.Now().UTC()
	snapshot := collector.Snapshot{
		Timestamp: start,
		System: collector.SystemInfo{
			Model:  "Test board",
			Kernel: "6.6",
		},
		Networks: []collector.Network{
			{Name: "eth0", State: "down"},
		},
		Thermals: []collector.Thermal{
			{Name: "soc", Celsius: 60},
		},
	}
	tracker.Observe(snapshot, support.KernelLog{})
	snapshot.Timestamp = start.Add(time.Second)
	snapshot.Networks[0].State = "up"
	snapshot.Thermals[0].Celsius = 90
	tracker.Observe(
		snapshot,
		support.KernelLog{
			Messages: []support.KernelMessage{
				{
					Sequence: 7,
					Level:    "WARNING",
					Message:  "test warning",
				},
			},
		},
	)
	events := tracker.Events(20)
	if len(events) != 4 {
		t.Fatalf("event count = %d, want 4", len(events))
	}
	if events[0].Category != "kernel" ||
		events[1].Category != "thermal" ||
		events[2].Category != "network" {
		t.Fatalf(
			"unexpected newest-first event order: %#v",
			events,
		)
	}
	reloaded := NewTracker(directory, "boot/test")
	if got := len(reloaded.Events(20)); got != 4 {
		t.Fatalf("persisted event count = %d, want 4", got)
	}
}

func assertCapability(
	t *testing.T,
	capabilities []Capability,
	id string,
	available bool,
) {
	t.Helper()
	for _, capability := range capabilities {
		if capability.ID == id {
			if capability.Available != available {
				t.Fatalf(
					"capability %s available = %v, want %v",
					id,
					capability.Available,
					available,
				)
			}
			return
		}
	}
	t.Fatalf("capability %s not found", id)
}
