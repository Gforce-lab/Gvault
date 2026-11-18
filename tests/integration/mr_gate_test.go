//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gvault/gvault/internal/api"
	"github.com/gvault/gvault/internal/api/handlers"
	"github.com/gvault/gvault/internal/authz"
	"github.com/gvault/gvault/internal/config"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider/fake"
	"github.com/gvault/gvault/internal/provider/nvidia"
	"github.com/gvault/gvault/internal/repository/postgres"
	"github.com/gvault/gvault/internal/service"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://vault:vault@localhost:5433/gvault?sslmode=disable"
	}
	db, err := postgres.Open(dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	if err := postgres.MigrateUp(context.Background(), db, "../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMRGateOverviewAuditsAndNamespacePolicy(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	fp := fake.New(20 * time.Millisecond)
	cfg := config.Config{
		DevAuthBypass: true, Provider: "fake", AllowTestProvider: true,
		FakeProviderStep: 20 * time.Millisecond, PollInterval: 15 * time.Millisecond,
		AllowedNamespaces: "inference",
	}
	svc := service.NewSnapshotService(
		db, postgres.NewSnapshotRepo(db), postgres.NewOperationRepo(db), postgres.NewAuditRepo(db),
		fp, slog.Default(), cfg.PollInterval,
	).WithNamespacePolicy(authz.NewNamespacePolicy(cfg.AllowedNamespaces))
	h := &handlers.API{DB: db, Svc: svc, Provider: fp}
	r := api.NewRouter(cfg, slog.Default(), h)

	// Forbidden namespace
	bad := []byte(`{"name":"x","clusterId":"c","namespace":"denied","source":{"podName":"p","containerName":"c"},"provider":"fake"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots", bytes.NewReader(bad))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for denied ns, got %d %s", w.Code, w.Body.String())
	}

	runID := domain.NewID("mrgate")
	okBody, _ := json.Marshal(map[string]any{
		"name": runID, "clusterId": "c1", "namespace": "inference", "provider": "fake",
		"source": map[string]string{"podName": "p1", "containerName": "main"},
	})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots", bytes.NewReader(okBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", runID)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusAccepted {
		t.Fatalf("capture=%d %s", w2.Code, w2.Body.String())
	}
	var accepted map[string]string
	_ = json.Unmarshal(w2.Body.Bytes(), &accepted)

	deadline := time.Now().Add(5 * time.Second)
	for {
		op, err := svc.GetOperation(context.Background(), accepted["operationId"])
		if err != nil {
			t.Fatal(err)
		}
		if op.Status == domain.OperationSucceeded {
			break
		}
		if op.Status == domain.OperationFailed || time.Now().After(deadline) {
			t.Fatalf("capture failed: %+v", op)
		}
		time.Sleep(20 * time.Millisecond)
	}

	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/api/v1/audits?limit=10", nil))
	if w3.Code != 200 {
		t.Fatalf("audits=%d %s", w3.Code, w3.Body.String())
	}
	var audits map[string]any
	_ = json.Unmarshal(w3.Body.Bytes(), &audits)
	items, _ := audits["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected audit events after capture")
	}

	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
	if w4.Code != 200 {
		t.Fatalf("overview=%d", w4.Code)
	}
}

func TestMRGateNVIDIADryRunAndExportFailClosed(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	np := nvidia.New(true)
	cfg := config.Config{DevAuthBypass: true, Provider: "nvidia-snapshot", EnableNVIDIAProvider: true}
	svc := service.NewSnapshotService(
		db, postgres.NewSnapshotRepo(db), postgres.NewOperationRepo(db), postgres.NewAuditRepo(db),
		np, slog.Default(), 20*time.Millisecond,
	)
	h := &handlers.API{DB: db, Svc: svc, Provider: np, EnableS3Export: false}
	r := api.NewRouter(cfg, slog.Default(), h)

	body := []byte(`{"namespace":"inference","name":"dry","podName":"p","containerName":"main"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/providers/nvidia/dry-run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("dry-run=%d %s", w.Code, w.Body.String())
	}

	exp := []byte(`{"snapshotId":"snap_x","bucket":"b","key":"k"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/export/s3", bytes.NewReader(exp))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code < 400 {
		t.Fatalf("export should fail-closed, got %d", w2.Code)
	}
}
