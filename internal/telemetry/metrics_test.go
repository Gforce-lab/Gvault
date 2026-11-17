package telemetry_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gvault/gvault/internal/telemetry"
)

func TestMetricsExposition(t *testing.T) {
	m := telemetry.New()
	m.IncOperation("fake", "capture_succeeded")
	m.IncCompatReject("GPU_MODEL_MISMATCH")
	m.SetReady(2)
	m.ObserveCaptureSeconds(0.5)

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"gvault_snapshot_operations_total",
		"gvault_compatibility_rejections_total",
		"gvault_snapshots_ready 2",
		"gvault_capture_duration_seconds_count",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}
