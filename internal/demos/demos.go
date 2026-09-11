package demos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Demo struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	API         string `json:"api"`
	DurationSec int    `json:"duration_seconds"`
	Installed   bool   `json:"installed"`
	command     string
	args        []string
}

type Status struct {
	State       string    `json:"state"`
	DemoID      string    `json:"demo_id,omitempty"`
	DemoName    string    `json:"demo_name,omitempty"`
	DurationSec int       `json:"duration_seconds,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	FinishedAt  time.Time `json:"finished_at,omitempty"`
	ExitCode    int       `json:"exit_code,omitempty"`
	Output      string    `json:"output,omitempty"`
}

var Catalog = []Demo{
	{
		ID:   "medical-vitals",
		Kind: "display-medical",
		Name: "Medical Vitals",
		Description: "Animated bedside monitoring experience " +
			"on the EVK display.",
		API:         "Qt 6 Widgets · Wayland",
		DurationSec: 180,
		command:     "/opt/var-studio-demos/bin/medical-vitals",
	},
	{
		ID:   "automotive-cluster",
		Kind: "display-automotive",
		Name: "Automotive Cluster",
		Description: "Digital cockpit and vehicle telemetry " +
			"experience on the EVK display.",
		API:         "Qt 6 Widgets · Wayland",
		DurationSec: 180,
		command:     "/opt/var-studio-demos/bin/automotive-cluster",
	},
	{
		ID:          "triangle",
		Kind:        "gpu",
		Name:        "Simple Triangle",
		Description: "Quick OpenGL ES 2.0 rendering validation.",
		API:         "OpenGL ES 2.0",
		DurationSec: 15,
		command: "/opt/imx-gpu-sdk/GLES2/S01_SimpleTriangle___Wayl" +
			"and/GLES2.S01_SimpleTriangle___Wayland",
		args: []string{
			"--ExitAfterFrame",
			"900",
			"--LogStatsMode",
			"average",
		},
	},
	{
		ID:          "bloom",
		Kind:        "gpu",
		Name:        "Bloom",
		Description: "Moderate OpenGL ES 3.0 post-processing workload.",
		API:         "OpenGL ES 3.0",
		DurationSec: 20,
		command: "/opt/imx-gpu-sdk/GLES3/Bloom___Wayland/" +
			"GLES3.Bloom___Wayland",
		args: []string{
			"--ExitAfterFrame",
			"1200",
			"--LogStatsMode",
			"average",
		},
	},
	{
		ID:          "stress",
		Kind:        "gpu",
		Name:        "T3D Stress Test",
		Description: "Sustained OpenGL ES 3.0 graphics stress " + "workload.",
		API:         "OpenGL ES 3.0",
		DurationSec: 30,
		command: "/opt/imx-gpu-sdk/GLES3/T3DStressTest___Wayland/" +
			"GLES3.T3DStressTest___Wayland",
		args: []string{
			"--ExitAfterFrame",
			"1800",
			"--LogStatsMode",
			"average",
		},
	},
	{
		ID:          "glmark2",
		Kind:        "gpu",
		Name:        "glmark2 Build",
		Description: "Standard glmark2 geometry benchmark scene.",
		API:         "OpenGL ES 2.0",
		DurationSec: 20,
		command:     "/usr/bin/glmark2-es2-wayland",
		args: []string{
			"--benchmark",
			"build:duration=20.0",
			"--results",
			"fps:cpu:shader",
		},
	},
	{
		ID:   "mobilenet-vx",
		Kind: "ml",
		Name: "MobileNet NPU Classification",
		Description: "Classify the packaged Grace Hopper image with " +
			"TensorFlow Lite and the Vivante VX delegate.",
		API:         "TensorFlow Lite · VX delegate",
		DurationSec: 8,
	},
}

func resolveDemo(template Demo) Demo {
	demo := template
	if demo.Kind == "ml" {
		candidates, _ := filepath.Glob(
			"/usr/bin/tensorflow-lite-*/examples/label_image",
		)
		sort.Strings(candidates)
		return resolveMLDemo(
			demo,
			candidates,
			"/usr/lib/libvx_delegate.so",
		)
	}
	demo.Installed = executable(demo.command)
	return demo
}

func resolveMLDemo(
	demo Demo,
	candidates []string,
	delegate string,
) Demo {
	for _, command := range candidates {
		base := filepath.Dir(command)
		model := filepath.Join(
			base,
			"mobilenet_v1_1.0_224_quant.tflite",
		)
		image := filepath.Join(base, "grace_hopper.bmp")
		labels := filepath.Join(base, "labels.txt")
		if executable(command) &&
			filesExist(model, image, labels, delegate) {
			demo.command = command
			demo.args = []string{
				"--external_delegate_path=" + delegate,
				"-m",
				model,
				"-i",
				image,
				"-l",
				labels,
				"-c",
				"100",
				"-w",
				"1",
				"-r",
				"5",
			}
			demo.Installed = true
			return demo
		}
	}
	return demo
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() &&
		info.Mode().Perm()&0o111 != 0
}

func filesExist(paths ...string) bool {
	for _, path := range paths {
		if info, err := os.Stat(path); err != nil ||
			!info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

type manager struct {
	mu     sync.Mutex
	status Status
	cancel context.CancelFunc
}

func Serve(ctx context.Context, socketPath string) error {
	if socketPath == "" {
		return errors.New(
			"demo runner socket path is empty",
		)
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o750); err != nil {
		return err
	}
	if err := os.Chown(filepath.Dir(socketPath), 0, 65532); err != nil {
		return err
	}
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer listener.Close()
	if err := os.Chmod(socketPath, 0o660); err != nil {
		return err
	}
	_ = os.Chown(socketPath, 0, 65532)
	m := &manager{status: Status{State: "idle"}}
	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /v1/demos",
		func(w http.ResponseWriter, _ *http.Request) {
			catalog := make([]Demo, len(Catalog))
			copy(catalog, Catalog)
			for index := range catalog {
				catalog[index] = resolveDemo(catalog[index])
			}
			writeJSON(w, catalog)
		},
	)
	mux.HandleFunc(
		"GET /v1/status",
		func(w http.ResponseWriter, _ *http.Request) {
			m.mu.Lock()
			defer m.mu.Unlock()
			writeJSON(w, m.status)
		},
	)
	mux.HandleFunc("POST /v1/run", m.run)
	mux.HandleFunc("POST /v1/stop", m.stop)
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	go func() { <-ctx.Done(); _ = server.Shutdown(context.Background()) }()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (m *manager) run(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Header.Get("X-VAR-Studio-Action") != "gpu-demo" {
		http.Error(
			w,
			"action header required",
			http.StatusForbidden,
		)
		return
	}
	var request struct {
		ID string `json:"id"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1024)).
		Decode(&request) !=
		nil {
		http.Error(
			w,
			"invalid request",
			http.StatusBadRequest,
		)
		return
	}
	var selected *Demo
	for index := range Catalog {
		if Catalog[index].ID == request.ID {
			resolved := resolveDemo(Catalog[index])
			selected = &resolved
			break
		}
	}
	if selected == nil {
		http.Error(w, "unknown demo", http.StatusNotFound)
		return
	}
	if !selected.Installed {
		http.Error(
			w,
			"demo is not installed",
			http.StatusNotFound,
		)
		return
	}
	m.mu.Lock()
	if m.status.State == "running" {
		m.mu.Unlock()
		http.Error(
			w,
			"another demo is already running",
			http.StatusConflict,
		)
		return
	}
	runCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(selected.DurationSec+5)*time.Second,
	)
	command := exec.CommandContext(
		runCtx,
		selected.command,
		selected.args...)
	command.Dir = filepath.Dir(selected.command)
	command.Env = displayEnvironment()
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	output := &limitedBuffer{limit: 8 << 10}
	command.Stdout, command.Stderr = output, output
	if err := command.Start(); err != nil {
		cancel()
		m.mu.Unlock()
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}
	m.cancel = func() {
		cancel()
		if command.Process != nil {
			_ = syscall.Kill(
				-command.Process.Pid,
				syscall.SIGTERM,
			)
		}
	}
	m.status = Status{
		State:       "running",
		DemoID:      selected.ID,
		DemoName:    selected.Name,
		DurationSec: selected.DurationSec,
		StartedAt:   time.Now().UTC(),
	}
	status := m.status
	m.mu.Unlock()
	go m.wait(command, output, selected)
	writeJSON(w, status)
}

