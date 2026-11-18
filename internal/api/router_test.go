package api_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gvault/gvault/internal/api"
	"github.com/gvault/gvault/internal/api/handlers"
	"github.com/gvault/gvault/internal/config"
	"github.com/gvault/gvault/internal/controller"
	"github.com/gvault/gvault/internal/provider/nvidia"
	"github.com/gvault/gvault/internal/telemetry"
)

func TestRouterPublicAndNVIDIADryRun(t *testing.T) {
	cfg := config.Config{
		DevAuthBypass: true, Provider: "nvidia-snapshot",
		EnableNVIDIAProvider: true, UIDir: "",
	}
	np := nvidia.New(true)
	h := &handlers.API{
		Provider: np, Metrics: telemetry.New(),
		CRDStore: controller.NewStore(),
	}
	r := api.NewRouter(cfg, slog.Default(), h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/version", nil))
	if w.Code != 200 {
		t.Fatalf("version=%d %s", w.Code, w.Body.String())
	}

	body := []byte(`{"namespace":"inference","name":"s1","podName":"p1","containerName":"main"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/providers/nvidia/dry-run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != 200 {
		t.Fatalf("dry-run=%d %s", w2.Code, w2.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["pinnedRelease"] != nvidia.PinnedRelease {
		t.Fatalf("%v", out)
	}
}

func TestRouterAuthRequiredWithoutBypass(t *testing.T) {
	cfg := config.Config{
		DevAuthBypass: false, APIKey: "k", Provider: "nvidia-snapshot", EnableNVIDIAProvider: true, UIDir: "",
	}
	p, _ := nvidia.NewWithMemoryCluster()
	h := &handlers.API{Provider: p, Metrics: telemetry.New(), CRDStore: controller.NewStore()}
	r := api.NewRouter(cfg, slog.Default(), h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil))
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	req.Header.Set("X-API-Key", "k")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != 200 {
		t.Fatalf("expected 200, got %d %s", w2.Code, w2.Body.String())
	}
}

func TestRouterExportS3FailClosed(t *testing.T) {
	cfg := config.Config{DevAuthBypass: true, Provider: "nvidia-snapshot", EnableNVIDIAProvider: true, UIDir: ""}
	h := &handlers.API{Provider: nvidia.New(true), EnableS3Export: false}
	r := api.NewRouter(cfg, slog.Default(), h)
	body := []byte(`{"snapshotId":"snap_1","bucket":"b","key":"k"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export/s3", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code < 400 {
		t.Fatalf("expected failure, got %d %s", w.Code, w.Body.String())
	}
}
