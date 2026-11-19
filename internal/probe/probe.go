package probe

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Result is a post-restore application readiness check.
type Result struct {
	Protocol  string `json:"protocol,omitempty"`
	Type      string `json:"type"`
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	Endpoint  string `json:"endpoint,omitempty"`
	Simulated bool   `json:"simulated"`
}

// Prober verifies a restored workload is request-ready.
type Prober interface {
	Check(ctx context.Context, endpoint string) Result
}

// HTTPProber GETs an endpoint and expects 2xx.
type HTTPProber struct {
	Client *http.Client
}

func (p HTTPProber) Check(ctx context.Context, endpoint string) Result {
	if endpoint == "" {
		return Result{Type: "http", OK: false, Message: "probe endpoint required"}
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{Type: "http", OK: false, Message: "invalid health endpoint", Endpoint: endpoint}
	}
	res, err := client.Do(req)
	if err != nil {
		return Result{Type: "http", OK: false, Message: "health request failed", Endpoint: endpoint}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Result{Type: "http", OK: false, Message: fmt.Sprintf("unexpected status %d", res.StatusCode), Endpoint: endpoint}
	}
	return Result{Type: "http", OK: true, Message: "HTTP health probe succeeded (inference not checked)", Endpoint: endpoint}
}
