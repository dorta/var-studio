package npu

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	historyFile   = "npu.jsonl"
	formatVersion = 1
)

type Device struct {
	CoreID int    `json:"core_id"`
	Model  string `json:"model"`
}

type Sample struct {
	Timestamp    time.Time `json:"timestamp"`
	UsagePercent float64   `json:"usage_percent"`
	ClockHz      uint64    `json:"clock_hz"`
	ClockEnabled bool      `json:"clock_enabled"`
}

type Report struct {
	Available         bool      `json:"available"`
	Reason            string    `json:"reason,omitempty"`
	Device            Device    `json:"device"`
	StartedAt         time.Time `json:"started_at"`
	SampleIntervalSec int       `json:"sample_interval_seconds"`
	Samples           []Sample  `json:"samples"`
}

type header struct {
	BootID        string    `json:"boot_id"`
	StartedAt     time.Time `json:"started_at"`
	FormatVersion int       `json:"format_version"`
	Available     bool      `json:"available"`
	Reason        string    `json:"reason,omitempty"`
	Device        Device    `json:"device"`
}

func Start(
	ctx context.Context,
	directory, bootID, sysRoot string,
) error {
	if sysRoot == "" {
		sysRoot = "/sys"
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	device, discoverErr := Discover(
		filepath.Join(sysRoot, "kernel/debug/gc/info"),
	)
	metadata := header{
		BootID: bootID, StartedAt: time.Now().UTC(), FormatVersion: formatVersion,
		Available: discoverErr == nil, Device: device,
	}
	if discoverErr != nil {
		metadata.Reason = discoverErr.Error()
	}
	path := filepath.Join(directory, historyFile)
	if existing, ok := readHeader(path); ok &&
		existing.BootID == bootID &&
		existing.FormatVersion == formatVersion &&
		existing.Device == device &&
		existing.Available == metadata.Available {
		metadata.StartedAt = existing.StartedAt
	} else if err := writeHeader(path, metadata); err != nil {
		return err
	}
	if os.Geteuid() == 0 {
		if err := os.Chown(path, 65532, 65532); err != nil {
			return fmt.Errorf(
				"grant dashboard access to NPU history: %w",
				err,
			)
		}
	}
	if !metadata.Available {
		<-ctx.Done()
		return nil
	}

	file, err := os.OpenFile(
		path,
		os.O_APPEND|os.O_WRONLY,
		0o640,
	)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	loadPath := filepath.Join(
		sysRoot,
		"kernel/debug/gc/load",
	)
	clockRoot := filepath.Join(
		sysRoot,
		"kernel/debug/clk/npu_root_clk",
	)
	record := func() {
		usage, err := ReadLoad(loadPath, device.CoreID)
		if err != nil {
			return
		}
		clock, _ := readUint(
			filepath.Join(clockRoot, "clk_rate"),
		)
		enabled, _ := readUint(
			filepath.Join(clockRoot, "clk_enable_count"),
		)
		_ = encoder.Encode(
			Sample{
				Timestamp:    time.Now().UTC(),
				UsagePercent: usage,
				ClockHz:      clock,
				ClockEnabled: enabled > 0,
			},
		)
	}
	record()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			record()
		case <-ctx.Done():
			return nil
		}
	}
}

func Discover(path string) (Device, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Device{}, fmt.Errorf(
			"Galcore inventory unavailable: %w",
			err,
		)
	}
	return ParseInfo(string(data))
}

func ParseInfo(value string) (Device, error) {
	current := Device{CoreID: -1}
	flush := func() (Device, bool) {
		model := strings.TrimSpace(current.Model)
		return current, current.CoreID >= 0 &&
			(model == "8000" || strings.HasPrefix(model, "8000"))
	}
	for _, line := range strings.Split(value, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, field := strings.TrimSpace(
			parts[0],
		), strings.TrimSpace(
			parts[1],
		)
		if key == "gpu" {
			if device, ok := flush(); ok {
				device.Model = "GC" + device.Model
				return device, nil
			}
			current = Device{CoreID: -1}
			current.CoreID, _ = strconv.Atoi(field)
		} else if key == "model" {
			current.Model = field
		}
	}
	if device, ok := flush(); ok {
		device.Model = "GC" + device.Model
		return device, nil
	}
	return Device{}, errors.New(
		"no supported Vivante NPU was detected",
	)
}

