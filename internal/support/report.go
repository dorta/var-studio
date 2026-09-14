package support

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dorta/var-studio/internal/collector"
	"github.com/dorta/var-studio/internal/history"
)

var secretPattern = regexp.MustCompile(
	`(?i)(password|passwd|token|secret|api[_-]?key)(\s*[=:]\s*)[^\s]+`,
)

var serialPattern = regexp.MustCompile(
	`(?i)(serial(?:\s+number)?\s*[=:]\s*)[^\s]+`,
)

var macPattern = regexp.MustCompile(
	`(?i)\b(?:[0-9a-f]{2}:){5}[0-9a-f]{2}\b`,
)

type Preview struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Snapshot    collector.Snapshot `json:"snapshot"`
	Metrics     history.Report     `json:"metrics"`
	Diagnostics map[string]string  `json:"diagnostics"`
}

type KernelMessage struct {
	Sequence      uint64  `json:"sequence"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	Level         string  `json:"level"`
	Message       string  `json:"message"`
}

type KernelLog struct {
	UpdatedAt time.Time       `json:"updated_at"`
	Available bool            `json:"available"`
	Messages  []KernelMessage `json:"messages"`
}

var diagnosticFiles = []struct{ name, relative, root string }{
	{"system/os-release.txt", "os-release", "etc"},
	{"system/buildinfo.txt", "buildinfo", "etc"},
	{"kernel/version.txt", "version", "proc"},
	{"kernel/cmdline.txt", "cmdline", "proc"},
	{"kernel/modules.txt", "modules", "proc"},
	{"kernel/tainted.txt", "sys/kernel/tainted", "proc"},
	{"system/cpuinfo.txt", "cpuinfo", "proc"},
	{"system/meminfo.txt", "meminfo", "proc"},
	{"system/partitions.txt", "partitions", "proc"},
	{"system/mounts.txt", "mounts", "proc"},
	{"system/interrupts.txt", "interrupts", "proc"},
	{"network/dev.txt", "net/dev", "proc"},
	{"network/route.txt", "net/route", "proc"},
	{"kernel/kernel.log", "kernel.log", "data"},
	{"kernel/kernel.previous.log", "kernel.log.1", "data"},
}

func BuildPreview(
	snapshot collector.Snapshot,
	metrics history.Report,
	paths collector.Paths,
	dataDirectory string,
) Preview {
	snapshot.Networks = append(
		[]collector.Network(nil),
		snapshot.Networks...)
	redactSnapshot(&snapshot)
	diagnostics := make(map[string]string)
	for _, file := range diagnosticFiles {
		root := map[string]string{"proc": paths.Proc, "etc": paths.Etc,
			"data": dataDirectory}[file.root]
		if value, err := readRedacted(filepath.Join(root, file.relative), 512<<10);
			err == nil {
			diagnostics[file.name] = value
		}
	}
	for _, path := range glob(filepath.Join(paths.Sys, "fs/pstore/*")) {
		if value, err := readRedacted(path, 512<<10); err == nil {
			diagnostics["kernel/pstore/"+filepath.Base(path)] = value
		}
	}
	return Preview{
		GeneratedAt: time.Now().UTC(),
		Snapshot:    snapshot,
		Metrics:     metrics,
		Diagnostics: diagnostics,
	}
}

func KernelTail(
	dataDirectory string,
	maxMessages int,
) KernelLog {
	result := KernelLog{
		UpdatedAt: time.Now().UTC(),
		Messages:  []KernelMessage{},
	}
	data, err := readTail(
		filepath.Join(dataDirectory, "kernel.log"),
		512<<10,
	)
	if err != nil {
		return result
	}
	result.Available = true
	scanner := bufio.NewScanner(
		strings.NewReader(string(data)),
	)
	for scanner.Scan() {
		if message, ok := parseKernelMessage(scanner.Text()); ok {
			result.Messages = append(
				result.Messages,
				message,
			)
		}
	}
	if maxMessages > 0 &&
		len(result.Messages) > maxMessages {
		result.Messages = result.Messages[len(result.Messages)-maxMessages:]
	}
	return result
}

func parseKernelMessage(line string) (KernelMessage, bool) {
	header, message, ok := strings.Cut(line, ";")
	if !ok || strings.TrimSpace(message) == "" {
		return KernelMessage{}, false
	}
	parts := strings.Split(header, ",")
	if len(parts) < 3 {
		return KernelMessage{}, false
	}
	priority, errPriority := strconv.Atoi(parts[0])
	sequence, errSequence := strconv.ParseUint(
		parts[1],
		10,
		64,
	)
	microseconds, errTime := strconv.ParseUint(
		parts[2],
		10,
		64,
	)
	if errPriority != nil || errSequence != nil ||
		errTime != nil {
		return KernelMessage{}, false
	}
	levels := []string{
		"EMERGENCY",
		"ALERT",
		"CRITICAL",
		"ERROR",
		"WARNING",
		"NOTICE",
		"INFO",
		"DEBUG",
	}
	return KernelMessage{
		Sequence:      sequence,
		UptimeSeconds: float64(microseconds) / 1e6,
		Level:         levels[priority&7],
		Message:       redact(strings.TrimSpace(message)),
	}, true
}

func Write(
	destination io.Writer,
	snapshot collector.Snapshot,
	metrics history.Report,
	paths collector.Paths,
	dataDirectory string,
	extras ...map[string]any,
) error {
	archive := zip.NewWriter(destination)
	readme := `VAR-Studio Support Report

Generated locally on the target board. No data was uploaded automatically.

Included: detected product and BSP, hardware/runtime inventory,
	network addresses,
metrics history, selected read-only proc/sys diagnostics,
	persistent kernel messages
when CAP_SYSLOG is available, and pstore crash records.

Excluded: environment variables, passwords, private keys, application files,
	user
home directories, Docker socket/content, ARP neighbors,
	and full process arguments.
Known credential patterns, serial numbers, and MAC addresses are redacted.
`
	if err := addText(archive, "README.txt", readme); err != nil {
		return err
	}
	redactSnapshot(&snapshot)
	if err := addJSON(archive, "inventory/snapshot.json", snapshot); err != nil {
		return err
	}
	if err := addJSON(archive, "metrics/history.json", metrics); err != nil {
		return err
	}
	for _, extra := range extras {
		for name, value := range extra {
			if err := addJSON(archive, name, value); err != nil {
				return err
			}
		}
	}
	for _, file := range diagnosticFiles {
		root := map[string]string{"proc": paths.Proc, "etc": paths.Etc,
			"data": dataDirectory}[file.root]
		_ = addFile(
			archive,
			file.name,
			filepath.Join(root, file.relative),
			10<<20,
		)
	}
	for _, path := range glob(filepath.Join(paths.Sys, "fs/pstore/*")) {
		_ = addFile(
			archive,
			"kernel/pstore/"+filepath.Base(path),
			path,
			4<<20,
		)
	}
	return archive.Close()
}

func redactSnapshot(snapshot *collector.Snapshot) {
	for index := range snapshot.Networks {
		snapshot.Networks[index].MAC = "[REDACTED]"
	}
}

func addJSON(
	archive *zip.Writer,
	name string,
	value any,
) error {
	w, err := archive.CreateHeader(
		&zip.FileHeader{
			Name:     name,
			Method:   zip.Deflate,
			Modified: time.Now().UTC(),
		},
	)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func addText(
	archive *zip.Writer,
	name, value string,
) error {
	w, err := archive.CreateHeader(
		&zip.FileHeader{
			Name:     name,
			Method:   zip.Deflate,
			Modified: time.Now().UTC(),
		},
	)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, redact(value))
	return err
}

func addFile(
	archive *zip.Writer,
	name, path string,
	limit int64,
) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return err
	}
	return addText(archive, name, string(data))
}

func readRedacted(
	path string,
	limit int64,
) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return "", err
	}
	return redact(string(data)), nil
}

func readTail(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	start := info.Size() - limit
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if start > 0 {
		if newline := strings.IndexByte(string(data), '\n'); newline >= 0 {
			data = data[newline+1:]
		}
	}
	return data, nil
}

func redact(value string) string {
	value = secretPattern.ReplaceAllString(
		value,
		`$1$2[REDACTED]`,
	)
	value = serialPattern.ReplaceAllString(
		value,
		`${1}[REDACTED]`,
	)
	return macPattern.ReplaceAllString(
		value,
		"[MAC REDACTED]",
	)
}

func glob(
	pattern string,
) []string {
	matches, _ := filepath.Glob(pattern)
	return matches
}

func Filename(hostname string) string {
	hostname = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '-' {
			return r
		}
		return '-'
	}, hostname)
	return fmt.Sprintf(
		"var-studio-support-%s-%s.zip",
		hostname,
		time.Now().UTC().Format("20060102-150405"),
	)
}
