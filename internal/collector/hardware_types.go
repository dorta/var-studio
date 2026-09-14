package collector

import "time"

type HardwareReport struct {
	Timestamp time.Time         `json:"timestamp"`
	Sections  []HardwareSection `json:"sections"`
}

type HardwareSection struct {
	ID     string          `json:"id"`
	Group  string          `json:"group"`
	Label  string          `json:"label"`
	Icon   string          `json:"icon"`
	Fields []HardwareField `json:"fields"`
	Tables []HardwareTable `json:"tables"`
}

type HardwareField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type HardwareTable struct {
	Title   string     `json:"title"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}
