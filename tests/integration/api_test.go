//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"log/slog"

	"github.com/gvault/gvault/internal/api"
	"github.com/gvault/gvault/internal/api/handlers"
	"github.com/gvault/gvault/internal/config"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider/fake"
	"github.com/gvault/gvault/internal/repository/postgres"
	"github.com/gvault/gvault/internal/service"
)

func TestCaptureRestoreIdempotency(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://vault:vault@localhost:5433/gvault?sslmode=disable"
	}
	db, err := postgres.Open(dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := postgres.MigrateUp(ctx, db, "../../db/migrations"); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		HTTPAddr: ":0", DevAuthBypass: true, Provider: "fake", AllowTestProvider: true,
		FakeProviderStep: 30 * time.Millisecond, PollInterval: 20 * time.Millisecond,
	}
	fp := fake.New(cfg.FakeProviderStep)
	svc := service.NewSnapshotService(db, postgres.NewSnapshotRepo(db), postgres.NewOperationRepo(db), postgres.NewAuditRepo(db), fp, slog.Default(), cfg.PollInterval)
	h := &handlers.API{DB: db, Svc: svc, Provider: fp}
	r := api.NewRouter(cfg, slog.Default(), h)

	runID := domain.NewID("itest")
	body, _ := json.Marshal(map[string]any{"name": runID, "clusterId": runID, "namespace": "ns1", "source": map[string]string{"podName": "p1", "containerName": "main"}, "provider": "fake"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", runID+"-capture")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("capture status=%d body=%s", w.Code, w.Body.String())
	}
	var accepted map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &accepted)
	snapID := accepted["snapshotId"]
	opID := accepted["operationId"]

	// Idempotent replay
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", runID+"-capture")
	r.ServeHTTP(w2, req2)
	var accepted2 map[string]string
	_ = json.Unmarshal(w2.Body.Bytes(), &accepted2)
	if accepted2["operationId"] != opID {
		t.Fatalf("idempotency broken: %v vs %v", accepted2, accepted)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/api/v1/operations/"+opID, nil))
		var op map[string]any
		_ = json.Unmarshal(w3.Body.Bytes(), &op)
		if op["provider"] != "fake" || op["simulated"] != true {
			t.Fatalf("fake operation must retain its provenance: %v", op)
		}
		if op["status"] == "SUCCEEDED" {
			break
		}
		if op["status"] == "FAILED" {
			t.Fatalf("capture failed: %v", op)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for capture: %v", op)
		}
		time.Sleep(30 * time.Millisecond)
	}

	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/"+snapID, nil))
	if w4.Code != 200 {
		t.Fatal(w4.Body.String())
	}

	restBody, _ := json.Marshal(map[string]string{"targetClusterId": runID, "namespace": "ns1", "podName": "restored-1"})
	wr := httptest.NewRecorder()
	rreq := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots/"+snapID+"/restore", bytes.NewReader(restBody))
	rreq.Header.Set("Content-Type", "application/json")
	rreq.Header.Set("Idempotency-Key", runID+"-restore")
	r.ServeHTTP(wr, rreq)
	if wr.Code != http.StatusAccepted {
		t.Fatalf("restore status=%d body=%s", wr.Code, wr.Body.String())
	}
	var restoreAccepted map[string]string
	if err := json.Unmarshal(wr.Body.Bytes(), &restoreAccepted); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		op, err := svc.GetOperation(ctx, restoreAccepted["operationId"])
		if err != nil {
			t.Fatal(err)
		}
		if op.Status == domain.OperationSucceeded {
			break
		}
		if op.Status == domain.OperationFailed || time.Now().After(deadline) {
			t.Fatalf("restore did not succeed: %+v", op)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBadInput(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://vault:vault@localhost:5433/gvault?sslmode=disable"
	}
	db, err := postgres.Open(dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer db.Close()
	_ = postgres.MigrateUp(context.Background(), db, "../../db/migrations")
	cfg := config.Config{DevAuthBypass: true, Provider: "fake"}
	fp := fake.New(time.Millisecond)
	svc := service.NewSnapshotService(db, postgres.NewSnapshotRepo(db), postgres.NewOperationRepo(db), postgres.NewAuditRepo(db), fp, slog.Default(), time.Millisecond)
	h := &handlers.API{DB: db, Svc: svc, Provider: fp}
	r := api.NewRouter(cfg, slog.Default(), h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/snapshots", bytes.NewReader([]byte(`{}`))))
	if w.Code == http.StatusAccepted {
		t.Fatal("expected validation failure")
	}
}
