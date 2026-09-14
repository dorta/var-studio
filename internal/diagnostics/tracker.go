package diagnostics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dorta/var-studio/internal/collector"
	"github.com/dorta/var-studio/internal/support"
)

type Tracker struct {
	mu             sync.Mutex
	path           string
	events         []Event
	initialized    bool
	networkStates  map[string]string
	thermalStates  map[string]string
	kernelSequence uint64
}

func NewTracker(dataDirectory, bootID string) *Tracker {
	tracker := &Tracker{
		path: filepath.Join(
			dataDirectory,
			"events-"+safeID(bootID)+".jsonl",
		),
		networkStates: map[string]string{},
		thermalStates: map[string]string{},
	}
	tracker.load()
	return tracker
}

func (t *Tracker) Observe(
	snapshot collector.Snapshot,
	kernel support.KernelLog,
) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.initialized {
		t.appendLocked(
			Event{
				Timestamp: snapshot.Timestamp,
				Category:  "system",
				Severity:  SeverityInfo,
				Title:     "VAR-Studio monitoring started",
				Detail: fmt.Sprintf(
					"%s · kernel %s",
					snapshot.System.Model,
					snapshot.System.Kernel,
				),
			},
		)
		t.initialized = true
	}
	for _, network := range snapshot.Networks {
		previous, known := t.networkStates[network.Name]
		if known && previous != network.State {
			severity := SeverityInfo
			if network.State != "up" {
				severity = SeverityWarning
			}
			t.appendLocked(
				Event{
					Timestamp: snapshot.Timestamp,
					Category:  "network",
					Severity:  severity,
					Title:     network.Name + " link changed",
					Detail: strings.ToUpper(
						network.State,
					),
					Metadata: map[string]any{
						"interface":      network.Name,
						"previous_state": previous,
						"state":          network.State,
					},
				},
			)
		}
		t.networkStates[network.Name] = network.State
	}
	for _, sensor := range snapshot.Thermals {
		state := thermalBand(sensor.Celsius)
		previous, known := t.thermalStates[sensor.Name]
		if known && previous != state &&
			(state == "warning" || state == "critical" || previous == "warning" ||
				previous == "critical") {
			severity := SeverityInfo
			if state == "warning" {
				severity = SeverityWarning
			}
			if state == "critical" {
				severity = SeverityCritical
			}
			t.appendLocked(
				Event{
					Timestamp: snapshot.Timestamp,
					Category:  "thermal",
					Severity:  severity,
					Title:     sensor.Name + " thermal state",
					Detail: fmt.Sprintf(
						"%.1f°C · %s",
						sensor.Celsius,
						state,
					),
					Metadata: map[string]any{
						"sensor":  sensor.Name,
						"celsius": sensor.Celsius,
						"state":   state,
					},
				},
			)
		}
		t.thermalStates[sensor.Name] = state
	}
	for _, message := range kernel.Messages {
		if message.Sequence <= t.kernelSequence {
			continue
		}
		t.kernelSequence = message.Sequence
		severity := SeverityInfo
		if message.Level == "WARNING" {
			severity = SeverityWarning
		}
		if message.Level == "ERROR" ||
			message.Level == "CRITICAL" ||
			message.Level == "ALERT" ||
			message.Level == "EMERGENCY" {
			severity = SeverityCritical
		}
		if severity != SeverityInfo {
			timestamp := snapshot.Timestamp
			if snapshot.System.UptimeSec > 0 &&
				message.UptimeSeconds > 0 {
				bootedAt := snapshot.Timestamp.Add(
					-time.Duration(
						snapshot.System.UptimeSec * float64(
							time.Second,
						),
					),
				)
				timestamp = bootedAt.Add(
					time.Duration(
						message.UptimeSeconds * float64(
							time.Second,
						),
					),
				)
			}
			t.appendLocked(
				Event{
					Timestamp: timestamp,
					Category:  "kernel",
					Severity:  severity,
					Title: "Kernel " + strings.ToLower(
						message.Level,
					),
					Detail: message.Message,
					Metadata: map[string]any{
						"sequence":       message.Sequence,
						"uptime_seconds": message.UptimeSeconds,
					},
				},
			)
		}
	}
}

func (t *Tracker) Add(
	category string,
	severity Severity,
	title, detail string,
	metadata map[string]any,
) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.appendLocked(
		Event{
			Timestamp: time.Now().UTC(),
			Category:  category,
			Severity:  severity,
			Title:     title,
			Detail:    detail,
			Metadata:  metadata,
		},
	)
}

func (t *Tracker) Events(limit int) []Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	if limit <= 0 || limit > len(t.events) {
		limit = len(t.events)
	}
	start := len(t.events) - limit
	result := append([]Event(nil), t.events[start:]...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1,
		right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func (t *Tracker) appendLocked(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	event.ID = fmt.Sprintf(
		"%d-%d",
		event.Timestamp.UnixNano(),
		len(t.events),
	)
	t.events = append(t.events, event)
	if len(t.events) > 500 {
		t.events = append(
			[]Event(nil),
			t.events[len(t.events)-500:]...)
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o750); err != nil {
		return
	}
	file, err := os.OpenFile(
		t.path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o640,
	)
	if err != nil {
		return
	}
	defer file.Close()
	_ = json.NewEncoder(file).Encode(event)
}

func (t *Tracker) load() {
	file, err := os.Open(t.path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if json.Unmarshal(scanner.Bytes(), &event) == nil {
			t.events = append(t.events, event)
		}
	}
	if len(t.events) > 500 {
		t.events = append(
			[]Event(nil),
			t.events[len(t.events)-500:]...)
	}
}

func thermalBand(celsius float64) string {
	switch {
	case celsius >= 95:
		return "critical"
	case celsius >= 85:
		return "warning"
	default:
		return "normal"
	}
}

func safeID(value string) string {
	return strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' {
			return character
		}
		return '-'
	}, value)
}
