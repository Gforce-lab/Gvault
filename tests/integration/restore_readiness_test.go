//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gvault/gvault/internal/api"
	"github.com/gvault/gvault/internal/api/dto"
	"github.com/gvault/gvault/internal/api/handlers"
	"github.com/gvault/gvault/internal/config"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/kube"
	"github.com/gvault/gvault/internal/provider/nvidia"
	"github.com/gvault/gvault/internal/repository/postgres"
	"github.com/gvault/gvault/internal/service"
	"k8s.io/client-go/rest"
)

// This exercises the NVIDIA adapter with simulated Kubernetes observations and
// a local HTTP server. It is not evidence of physical GPU checkpoint/restore.
func simulatedKubernetes(t *testing.T, podIP string, creates *atomic.Int32) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	var pod map[string]any
	observations := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/namespaces/inference/pods":
			if pod != nil {
				http.Error(w, "duplicate restore Pod", http.StatusConflict)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&pod); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			creates.Add(1)
			metadata := pod["metadata"].(map[string]any)
			metadata["uid"] = "simulated-restored-worker-uid"
			if metadata["annotations"].(map[string]any)[nvidia.RestoreFromAnnotation] != "upstream" {
				t.Error("missing restore annotation in submitted Pod")
			}
			spec := pod["spec"].(map[string]any)
			if spec["restartPolicy"] != "Never" || len(spec["volumes"].([]any)) != 1 {
				t.Error("missing Snapshot control contract in submitted Pod")
			}
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/namespaces/inference/pods/restored" && pod != nil:
			observations++
			restored, ready, reason := "False", "False", "RestoreInProgress"
			if observations >= 2 {
				restored, reason = "True", "RestoreSucceeded"
			}
			if observations >= 3 {
				ready = "True"
			}
			pod["status"] = map[string]any{
				"phase": "Running", "podIP": podIP,
				"conditions": []any{
					map[string]any{"type": "nvidia.com/Restored", "status": restored, "reason": reason},
					map[string]any{"type": "Ready", "status": ready},
				},
			}
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(pod)
	}))
}

