package history

import (
	"testing"
	"time"
)

func TestStorePersistsWithinBootAndResetsOnNewBoot(
	t *testing.T,
) {
	directory := t.TempDir()
	store, err := Open(directory, "boot-a", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	sample := Sample{
		Timestamp:     time.Now().UTC(),
		CPUPercent:    25,
		PerCore:       []float64{10, 40},
		MemoryPercent: 50,
	}
	if err := store.Append(sample); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(
		directory,
		"boot-a",
		5*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Report(100).Samples; len(got) != 1 ||
		got[0].MemoryPercent != 50 {
		t.Fatalf("unexpected persisted samples: %+v", got)
	}
	_ = reopened.Close()

	newBoot, err := Open(directory, "boot-b", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer newBoot.Close()
	if got := newBoot.Report(100).Samples; len(got) != 0 {
		t.Fatalf(
			"history was not reset for a new boot: %+v",
			got,
		)
	}
}

func TestDownsampleAveragesBuckets(t *testing.T) {
	samples := []Sample{
		{
			Timestamp:     time.Unix(1, 0),
			CPUPercent:    10,
			PerCore:       []float64{20},
			MemoryPercent: 30,
		},
		{
			Timestamp:     time.Unix(2, 0),
			CPUPercent:    30,
			PerCore:       []float64{40},
			MemoryPercent: 50,
		},
		{
			Timestamp:     time.Unix(3, 0),
			CPUPercent:    50,
			PerCore:       []float64{60},
			MemoryPercent: 70,
		},
		{
			Timestamp:     time.Unix(4, 0),
			CPUPercent:    70,
			PerCore:       []float64{80},
			MemoryPercent: 90,
		},
	}
	got := downsample(samples, 2)
	if len(got) != 2 || got[0].CPUPercent != 20 ||
		got[1].PerCore[0] != 70 {
		t.Fatalf("unexpected downsample: %+v", got)
	}
}

func TestReportSinceFiltersBeforeDownsampling(
	t *testing.T,
) {
	store, err := Open(t.TempDir(), "boot-a", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	base := time.Now().UTC().Add(-2 * time.Hour)
	for index := 0; index < 3; index++ {
		_ = store.Append(
			Sample{
				Timestamp: base.Add(
					time.Duration(index) * time.Hour,
				),
				CPUPercent: float64(index),
			},
		)
	}
	got := store.ReportSince(
		base.Add(30*time.Minute),
		10,
	).Samples
	if len(got) != 2 || got[0].CPUPercent != 1 {
		t.Fatalf("unexpected filtered report: %+v", got)
	}
}