func displayEnvironment() []string {
	runtimeDir := "/run/user/0"
	display := "wayland-1"
	if configured := os.Getenv("WAYLAND_DISPLAY"); configured != "" {
		display = configured
	}
	sockets, _ := filepath.Glob("/run/user/*/wayland-*")
	sort.Strings(sockets)
	for _, socket := range sockets {
		if strings.HasSuffix(socket, ".lock") {
			continue
		}
		info, err := os.Stat(socket)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			runtimeDir = filepath.Dir(socket)
			display = filepath.Base(socket)
			break
		}
	}
	return append(
		os.Environ(),
		"XDG_RUNTIME_DIR="+runtimeDir,
		"WAYLAND_DISPLAY="+display,
		"QT_QPA_PLATFORM=wayland",
	)
}

func (m *manager) wait(
	command *exec.Cmd,
	output *limitedBuffer,
	selected *Demo,
) {
	err := command.Wait()
	exitCode, state := 0, "completed"
	if err != nil {
		state = "failed"
		if exit, ok := err.(*exec.ExitError); ok {
			exitCode = exit.ExitCode()
		} else {
			exitCode = -1
		}
	}
	m.mu.Lock()
	m.status = Status{
		State:       state,
		DemoID:      selected.ID,
		DemoName:    selected.Name,
		DurationSec: selected.DurationSec,
		StartedAt:   m.status.StartedAt,
		FinishedAt:  time.Now().UTC(),
		ExitCode:    exitCode,
		Output:      output.String(),
	}
	m.cancel = nil
	m.mu.Unlock()
}

func (m *manager) stop(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Header.Get("X-VAR-Studio-Action") != "gpu-demo" {
		http.Error(
			w,
			"action header required",
			http.StatusForbidden,
		)
		return
	}
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel == nil {
		http.Error(
			w,
			"no demo is running",
			http.StatusConflict,
		)
		return
	}
	cancel()
	writeJSON(w, map[string]string{"status": "stopping"})
}

type limitedBuffer struct {
	mu    sync.Mutex
	data  bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.data.Len()
	if remaining > 0 {
		_, _ = b.data.Write(
			value[:min(len(value), remaining)],
		)
	}
	return len(value), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.String()
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
