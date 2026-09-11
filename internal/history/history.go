package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const historyFile = "metrics.jsonl"

type Sample struct {
	Timestamp     time.Time `json:"timestamp"`
	CPUPercent    float64   `json:"cpu_percent"`
	PerCore       []float64 `json:"per_core"`
	MemoryPercent float64   `json:"memory_percent"`
}

type Report struct {
	BootID            string    `json:"boot_id"`
	StartedAt         time.Time `json:"started_at"`
	SampleIntervalSec int       `json:"sample_interval_seconds"`
	Samples           []Sample  `json:"samples"`
}

type header struct {
	BootID    string    `json:"boot_id"`
	StartedAt time.Time `json:"started_at"`
}

type Store struct {
	mu        sync.RWMutex
	bootID    string
	startedAt time.Time
	interval  time.Duration
	samples   []Sample
	file      *os.File
}

func Open(
	directory, bootID string,
	interval time.Duration,
) (*Store, error) {
	if directory == "" {
		return nil, errors.New("history directory is empty")
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, err
	}
	path := filepath.Join(directory, historyFile)
	store := &Store{
		bootID:    bootID,
		interval:  interval,
		startedAt: time.Now().UTC(),
		samples:   []Sample{},
	}
	if err := store.load(path); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o640,
	)
	if err != nil {
		return nil, err
	}
	store.file = file
	return store, nil
}

func (s *Store) load(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return s.reset(path)
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var existing header
	if !scanner.Scan() ||
		json.Unmarshal(scanner.Bytes(), &existing) != nil ||
		existing.BootID != s.bootID {
		return s.reset(path)
	}
	s.startedAt = existing.StartedAt
	for scanner.Scan() {
		var sample Sample
		if json.Unmarshal(
			scanner.Bytes(),
			&sample,
		) == nil &&
			!sample.Timestamp.IsZero() {
			s.samples = append(s.samples, sample)
		}
	}
	return scanner.Err()
}

func (s *Store) reset(path string) error {
	s.startedAt = time.Now().UTC()
	s.samples = []Sample{}
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		0o640,
	)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).
		Encode(header{BootID: s.bootID, StartedAt: s.startedAt})
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func (s *Store) Append(sample Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sample.PerCore = append(
		[]float64(nil),
		sample.PerCore...)
	s.samples = append(s.samples, sample)
	return json.NewEncoder(s.file).Encode(sample)
}

func (s *Store) Report(maxPoints int) Report {
	return s.ReportSince(time.Time{}, maxPoints)
}

func (s *Store) ReportSince(
	since time.Time,
	maxPoints int,
) Report {
	s.mu.RLock()
	defer s.mu.RUnlock()
	samples := s.samples
	if !since.IsZero() {
		index := sort.Search(
			len(samples),
			func(index int) bool { return !samples[index].Timestamp.Before(since) },
		)
		samples = samples[index:]
	}
	return Report{
		BootID:            s.bootID,
		StartedAt:         s.startedAt,
		SampleIntervalSec: int(s.interval.Seconds()),
		Samples:           downsample(samples, maxPoints),
	}
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	return s.file.Close()
}

func downsample(samples []Sample, maxPoints int) []Sample {
	if maxPoints <= 0 || len(samples) <= maxPoints {
		result := make([]Sample, len(samples))
		copy(result, samples)
		return result
	}
	result := make([]Sample, 0, maxPoints)
	for bucket := 0; bucket < maxPoints; bucket++ {
		start := bucket * len(samples) / maxPoints
		end := (bucket + 1) * len(samples) / maxPoints
		if end <= start {
			continue
		}
		result = append(result, average(samples[start:end]))
	}
	return result
}

func average(samples []Sample) Sample {
	result := Sample{
		Timestamp: samples[len(samples)-1].Timestamp,
	}
	cores := 0
	for _, sample := range samples {
		if len(sample.PerCore) > cores {
			cores = len(sample.PerCore)
		}
	}
	result.PerCore = make([]float64, cores)
	counts := make([]int, cores)
	for _, sample := range samples {
		result.CPUPercent += sample.CPUPercent
		result.MemoryPercent += sample.MemoryPercent
		for core, value := range sample.PerCore {
			result.PerCore[core] += value
			counts[core]++
		}
	}
	result.CPUPercent /= float64(len(samples))
	result.MemoryPercent /= float64(len(samples))
	for core := range result.PerCore {
		if counts[core] > 0 {
			result.PerCore[core] /= float64(counts[core])
		}
	}
	return result
}
