package npu

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const fixtureInfo = `gpu      : 0
model    : 7000
revision : 6204

gpu      : 1
model    : 8000
revision : 8002

gpu      : 16
model    :  520
revision : 5341
`

func TestParseInfoFindsNPUWithoutHardcodedCore(
	t *testing.T,
) {
	device, err := ParseInfo(fixtureInfo)
	if err != nil || device.CoreID != 1 ||
		device.Model != "GC8000" {
		t.Fatalf("ParseInfo() = %+v, %v", device, err)
	}
}

func TestParseLoadUsesDiscoveredCore(t *testing.T) {
	load, err := ParseLoad(
		"core : 0\nload : 12%\n\ncore : 1\nload : 73%\n",
		1,
	)
	if err != nil || load != 73 {
		t.Fatalf("ParseLoad() = %v, %v", load, err)
	}
}

func TestDownsampleAveragesUsageAndClock(t *testing.T) {
	samples := []Sample{
		{
			Timestamp:    time.Unix(1, 0),
			UsagePercent: 20,
			ClockHz:      800,
			ClockEnabled: true,
		},
		{
			Timestamp:    time.Unix(2, 0),
			UsagePercent: 40,
			ClockHz:      1000,
		},
	}
	got := downsample(samples, 1)
	if len(got) != 1 || got[0].UsagePercent != 30 ||
		got[0].ClockHz != 900 ||
		!got[0].ClockEnabled {
		t.Fatalf("downsample() = %+v", got)
	}
}

func TestUnavailableHistoryIsReadableByDashboard(
	t *testing.T,
) {
	if os.Geteuid() != 0 {
		t.Skip("ownership assertion requires root")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	directory := t.TempDir()
	if err := Start(
		ctx,
		directory,
		"test-boot",
		filepath.Join(directory, "missing-sys"),
	); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(directory, historyFile))
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("history ownership is unavailable")
	}
	if stat.Uid != 65532 || stat.Gid != 65532 {
		t.Fatalf(
			"history owner = %d:%d, want 65532:65532",
			stat.Uid,
			stat.Gid,
		)
	}
}
