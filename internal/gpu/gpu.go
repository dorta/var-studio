package gpu

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	historyFile   = "gpu.jsonl"
	formatVersion = 2
)

type Sample struct {
	Timestamp        time.Time `json:"timestamp"`
	UsagePercent     float64   `json:"usage_percent"`
	MemoryController float64   `json:"memory_controller_percent"`
}

type Report struct {
	Available         bool      `json:"available"`
	StartedAt         time.Time `json:"started_at"`
	SampleIntervalSec int       `json:"sample_interval_seconds"`
	Samples           []Sample  `json:"samples"`
}

type header struct {
	BootID        string    `json:"boot_id"`
	StartedAt     time.Time `json:"started_at"`
	FormatVersion int       `json:"format_version"`
}

func Start(
	ctx context.Context,
	directory, bootID, executable string,
) error {
	if executable == "" {
		executable = "/usr/bin/gputop"
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	path := filepath.Join(directory, historyFile)
	startedAt := time.Now().UTC()
	if existing, ok := readHeader(path); !ok ||
		existing.BootID != bootID ||
		existing.FormatVersion != formatVersion {
		file, err := os.OpenFile(
			path,
			os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
			0o640,
		)
		if err != nil {
			return err
		}
		err = json.NewEncoder(file).
			Encode(header{BootID: bootID, StartedAt: startedAt,
				FormatVersion: formatVersion})
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
	}
	file, err := os.OpenFile(
		path,
		os.O_APPEND|os.O_WRONLY,
		0o640,
	)
	if err != nil {
		return err
	}
	if os.Geteuid() == 0 {
		_ = os.Chown(path, 65532, 65532)
	}
	defer file.Close()

	command := exec.CommandContext(
		ctx,
		executable,
		"-m",
		"occupancy",
		"-f",
	)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	scanner := bufio.NewScanner(stdout)
	memoryController := 0.0
	usageRecorded := false
	for scanner.Scan() {
		line := strings.TrimSpace(stripANSI(scanner.Text()))
		if strings.HasPrefix(line, "Occupancy |") {
			usageRecorded = false
			continue
		}
		name, value, ok := parsePercent(line)
		if !ok {
			continue
		}
		switch name {
		case "MC":
			memoryController = value
		case "USAGE":
			// Some SoCs expose more than one GPU engine.
			// gputop emits a USAGE line for each engine in
			// one Occupancy block; the first one is the
			// primary 3D engine represented by this
			// single-series API.
			if usageRecorded {
				continue
			}
			usageRecorded = true
			if err := encoder.Encode(Sample{Timestamp: time.Now().UTC(),
				UsagePercent: value, MemoryController: memoryController}); err != nil {
				_ = command.Process.Kill()
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	err = command.Wait()
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func Read(
	directory, bootID string,
	since time.Time,
	maxPoints int,
) Report {
	path := filepath.Join(directory, historyFile)
	file, err := os.Open(path)
	if err != nil {
		return Report{Samples: []Sample{}}
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var metadata header
	if !scanner.Scan() ||
		json.Unmarshal(scanner.Bytes(), &metadata) != nil ||
		metadata.BootID != bootID ||
		metadata.FormatVersion != formatVersion {
		return Report{Samples: []Sample{}}
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
		Available:         true,
		StartedAt:         metadata.StartedAt,
		SampleIntervalSec: 1,
		Samples:           downsample(samples, maxPoints),
	}
}

func readHeader(path string) (header, bool) {
	file, err := os.Open(path)
	if err != nil {
		return header{}, false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var value header
	if !scanner.Scan() ||
		json.Unmarshal(scanner.Bytes(), &value) != nil {
		return header{}, false
	}
	return value, true
}

func parsePercent(line string) (string, float64, bool) {
	line = strings.TrimSpace(stripANSI(line))
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", 0, false
	}
	name := fields[0]
	if name != "MC" && name != "USAGE" {
		return "", 0, false
	}
	value, err := strconv.ParseFloat(
		strings.TrimSuffix(fields[len(fields)-1], "%"),
		64,
	)
	return name, value, err == nil
}

func stripANSI(value string) string {
	for {
		start := strings.IndexByte(value, 0x1b)
		if start < 0 {
			return value
		}
		end := start + 2
		for end < len(value) && (value[end] < '@' || value[end] > '~') {
			end++
		}
		if end < len(value) {
			end++
		}
		value = value[:start] + value[end:]
	}
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
			value.MemoryController += sample.MemoryController
		}
		count := float64(end - start)
		value.UsagePercent /= count
		value.MemoryController /= count
		result = append(result, value)
	}
	sort.Slice(
		result,
		func(i, j int) bool { return result[i].Timestamp.Before(
			result[j].Timestamp) },
	)
	return result
}
