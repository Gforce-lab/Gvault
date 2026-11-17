package telemetry

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// Metrics is a low-cardinality in-process registry for beta Prometheus exposition.
type Metrics struct {
	opsTotal          sync.Map // key provider|outcome -> *atomic.Uint64
	compatRejects     sync.Map // key reason -> *atomic.Uint64
	snapshotsReady    atomic.Int64
	captureSecondsSum atomic.Uint64
	captureCount      atomic.Uint64
	restoreSecondsSum atomic.Uint64
	restoreCount      atomic.Uint64
}

func New() *Metrics { return &Metrics{} }

func (m *Metrics) IncOperation(provider, outcome string) {
	incMap(&m.opsTotal, provider+"|"+outcome)
}

func (m *Metrics) IncCompatReject(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	incMap(&m.compatRejects, reason)
}

func (m *Metrics) SetReady(n int64) { m.snapshotsReady.Store(n) }

func (m *Metrics) ObserveCaptureSeconds(sec float64) {
	m.captureSecondsSum.Add(uint64(sec * 1000))
	m.captureCount.Add(1)
}

func (m *Metrics) ObserveRestoreSeconds(sec float64) {
	m.restoreSecondsSum.Add(uint64(sec * 1000))
	m.restoreCount.Add(1)
}

func incMap(m *sync.Map, key string) {
	v, _ := m.LoadOrStore(key, &atomic.Uint64{})
	v.(*atomic.Uint64).Add(1)
}

// Handler exposes Prometheus text format at /metrics.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		var b strings.Builder
		b.WriteString("# HELP gvault_snapshot_operations_total Snapshot operations by provider and outcome\n")
		b.WriteString("# TYPE gvault_snapshot_operations_total counter\n")
		m.opsTotal.Range(func(k, v any) bool {
			parts := strings.SplitN(k.(string), "|", 2)
			provider, outcome := "unknown", "unknown"
			if len(parts) == 2 {
				provider, outcome = parts[0], parts[1]
			}
			fmt.Fprintf(&b, "gvault_snapshot_operations_total{provider=%q,outcome=%q} %d\n", provider, outcome, v.(*atomic.Uint64).Load())
			return true
		})
		b.WriteString("# HELP gvault_compatibility_rejections_total Compatibility preflight rejections\n")
		b.WriteString("# TYPE gvault_compatibility_rejections_total counter\n")
		m.compatRejects.Range(func(k, v any) bool {
			fmt.Fprintf(&b, "gvault_compatibility_rejections_total{reason=%q} %d\n", k.(string), v.(*atomic.Uint64).Load())
			return true
		})
		b.WriteString("# HELP gvault_snapshots_ready Ready snapshots gauge\n")
		b.WriteString("# TYPE gvault_snapshots_ready gauge\n")
		fmt.Fprintf(&b, "gvault_snapshots_ready %d\n", m.snapshotsReady.Load())
		if c := m.captureCount.Load(); c > 0 {
			b.WriteString("# HELP gvault_capture_duration_seconds Capture duration (sum/count for beta)\n")
			b.WriteString("# TYPE gvault_capture_duration_seconds summary\n")
			fmt.Fprintf(&b, "gvault_capture_duration_seconds_sum %.3f\n", float64(m.captureSecondsSum.Load())/1000)
			fmt.Fprintf(&b, "gvault_capture_duration_seconds_count %d\n", c)
		}
		if c := m.restoreCount.Load(); c > 0 {
			b.WriteString("# HELP gvault_restore_duration_seconds Restore duration (sum/count for beta)\n")
			b.WriteString("# TYPE gvault_restore_duration_seconds summary\n")
			fmt.Fprintf(&b, "gvault_restore_duration_seconds_sum %.3f\n", float64(m.restoreSecondsSum.Load())/1000)
			fmt.Fprintf(&b, "gvault_restore_duration_seconds_count %d\n", c)
		}
		_, _ = w.Write([]byte(b.String()))
	}
}
