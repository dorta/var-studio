package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dorta/var-studio/internal/camera"
	"github.com/dorta/var-studio/internal/collector"
	"github.com/dorta/var-studio/internal/commits"
	"github.com/dorta/var-studio/internal/demos"
	"github.com/dorta/var-studio/internal/diagnostics"
	"github.com/dorta/var-studio/internal/gpu"
	"github.com/dorta/var-studio/internal/history"
	"github.com/dorta/var-studio/internal/npu"
	"github.com/dorta/var-studio/internal/support"
	"github.com/dorta/var-studio/internal/supportlog"
)

//go:embed web/*
var assets embed.FS

func main() {
	addr := env("LISTEN_ADDR", ":9090")
	paths := collector.Paths{
		Proc: env("HOST_PROC", "/host/proc"),
		Sys:  env("HOST_SYS", "/host/sys"),
		Etc:  env("HOST_ETC", "/host/etc"),
	}

	c := collector.New(paths)
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	bootID := strings.TrimSpace(
		readFile(paths.Proc + "/sys/kernel/random/boot_id"),
	)
	if bootID == "" {
		bootID = "unknown-boot"
	}
	const historyInterval = 5 * time.Second
	dataDirectory := env("HISTORY_DIR", "/data")
	if env(
		"SERVICE_MODE",
		"dashboard",
	) == "kernel-log-spooler" {
		if err := supportlog.Start(ctx, dataDirectory, bootID, env("HOST_KMSG",
			"/host/dev/kmsg")); err != nil {
			slog.Error(
				"kernel log collection unavailable",
				"error",
				err,
			)
			os.Exit(1)
		}
		slog.Info("kernel log spooler started")
		<-ctx.Done()
		return
	}
	if env(
		"SERVICE_MODE",
		"dashboard",
	) == "gpu-metrics-spooler" {
		if err := gpu.Start(ctx, dataDirectory, bootID, env("GPUTOP_PATH",
			"/usr/bin/gputop")); err != nil {
			slog.Error(
				"GPU metrics collection unavailable",
				"error",
				err,
			)
			os.Exit(1)
		}
		return
	}
	if env(
		"SERVICE_MODE",
		"dashboard",
	) == "npu-metrics-spooler" {
		if err := npu.Start(ctx, dataDirectory, bootID, env("HOST_SYS",
			"/host/sys")); err != nil {
			slog.Error(
				"NPU metrics collection unavailable",
				"error",
				err,
			)
			os.Exit(1)
		}
		return
	}
	if env(
		"SERVICE_MODE",
		"dashboard",
	) == "gpu-demo-runner" {
		if err := demos.Serve(ctx, env("DEMO_SOCKET", "/run/var-studio/demo.sock"));
			err != nil {
			slog.Error(
				"GPU demo runner unavailable",
				"error",
				err,
			)
			os.Exit(1)
		}
		return
	}
	if env("SERVICE_MODE", "dashboard") == "camera-runner" {
		if err := camera.Serve(ctx, env("CAMERA_SOCKET",
			"/run/var-studio-camera/camera.sock")); err != nil {
			slog.Error(
				"camera runner unavailable",
				"error",
				err,
			)
			os.Exit(1)
		}
		return
	}
	diagnosticEngine := diagnostics.New(paths)
	eventTracker := diagnostics.NewTracker(
		dataDirectory,
		bootID,
	)
	commitResolver := commits.New()

	metricHistory, err := history.Open(
		dataDirectory,
		bootID,
		historyInterval,
	)
	if err != nil {
		slog.Error("open metrics history", "error", err)
		os.Exit(1)
	}
	defer metricHistory.Close()
	recordHistory := func() {
		snapshot := c.Snapshot()
		eventTracker.Observe(
			snapshot,
			support.KernelTail(dataDirectory, 200),
		)
		memoryPercent := 0.0
		if snapshot.Metrics.MemoryTotal > 0 {
			memoryPercent = 100 * float64(
				snapshot.Metrics.MemoryUsed,
			) / float64(
				snapshot.Metrics.MemoryTotal,
			)
		}
		if err := metricHistory.Append(history.Sample{
			Timestamp:     snapshot.Timestamp,
			CPUPercent:    snapshot.Metrics.CPUPercent,
			PerCore:       snapshot.Metrics.PerCore,
			MemoryPercent: memoryPercent,
		}); err != nil {
			slog.Warn(
				"append metrics history",
				"error",
				err,
			)
		}
	}
	recordHistory()
	go func() {
		ticker := time.NewTicker(historyInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				recordHistory()
			case <-ctx.Done():
				return
			}
		}
	}()

	web, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /api/v1/snapshot",
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, c.Snapshot())
		},
	)
	mux.HandleFunc(
		"GET /api/v1/commit-metadata",
		func(w http.ResponseWriter, r *http.Request) {
			snapshot := c.Snapshot()
			references := make(
				[]commits.Reference,
				0,
				len(snapshot.BSP.Layers),
			)
			for _, layer := range snapshot.BSP.Layers {
				references = append(
					references,
					commits.Reference{
						Name:        layer.Name,
						Revision:    layer.Revision,
						RevisionURL: layer.RevisionURL,
					},
				)
			}
			w.Header().
				Set("Cache-Control", "private, max-age=3600")
			writeJSON(
				w,
				http.StatusOK,
				commitResolver.ResolveAll(
					r.Context(),
					references,
				),
			)
		},
	)
	mux.HandleFunc(
		"GET /api/v1/hardware",
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, c.Hardware())
		},
	)
	mux.HandleFunc(
		"GET /api/v1/history",
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(
				w,
				http.StatusOK,
				metricHistory.ReportSince(
					historyStart(
						r.URL.Query().Get("range"),
					),
					720,
				),
			)
		},
	)
	mux.HandleFunc(
		"GET /api/v1/gpu-history",
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(
				w,
				http.StatusOK,
				gpu.Read(
					dataDirectory,
					bootID,
					historyStart(
						r.URL.Query().Get("range"),
					),
					720,
				),
			)
		},
	)
	mux.HandleFunc(
		"GET /api/v1/npu-history",
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(
				w,
				http.StatusOK,
				npu.Read(
					dataDirectory,
					bootID,
					historyStart(
						r.URL.Query().Get("range"),
					),
					720,
				),
			)
		},
	)
	demoSocket := env(
		"DEMO_SOCKET",
		"/run/var-studio-demo/demo.sock",
	)
	mux.HandleFunc(
		"GET /api/v1/gpu-demos",
		func(w http.ResponseWriter, r *http.Request) { proxyDemo(w, r, demoSocket,
			"/v1/demos") },
	)
	mux.HandleFunc(
		"GET /api/v1/gpu-demo-status",
		func(w http.ResponseWriter, r *http.Request) { proxyDemo(w, r, demoSocket,
			"/v1/status") },
	)
	mux.HandleFunc(
		"POST /api/v1/gpu-demo-run",
		func(w http.ResponseWriter, r *http.Request) { proxyDemo(w, r, demoSocket,
			"/v1/run") },
	)
	mux.HandleFunc(
		"POST /api/v1/gpu-demo-stop",
		func(w http.ResponseWriter, r *http.Request) { proxyDemo(w, r, demoSocket,
			"/v1/stop") },
	)
	cameraSocket := env(
		"CAMERA_SOCKET",
		"/run/var-studio-camera/camera.sock",
	)
	mux.HandleFunc(
		"GET /api/v1/cameras",
		func(w http.ResponseWriter, r *http.Request) { proxyCamera(w, r,
			cameraSocket, "/v1/devices", false) },
	)
	mux.HandleFunc(
		"GET /api/v1/camera-stream",
		func(w http.ResponseWriter, r *http.Request) {
			proxyCamera(
				w,
				r,
				cameraSocket,
				"/v1/stream?"+r.URL.RawQuery,
				true,
			)
		},
	)
	mux.HandleFunc(
		"GET /api/v1/support-preview",
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(
				w,
				http.StatusOK,
				support.BuildPreview(
					c.Snapshot(),
					metricHistory.Report(1440),
					paths,
					dataDirectory,
				),
			)
		},
	)
	mux.HandleFunc(
		"GET /api/v1/kernel-log",
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(
				w,
				http.StatusOK,
				support.KernelTail(dataDirectory, 200),
			)
		},
	)
	mux.HandleFunc(
		"GET /api/v1/support-report",
		func(w http.ResponseWriter, r *http.Request) {
			snapshot := c.Snapshot()
			w.Header().
				Set("Content-Type", "application/zip")
			w.Header().
				Set("Content-Disposition", `attachment; filename="`+support.Filename(
					snapshot.System.Hostname)+`"`)
			w.Header().Set("Cache-Control", "no-store")
			dashboard := diagnosticEngine.Build(
				snapshot,
				support.KernelTail(dataDirectory, 200),
				eventTracker.Events(500),
			)
			if err := support.Write(w, snapshot, metricHistory.Report(1440), paths,
				dataDirectory, map[string]any{
				"diagnostics/health.json":       dashboard.Health,
				"diagnostics/events.json":       dashboard.Events,
				"diagnostics/capabilities.json": dashboard.Capabilities,
				"metrics/npu-history.json":      npu.Read(dataDirectory, bootID,
					time.Time{}, 1440),
			}); err != nil {
				slog.Warn(
					"generate support report",
					"error",
					err,
				)
			}
		},
	)
	mux.HandleFunc(
		"GET /api/v1/health",
		func(w http.ResponseWriter, r *http.Request) {
			dashboard := diagnosticEngine.Build(
				c.Snapshot(),
				support.KernelTail(dataDirectory, 200),
				eventTracker.Events(100),
			)
			writeJSON(w, http.StatusOK, dashboard.Health)
		},
	)
	mux.HandleFunc(
		"GET /api/v1/diagnostics",
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(
				w,
				http.StatusOK,
				diagnosticEngine.Build(
					c.Snapshot(),
					support.KernelTail(dataDirectory, 200),
					eventTracker.Events(100),
				),
			)
		},
	)
	mux.HandleFunc(
		"GET /api/v1/events",
		func(w http.ResponseWriter, r *http.Request) {
			writeJSON(
				w,
				http.StatusOK,
				eventTracker.Events(200),
			)
		},
	)
	mux.HandleFunc(
		"POST /api/v1/diagnostic-run",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(
				"X-VAR-Studio-Action",
			) != "diagnostic" {
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
			result, ok := diagnosticEngine.Run(
				request.ID,
				c.Snapshot(),
				support.KernelTail(dataDirectory, 200),
			)
			if !ok {
				http.Error(
					w,
					"unknown diagnostic action",
					http.StatusNotFound,
				)
				return
			}
			eventTracker.Add(
				"diagnostic",
				diagnostics.SeverityInfo,
				"Diagnostic completed",
				result.Summary,
				map[string]any{
					"action_id": request.ID,
					"status":    result.Status,
				},
			)
			writeJSON(w, http.StatusOK, result)
		},
	)
	mux.Handle("GET /", http.FileServer(http.FS(web)))

	server := &http.Server{
		Addr: addr,
		Handler: securityHeaders(
			requestLogger(mux),
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown", "error", err)
		}
	}()

	slog.Info("VAR-Studio listening", "address", addr)
	if err := server.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func historyStart(selected string) time.Time {
	durations := map[string]time.Duration{
		"hour":  time.Hour,
		"day":   24 * time.Hour,
		"week":  7 * 24 * time.Hour,
		"month": 30 * 24 * time.Hour,
	}
	if duration, ok := durations[selected]; ok {
		return time.Now().UTC().Add(-duration)
	}
	return time.Time{}
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	w.Header().
		Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Warn("write response", "error", err)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().
				Set("Content-Security-Policy",
					"default-src 'self'; style-src 'self'; style-src-"+
						"attr 'unsafe-inline'; script-src 'self'; img-src "+
							"'self' data:; connect-src 'self'; object-src "+
								"'none'; base-uri 'none'; frame-ancestors 'none'")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().
				Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			next.ServeHTTP(w, r)
		},
	)
}

