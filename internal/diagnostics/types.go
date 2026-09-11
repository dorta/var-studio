package diagnostics

import "time"

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Event struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Category  string         `json:"category"`
	Severity  Severity       `json:"severity"`
	Title     string         `json:"title"`
	Detail    string         `json:"detail"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Check struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Summary  string   `json:"summary"`
	Evidence string   `json:"evidence,omitempty"`
	Severity Severity `json:"severity"`
}

type Health struct {
	Status      string    `json:"status"`
	Summary     string    `json:"summary"`
	EvaluatedAt time.Time `json:"evaluated_at"`
	Passed      int       `json:"passed"`
	Warnings    int       `json:"warnings"`
	Critical    int       `json:"critical"`
	Checks      []Check   `json:"checks"`
}

type Capability struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Available bool   `json:"available"`
	Count     int    `json:"count,omitempty"`
	Detail    string `json:"detail"`
}

type GuideStep struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

type Guide struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Conclusion  string      `json:"conclusion"`
	Steps       []GuideStep `json:"steps"`
}

type Action struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Available   bool   `json:"available"`
	Unavailable string `json:"unavailable_reason,omitempty"`
}

type Result struct {
	ActionID  string      `json:"action_id"`
	Status    string      `json:"status"`
	Summary   string      `json:"summary"`
	StartedAt time.Time   `json:"started_at"`
	Duration  int64       `json:"duration_ms"`
	Checks    []GuideStep `json:"checks"`
}

type Dashboard struct {
	Health       Health       `json:"health"`
	Events       []Event      `json:"events"`
	Capabilities []Capability `json:"capabilities"`
	Guides       []Guide      `json:"guides"`
	Actions      []Action     `json:"actions"`
}
