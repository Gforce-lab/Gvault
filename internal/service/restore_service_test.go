package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider"
)

type restoreObserver struct {
	provider.CheckpointProvider
	observe func(context.Context) (provider.RestoreState, error)
}

func (p restoreObserver) Name() string { return "nvidia-snapshot" }
func (p restoreObserver) RestoreStatus(ctx context.Context, _ provider.ProviderRef) (provider.RestoreState, error) {
	return p.observe(ctx)
}

func testRestoreService(p provider.CheckpointProvider) *SnapshotService {
	return NewSnapshotService(nil, nil, nil, nil, p, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Millisecond)
}

func assertErrorCode(t *testing.T, err error, code domain.ErrorCode) {
	t.Helper()
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error=%v want code=%s", err, code)
	}
}

func TestRestoreWaitsForReadinessBeforeInference(t *testing.T) {
	var polls atomic.Int32
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if polls.Load() < 3 {
			t.Error("inference attempted before provider became ready")
		}
		calls.Add(1)
		_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"text":"ready"}]}`))
	}))
	defer srv.Close()
	ip, portText, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	p := restoreObserver{observe: func(context.Context) (provider.RestoreState, error) {
		phase := domain.OperationRunning
		if polls.Add(1) >= 3 {
			phase = domain.OperationSucceeded
		}
		return provider.RestoreState{Phase: phase, PodIP: ip}, nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := testRestoreService(p).awaitRestore(ctx, provider.ProviderRef{}, domain.RestoreRequest{
		InferenceProbe: &domain.InferenceProbe{Model: "test-model", Port: port},
	})
	if err != nil || result == nil || !result.OK || result.Type != "inference" || calls.Load() != 1 || polls.Load() != 4 {
		t.Fatalf("result=%+v err=%v calls=%d polls=%d", result, err, calls.Load(), polls.Load())
	}
}

func TestRestoreProviderFailureDoesNotProbe(t *testing.T) {
	p := restoreObserver{observe: func(context.Context) (provider.RestoreState, error) {
		return provider.RestoreState{Phase: domain.OperationFailed, Message: "restore failed"}, nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := testRestoreService(p).awaitRestore(ctx, provider.ProviderRef{}, domain.RestoreRequest{
		InferenceProbe: &domain.InferenceProbe{Model: "test-model"},
	})
	if result != nil {
		t.Fatalf("probe attempted after failed restore: %+v", result)
	}
	assertErrorCode(t, err, domain.CodeProvider)
}

func TestRestoreProviderDeadline(t *testing.T) {
	for _, blocks := range []bool{false, true} {
		t.Run(strconv.FormatBool(blocks), func(t *testing.T) {
			p := restoreObserver{observe: func(ctx context.Context) (provider.RestoreState, error) {
				if blocks {
					<-ctx.Done()
					return provider.RestoreState{}, ctx.Err()
				}
				return provider.RestoreState{Phase: domain.OperationRunning}, nil
			}}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			result, err := testRestoreService(p).awaitRestore(ctx, provider.ProviderRef{}, domain.RestoreRequest{})
			if result != nil {
				t.Fatalf("unexpected result: %+v", result)
			}
			assertErrorCode(t, err, domain.CodeRestoreTimeout)
		})
	}
}

func TestRestoreInferenceRequiresPodIP(t *testing.T) {
	p := restoreObserver{observe: func(context.Context) (provider.RestoreState, error) {
		return provider.RestoreState{Phase: domain.OperationSucceeded}, nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := testRestoreService(p).awaitRestore(ctx, provider.ProviderRef{}, domain.RestoreRequest{
		InferenceProbe: &domain.InferenceProbe{Model: "test-model"},
	})
	if result == nil || result.OK {
		t.Fatalf("result=%+v", result)
	}
	assertErrorCode(t, err, domain.CodeProbeFailed)
}

func TestRestoreHealthFailureTimesOutAndRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	p := restoreObserver{observe: func(context.Context) (provider.RestoreState, error) {
		return provider.RestoreState{Phase: domain.OperationSucceeded}, nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result, err := testRestoreService(p).awaitRestore(ctx, provider.ProviderRef{}, domain.RestoreRequest{ProbeEndpoint: srv.URL})
	if result == nil || result.OK || calls.Load() < 2 {
		t.Fatalf("result=%+v calls=%d", result, calls.Load())
	}
	assertErrorCode(t, err, domain.CodeProbeFailed)
}

func TestRestoreRechecksReadinessAfterProbe(t *testing.T) {
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	p := restoreObserver{observe: func(context.Context) (provider.RestoreState, error) {
		phase := domain.OperationSucceeded
		if polls.Add(1) > 1 {
			phase = domain.OperationFailed
		}
		return provider.RestoreState{Phase: phase}, nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := testRestoreService(p).awaitRestore(ctx, provider.ProviderRef{}, domain.RestoreRequest{ProbeEndpoint: srv.URL})
	if result == nil || result.OK {
		t.Fatalf("result=%+v", result)
	}
	assertErrorCode(t, err, domain.CodeProbeFailed)
}

func TestRestoreProbeValidationBeforePersistence(t *testing.T) {
	for name, spec := range map[string]domain.InferenceProbe{
		"unknown protocol": {Protocol: "arbitrary", Model: "m"},
		"missing model":    {}, "blank model": {Model: "  "},
		"negative port": {Model: "m", Port: -1}, "invalid port": {Model: "m", Port: 65536},
		"negative timeout": {Model: "m", TimeoutSeconds: -1}, "unbounded timeout": {Model: "m", TimeoutSeconds: 301},
	} {
		t.Run(name, func(t *testing.T) {
			svc := testRestoreService(restoreObserver{})
			_, err := svc.Restore(context.Background(), domain.RestoreRequest{
				SnapshotID: "s", ClusterID: "c", Namespace: "ns", PodName: "worker", InferenceProbe: &spec,
			}, domain.TargetInfo{})
			assertErrorCode(t, err, domain.CodeValidation)
		})
	}
	if err := validateRestoreProbe(domain.RestoreRequest{ProbeEndpoint: "http://health", InferenceProbe: &domain.InferenceProbe{Model: "m"}}); err == nil {
		t.Fatal("expected conflicting probe modes to fail validation")
	}
}

func TestSnapshotVLLMProbeAllowsOmittedModel(t *testing.T) {
	if err := validateRestoreProbe(domain.RestoreRequest{InferenceProbe: &domain.InferenceProbe{Protocol: "snapshot-vllm"}}); err != nil {
		t.Fatal(err)
	}
}
