package probe

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Result is a post-restore application readiness check.
type Result struct {
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
	Endpoint string `json:"endpoint,omitempty"`
	Simulated bool  `json:"simulated"`
}

// Prober verifies a restored workload is request-ready.
type Prober interface {
	Check(ctx context.Context, endpoint string) Result
}

// FakeProber always succeeds after a tiny delay — used with FakeProvider (simulated).
type FakeProber struct{}

func (FakeProber) Check(ctx context.Context, endpoint string) Result {
	select {
	case <-ctx.Done():
		return Result{OK: false, Message: ctx.Err().Error(), Simulated: true}
	case <-time.After(5 * time.Millisecond):
		return Result{OK: true, Message: "fake probe ok — simulated inference readiness", Endpoint: endpoint, Simulated: true}
	}
}

// HTTPProber GETs an endpoint and expects 2xx.
type HTTPProber struct {
	Client *http.Client
}

func (p HTTPProber) Check(ctx context.Context, endpoint string) Result {
	if endpoint == "" {
		return Result{OK: false, Message: "probe endpoint required"}
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{OK: false, Message: err.Error(), Endpoint: endpoint}
	}
	res, err := client.Do(req)
	if err != nil {
		return Result{OK: false, Message: err.Error(), Endpoint: endpoint}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Result{OK: false, Message: fmt.Sprintf("unexpected status %d", res.StatusCode), Endpoint: endpoint}
	}
	return Result{OK: true, Message: "inference/health probe succeeded", Endpoint: endpoint}
}
