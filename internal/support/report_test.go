package support

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorta/var-studio/internal/collector"
	"github.com/dorta/var-studio/internal/history"
)

func TestReportRedactsSensitiveValues(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"proc/sys/kernel", "proc/net", "etc",
		"sys/fs/pstore", "data"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "proc/cmdline"), []byte(
		"console=tty password=visible serial=ABC123\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data/kernel.log"), []byte(
		"device aa:bb:cc:dd:ee:ff token=visible\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	snapshot := collector.Snapshot{
		System: collector.SystemInfo{Hostname: "board"},
		Networks: []collector.Network{
			{MAC: "aa:bb:cc:dd:ee:ff"},
		},
	}
	var output bytes.Buffer
	err := Write(
		&output,
		snapshot,
		history.Report{},
		collector.Paths{
			Proc: filepath.Join(root, "proc"),
			Sys:  filepath.Join(root, "sys"),
			Etc:  filepath.Join(root, "etc"),
		},
		filepath.Join(root, "data"),
	)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(
		bytes.NewReader(output.Bytes()),
		int64(output.Len()),
	)
	if err != nil {
		t.Fatal(err)
	}
	all := ""
	for _, file := range archive.File {
		reader, _ := file.Open()
		data, _ := io.ReadAll(reader)
		reader.Close()
		all += string(data)
	}
	if strings.Contains(all, "visible") ||
		strings.Contains(all, "aa:bb:cc:dd:ee:ff") ||
		strings.Contains(all, "ABC123") {
		t.Fatalf(
			"support report leaked a redacted value: %s",
			all,
		)
	}
}

func TestPreviewAndKernelTailAreRedacted(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"proc", "etc", "sys/fs/pstore", "data"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "etc/buildinfo"), []byte(
		"token=visible\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data/kernel.log"), []byte(
		"6,42,1500000,-;device aa:bb:cc:dd:ee:ff "+"password=visible\n"), 0o640);
			err != nil {
		t.Fatal(err)
	}
	paths := collector.Paths{
		Proc: filepath.Join(root, "proc"),
		Sys:  filepath.Join(root, "sys"),
		Etc:  filepath.Join(root, "etc"),
	}
	snapshot := collector.Snapshot{
		Networks: []collector.Network{
			{MAC: "aa:bb:cc:dd:ee:ff"},
		},
	}
	preview := BuildPreview(
		snapshot,
		history.Report{},
		paths,
		filepath.Join(root, "data"),
	)
	encoded, _ := json.Marshal(preview)
	if strings.Contains(string(encoded), "visible") ||
		strings.Contains(
			string(encoded),
			"aa:bb:cc:dd:ee:ff",
		) {
		t.Fatalf(
			"preview leaked a redacted value: %s",
			encoded,
		)
	}
	log := KernelTail(filepath.Join(root, "data"), 20)
	if !log.Available || len(log.Messages) != 1 {
		t.Fatalf("unexpected kernel log: %+v", log)
	}
	message := log.Messages[0]
	if message.Level != "INFO" || message.Sequence != 42 ||
		message.UptimeSeconds != 1.5 {
		t.Fatalf(
			"kernel metadata was not parsed: %+v",
			message,
		)
	}
	if strings.Contains(message.Message, "visible") ||
		strings.Contains(
			message.Message,
			"aa:bb:cc:dd:ee:ff",
		) {
		t.Fatalf(
			"kernel message leaked a redacted value: %s",
			message.Message,
		)
	}
}