func requestLogger(next http.Handler) http.Handler {
	quiet := map[string]bool{
		"/api/v1/snapshot":        true,
		"/api/v1/commit-metadata": true,
		"/api/v1/history":         true,
		"/api/v1/gpu-history":     true,
		"/api/v1/npu-history":     true,
		"/api/v1/gpu-demo-status": true,
		"/api/v1/kernel-log":      true,
		"/api/v1/camera-stream":   true,
		"/api/v1/health":          true,
		"/api/v1/diagnostics":     true,
		"/api/v1/events":          true,
	}
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			if !quiet[r.URL.Path] {
				slog.Info(
					"request",
					"method",
					r.Method,
					"path",
					r.URL.Path,
					"duration_ms",
					strconv.FormatInt(
						time.Since(start).Milliseconds(),
						10,
					),
					"remote",
					remoteIP(r.RemoteAddr),
				)
			}
		},
	)
}

func remoteIP(addr string) string {
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return strings.Trim(addr[:i], "[]")
	}
	return addr
}

func proxyDemo(
	w http.ResponseWriter,
	r *http.Request,
	socketPath, target string,
) {
	if r.Method == http.MethodPost &&
		r.Header.Get("X-VAR-Studio-Action") != "gpu-demo" {
		http.Error(
			w,
			"action header required",
			http.StatusForbidden,
		)
		return
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(
				ctx,
				"unix",
				socketPath,
			)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   4 * time.Second,
	}
	request, err := http.NewRequestWithContext(
		r.Context(),
		r.Method,
		"http://unix"+target,
		io.LimitReader(r.Body, 2048),
	)
	if err != nil {
		http.Error(
			w,
			"invalid demo request",
			http.StatusBadRequest,
		)
		return
	}
	request.Header.Set(
		"X-VAR-Studio-Action",
		r.Header.Get("X-VAR-Studio-Action"),
	)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		writeJSON(
			w,
			http.StatusServiceUnavailable,
			map[string]string{
				"error": "GPU demo runner is unavailable",
			},
		)
		return
	}
	defer response.Body.Close()
	w.Header().
		Set("Content-Type", response.Header.Get("Content-Type"))
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(
		w,
		io.LimitReader(response.Body, 128<<10),
	)
}

func proxyCamera(
	w http.ResponseWriter,
	r *http.Request,
	socketPath, target string,
	stream bool,
) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(
				ctx,
				"unix",
				socketPath,
			)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   4 * time.Second,
	}
	if stream {
		client.Timeout = 0
	}
	request, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		"http://unix"+target,
		nil,
	)
	if err != nil {
		http.Error(
			w,
			"invalid camera request",
			http.StatusBadRequest,
		)
		return
	}
	response, err := client.Do(request)
	if err != nil {
		writeJSON(
			w,
			http.StatusServiceUnavailable,
			map[string]string{
				"error": "Camera runner is unavailable",
			},
		)
		return
	}
	defer response.Body.Close()
	w.Header().
		Set("Content-Type", response.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(response.StatusCode)
	if stream && response.StatusCode == http.StatusOK {
		_ = http.NewResponseController(w).
			SetWriteDeadline(time.Time{})
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = io.Copy(w, response.Body)
		return
	}
	_, _ = io.Copy(
		w,
		io.LimitReader(response.Body, 128<<10),
	)
}