func TestRestoreInferenceOperationAndAudit(t *testing.T) {
	for _, protocol := range []string{"openai", "snapshot-vllm"} {
		t.Run(protocol, func(t *testing.T) {
			for _, succeeds := range []bool{true, false} {
				t.Run(strconv.FormatBool(succeeds), func(t *testing.T) {
					dsn := os.Getenv("DATABASE_URL")
					if dsn == "" {
						dsn = "postgres://vault:vault@localhost:5433/gvault?sslmode=disable"
					}
					db, err := postgres.Open(dsn)
					if err != nil {
						if os.Getenv("DATABASE_URL") != "" {
							t.Fatal(err)
						}
						t.Skipf("postgres unavailable: %v", err)
					}
					defer db.Close()
					ctx := context.Background()
					if err := postgres.MigrateUp(ctx, db, "../../db/migrations"); err != nil {
						t.Fatal(err)
					}
					entered, release := make(chan struct{}), make(chan struct{})
					var releaseOnce sync.Once
					unblock := func() { releaseOnce.Do(func() { close(release) }) }
					var calls atomic.Int32
					worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var payload map[string]any
						if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
							t.Error(err)
						}
						path := "/v1/completions"
						if protocol == "snapshot-vllm" {
							path = "/generate"
						}
						if r.Method != http.MethodPost || r.URL.Path != path || payload["prompt"] != "Say ready." {
							t.Errorf("unexpected inference request %s %s", r.Method, r.URL.Path)
						}
						if calls.Add(1) == 1 {
							close(entered)
						}
						select {
						case <-release:
						case <-r.Context().Done():
							return
						}
						if !succeeds {
							w.WriteHeader(http.StatusServiceUnavailable)
							_, _ = w.Write([]byte("private-failure-body"))
							return
						}
						if protocol == "snapshot-vllm" {
							_, _ = w.Write([]byte(`{"text":"private-generation-body"}`))
						} else {
							_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"text":"private-generation-body"}]}`))
						}
					}))
					defer worker.Close()
					defer unblock()
					ip, portText, _ := net.SplitHostPort(worker.Listener.Addr().String())
					port, _ := strconv.Atoi(portText)
					var creates atomic.Int32
					kubeServer := simulatedKubernetes(t, ip, &creates)
					defer kubeServer.Close()
					kc, err := kube.NewForConfig(&rest.Config{Host: kubeServer.URL})
					if err != nil {
						t.Fatal(err)
					}
					p := nvidia.New(true).WithKubeClient(kc)
					snaps, ops, audits := postgres.NewSnapshotRepo(db), postgres.NewOperationRepo(db), postgres.NewAuditRepo(db)
					id := domain.NewID("readiness")
					digest := "sha256:" + strings.Repeat("a", 64)
					snap := &domain.Snapshot{
						ID: id, Name: id, ClusterID: id, Namespace: "inference", Status: domain.SnapshotReady,
						Provider: "nvidia-snapshot", SourcePodName: "source", SourceContainerName: "main",
						ProviderSnapshotRef: domain.ProviderSnapshotRef{Kind: "PodSnapshot", Name: "upstream", Namespace: "inference", UID: "upstream-uid"},
						GPUProduct:          "test-gpu", GPUCount: 1, DriverVersion: "test-driver", Architecture: "amd64", ImageDigest: digest,
						Artifact: domain.ArtifactRef{Backend: "nvidia-snapshot"}, Labels: map[string]string{}, CreatedAt: time.Now().UTC(), ResourceVersion: 1,
					}
					if err := snaps.Create(ctx, nil, snap); err != nil {
						t.Fatal(err)
					}
					logger := slog.New(slog.NewTextHandler(io.Discard, nil))
					svc := service.NewSnapshotService(db, snaps, ops, audits, p, logger, time.Millisecond).WithRestoreTimeout(2 * time.Second)
					router := api.NewRouter(config.Config{DevAuthBypass: true}, logger, &handlers.API{DB: db, Svc: svc, Provider: p})
					apiServer := httptest.NewServer(router)
					defer apiServer.Close()
					body, _ := json.Marshal(map[string]any{
						"targetClusterId": id, "namespace": "inference", "podName": "restored",
						"inferenceProbe": map[string]any{"protocol": protocol, "model": "test-model", "port": port, "timeoutSeconds": 1},
						"gpuProduct":     "test-gpu", "driverVersion": "test-driver", "architecture": "amd64", "imageDigest": digest,
						"restorePodSpec": map[string]any{"containers": []any{map[string]any{"name": "main", "image": "example/vllm@" + digest, "resources": map[string]any{"limits": map[string]any{"nvidia.com/gpu": "1"}}, "env": []any{map[string]any{"name": "PRIVATE_TEST_ENV", "value": "private-environment-value"}}}}},
					})
					submit := func() dto.AcceptedBody {
						req, err := http.NewRequest(http.MethodPost, apiServer.URL+"/api/v1/snapshots/"+id+"/restore", bytes.NewReader(body))
						if err != nil {
							t.Fatal(err)
						}
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("Idempotency-Key", id)
						response, err := apiServer.Client().Do(req)
						if err != nil {
							t.Fatal(err)
						}
						defer response.Body.Close()
						if response.StatusCode != http.StatusAccepted {
							b, _ := io.ReadAll(response.Body)
							t.Fatalf("status=%d body=%s", response.StatusCode, b)
						}
						var accepted dto.AcceptedBody
						if err := json.NewDecoder(response.Body).Decode(&accepted); err != nil {
							t.Fatal(err)
						}
						return accepted
					}
					accepted := submit()
					select {
					case <-entered:
					case <-time.After(time.Second):
						t.Fatal("inference probe was not called")
					}
					op, err := ops.Get(ctx, accepted.OperationID)
					if err != nil || op.Status != domain.OperationRunning || op.FinishedAt != nil {
						t.Fatalf("operation must remain RUNNING during generation: %+v err=%v", op, err)
					}
					if replay := submit(); replay.OperationID != accepted.OperationID {
						t.Fatal("idempotent replay created another restore")
					}
					if creates.Load() != 1 {
						t.Fatalf("restore replay created %d Pods", creates.Load())
					}
					unblock()
					deadline := time.Now().Add(4 * time.Second)
					for {
						op, err = ops.Get(ctx, accepted.OperationID)
						if err != nil {
							t.Fatal(err)
						}
						if op.Status == domain.OperationSucceeded || op.Status == domain.OperationFailed {
							break
						}
						if time.Now().After(deadline) {
							t.Fatalf("operation did not finish: %+v", op)
						}
						time.Sleep(10 * time.Millisecond)
					}
					if (op.Status == domain.OperationSucceeded) != succeeds || op.FinishedAt == nil {
						t.Fatalf("operation=%+v", op)
					}
					if !succeeds && (op.ErrorCode == nil || *op.ErrorCode != string(domain.CodeProbeFailed)) {
						t.Fatalf("missing probe failure code: %+v", op)
					}
					response, err := apiServer.Client().Get(apiServer.URL + "/api/v1/operations/" + accepted.OperationID)
					if err != nil {
						t.Fatal(err)
					}
					var view dto.OperationView
					err = json.NewDecoder(response.Body).Decode(&view)
					response.Body.Close()
					if err != nil || response.StatusCode != http.StatusOK || view.Status != string(op.Status) || view.Provider != "nvidia-snapshot" || view.Simulated {
						t.Fatalf("operation response=%+v error=%v", view, err)
					}
					response, err = apiServer.Client().Get(apiServer.URL + "/api/v1/operations?limit=200")
					if err != nil {
						t.Fatal(err)
					}
					var listed struct {
						Items []dto.OperationView `json:"items"`
					}
					err = json.NewDecoder(response.Body).Decode(&listed)
					response.Body.Close()
					if err != nil || response.StatusCode != http.StatusOK {
						t.Fatalf("operations list failed: %v", err)
					}
					foundOperation := false
					for _, item := range listed.Items {
						if item.ID == op.ID {
							foundOperation = true
							if item.Provider != "nvidia-snapshot" || item.Simulated {
								t.Fatalf("wrong listed provenance: %+v", item)
							}
						}
					}
					if !foundOperation {
						t.Fatal("completed restore missing from operation list")
					}
					// A failed worker verification must not invalidate the reusable snapshot.
					stored, err := snaps.Get(ctx, id)
					if err != nil || stored.Status != domain.SnapshotReady {
						t.Fatalf("snapshot=%+v err=%v", stored, err)
					}
					events, err := audits.ListRecent(ctx, 100)
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, event := range events {
						if event.ResourceID != id {
							continue
						}
						allRaw, _ := json.Marshal(event.Attributes)
						if strings.Contains(string(allRaw), "private-") {
							t.Fatal("workload or generated content leaked into audit")
						}
						if event.Action != "snapshot.restore.probe" {
							continue
						}
						found = true
						if (event.Decision == "passed") != succeeds || event.Attributes["type"] != "inference" || event.Attributes["protocol"] != protocol {
							t.Fatalf("audit=%+v", event)
						}
						raw, _ := json.Marshal(event.Attributes)
						if strings.Contains(string(raw), "private-") || strings.Contains(string(raw), "Say ready.") {
							t.Fatalf("probe content leaked into audit: %s", raw)
						}
					}
					if !found {
						t.Fatal("missing inference audit event")
					}
				})
			}
		})
	}
}