func ReadLoad(path string, coreID int) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return ParseLoad(string(data), coreID)
}

func ParseLoad(value string, coreID int) (float64, error) {
	current := -1
	for _, line := range strings.Split(value, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, field := strings.TrimSpace(
			parts[0],
		), strings.TrimSpace(
			parts[1],
		)
		switch key {
		case "core":
			current, _ = strconv.Atoi(field)
		case "load":
			if current == coreID {
				load, err := strconv.ParseFloat(
					strings.TrimSpace(
						strings.TrimSuffix(field, "%"),
					),
					64,
				)
				return load, err
			}
		}
	}
	return 0, fmt.Errorf(
		"load for Galcore core %d is unavailable",
		coreID,
	)
}

func Read(
	directory, bootID string,
	since time.Time,
	maxPoints int,
) Report {
	file, err := os.Open(
		filepath.Join(directory, historyFile),
	)
	if err != nil {
		return Report{
			Reason: fmt.Sprintf(
				"NPU history unavailable: %v",
				err,
			),
			Samples: []Sample{},
		}
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var metadata header
	if !scanner.Scan() ||
		json.Unmarshal(scanner.Bytes(), &metadata) != nil ||
		metadata.BootID != bootID ||
		metadata.FormatVersion != formatVersion {
		return Report{
			Reason:  "NPU history is unavailable for this boot",
			Samples: []Sample{},
		}
	}
	samples := make([]Sample, 0)
	for scanner.Scan() {
		var sample Sample
		if json.Unmarshal(
			scanner.Bytes(),
			&sample,
		) == nil &&
			!sample.Timestamp.IsZero() &&
			(since.IsZero() || !sample.Timestamp.Before(since)) {
			samples = append(samples, sample)
		}
	}
	return Report{
		Available:         metadata.Available,
		Reason:            metadata.Reason,
		Device:            metadata.Device,
		StartedAt:         metadata.StartedAt,
		SampleIntervalSec: 1,
		Samples:           downsample(samples, maxPoints),
	}
}

func writeHeader(path string, metadata header) error {
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		0o640,
	)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(metadata)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func readHeader(path string) (header, bool) {
	file, err := os.Open(path)
	if err != nil {
		return header{}, false
	}
	defer file.Close()
	var value header
	err = json.NewDecoder(file).Decode(&value)
	return value, err == nil
}

func readUint(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(
		strings.TrimSpace(string(data)),
		10,
		64,
	)
}

func downsample(samples []Sample, maxPoints int) []Sample {
	if maxPoints <= 0 || len(samples) <= maxPoints {
		return samples
	}
	result := make([]Sample, 0, maxPoints)
	for bucket := 0; bucket < maxPoints; bucket++ {
		start, end := bucket*len(
			samples,
		)/maxPoints, (bucket+1)*len(
			samples,
		)/maxPoints
		if end <= start {
			continue
		}
		value := Sample{Timestamp: samples[end-1].Timestamp}
		for _, sample := range samples[start:end] {
			value.UsagePercent += sample.UsagePercent
			value.ClockHz += sample.ClockHz
			value.ClockEnabled = value.ClockEnabled ||
				sample.ClockEnabled
		}
		count := float64(end - start)
		value.UsagePercent /= count
		value.ClockHz /= uint64(end - start)
		result = append(result, value)
	}
	sort.Slice(
		result,
		func(i, j int) bool {
			return result[i].Timestamp.Before(
				result[j].Timestamp)
		},
	)
	return result
}
