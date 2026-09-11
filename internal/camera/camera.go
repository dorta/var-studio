//go:build linux

package camera

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"unsafe"
)

const (
	vidiocQuerycap        = 0x80685600
	capVideoCapture       = 0x00000001
	capVideoOutput        = 0x00000002
	capVideoCaptureMPlane = 0x00001000
	capVideoOutputMPlane  = 0x00002000
	capVideoM2M           = 0x00004000
	capVideoM2MMPlane     = 0x00008000
	capDeviceCapabilities = 0x80000000
)

type capability struct {
	Driver       [16]byte
	Card         [32]byte
	BusInfo      [32]byte
	Version      uint32
	Capabilities uint32
	DeviceCaps   uint32
	Reserved     [3]uint32
}

type Device struct {
	Path         string   `json:"path"`
	Name         string   `json:"name"`
	Driver       string   `json:"driver"`
	Bus          string   `json:"bus,omitempty"`
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"`
	Capture      bool     `json:"capture"`
}

type Inventory struct {
	Devices      []Device `json:"devices"`
	CaptureCount int      `json:"capture_count"`
	Message      string   `json:"message"`
}

func Discover() Inventory {
	paths, _ := filepath.Glob("/dev/video*")
	sort.Strings(paths)
	result := Inventory{
		Devices: make([]Device, 0, len(paths)),
	}
	for _, path := range paths {
		device, err := inspect(path)
		if err != nil {
			continue
		}
		result.Devices = append(result.Devices, device)
		if device.Capture {
			result.CaptureCount++
		}
	}
	if result.CaptureCount > 0 {
		result.Message = "Camera capture is available. Live view starts " +
			"only when requested."
	} else if len(result.Devices) > 0 {
		result.Message = "Video accelerators were detected, but no camera " +
			"capture device completed initialization."
	} else {
		result.Message = "No V4L2 video devices were detected."
	}
	return result
}

func inspect(path string) (Device, error) {
	file, err := os.OpenFile(
		path,
		os.O_RDWR|syscall.O_NONBLOCK,
		0,
	)
	if err != nil {
		file, err = os.Open(path)
		if err != nil {
			return Device{}, err
		}
	}
	defer file.Close()
	var caps capability
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		file.Fd(),
		vidiocQuerycap,
		uintptr(unsafe.Pointer(&caps)),
	)
	if errno != 0 {
		return Device{}, errno
	}
	flags := caps.Capabilities
	if flags&capDeviceCapabilities != 0 {
		flags = caps.DeviceCaps
	}
	capture := flags&(capVideoCapture|capVideoCaptureMPlane) != 0
	output := flags&(capVideoOutput|capVideoOutputMPlane) != 0
	m2m := flags&(capVideoM2M|capVideoM2MMPlane) != 0
	role := "Video device"
	switch {
	case capture && !m2m:
		role = "Camera capture"
	case m2m:
		role = "Video accelerator"
	case output:
		role = "Video output"
	}
	labels := make([]string, 0, 3)
	if capture {
		labels = append(labels, "Capture")
	}
	if output {
		labels = append(labels, "Output")
	}
	if m2m {
		labels = append(labels, "Memory-to-memory")
	}
	return Device{
		Path: path, Name: cString(caps.Card[:]), Driver: cString(caps.Driver[:]),
		Bus: cString(
			caps.BusInfo[:],
		), Role: role, Capabilities: labels, Capture: capture && !m2m,
	}, nil
}

func cString(value []byte) string {
	if index := bytes.IndexByte(value, 0); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(string(value))
}

type server struct {
	mu        sync.Mutex
	streaming bool
}

func Serve(ctx context.Context, socketPath string) error {
	if socketPath == "" {
		return errors.New(
			"camera runner socket path is empty",
		)
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o750); err != nil {
		return err
	}
	_ = os.Chown(filepath.Dir(socketPath), 0, 65532)
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
	s := &server{}
	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /v1/devices",
		func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, Discover()) },
	)
	mux.HandleFunc("GET /v1/stream", s.stream)
	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      0,
	}
	go func() { <-ctx.Done(); _ = httpServer.Shutdown(context.Background()) }()
	err = httpServer.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *server) stream(
	w http.ResponseWriter,
	r *http.Request,
) {
	requested := r.URL.Query().Get("device")
	var selected *Device
	for _, device := range Discover().Devices {
		if device.Path == requested && device.Capture {
			value := device
			selected = &value
			break
		}
	}
	if selected == nil {
		http.Error(
			w,
			"camera capture device is unavailable",
			http.StatusNotFound,
		)
		return
	}
	gstreamer := "/usr/bin/gst-launch-1.0"
	if _, err := os.Stat(gstreamer); err != nil {
		http.Error(
			w,
			"GStreamer is not installed",
			http.StatusServiceUnavailable,
		)
		return
	}
	s.mu.Lock()
	if s.streaming {
		s.mu.Unlock()
		http.Error(
			w,
			"camera is already streaming",
			http.StatusConflict,
		)
		return
	}
	s.streaming = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.streaming = false; s.mu.Unlock() }()

	streamCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	command := exec.CommandContext(
		streamCtx,
		gstreamer,
		"-q",
		"v4l2src",
		"device="+selected.Path,
		"!",
		"decodebin",
		"!",
		"videoconvert",
		"!",
		"videoscale",
		"!",
		"video/x-raw,width=1280,height=720",
		"!",
		"jpegenc",
		"quality=80",
		"!",
		"multipartmux",
		"boundary=frame",
		"!",
		"fdsink",
		"fd=1",
	)
	stdout, err := command.StdoutPipe()
	if err != nil {
		http.Error(
			w,
			"unable to prepare camera pipeline",
			http.StatusInternalServerError,
		)
		return
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		http.Error(
			w,
			fmt.Sprintf(
				"unable to start camera pipeline: %v",
				err,
			),
			http.StatusInternalServerError,
		)
		return
	}
	w.Header().
		Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().
		Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	_, _ = io.Copy(w, stdout)
	cancel()
	_ = command.Wait()
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().
		Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}
