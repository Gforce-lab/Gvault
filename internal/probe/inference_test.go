package probe_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gvault/gvault/internal/probe"
)

func TestInferenceEndpoint(t *testing.T) {
	for _, tc := range []struct {
		ip   string
		port int
		want string
	}{
		{"10.0.0.2", 0, "http://10.0.0.2:8000/v1/completions"},
		{"2001:db8::2", 9000, "http://[2001:db8::2]:9000/v1/completions"},
		{"worker-service", 8000, ""},
		{"", 8000, ""},
		{"10.0.0.2", -1, ""},
		{"10.0.0.2", 65536, ""},
	} {
		t.Run(tc.ip+tc.want, func(t *testing.T) {
			got, err := probe.InferenceEndpoint(tc.ip, tc.port)
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("endpoint=%q err=%v", got, err)
			}
		})
	}
}

func TestInferenceRetriesWarmupAndValidatesRequest(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/completions" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Model       string  `json:"model"`
			Prompt      string  `json:"prompt"`
			MaxTokens   int     `json:"max_tokens"`
			Temperature float64 `json:"temperature"`
			Stream      bool    `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "test-model" || body.Prompt != "Say ready." || body.MaxTokens != 8 || body.Stream || body.Temperature != 0 {
			t.Errorf("unexpected probe payload: %+v", body)
		}
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"text":"ready"}]}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := (probe.InferenceProber{RetryInterval: time.Millisecond}).Check(ctx, srv.URL+"/v1/completions", "test-model")
	if !result.OK || result.Simulated || result.Type != "inference" || attempts.Load() != 3 {
		t.Fatalf("result=%+v attempts=%d", result, attempts.Load())
	}
}

func TestInferenceRejectsInvalidResponsesWithoutLeakingBodies(t *testing.T) {
	wantDiagnostic := map[string]string{
		"health response": "model does not match", "empty completion": "no generated text",
		"missing model": "model does not match", "wrong model": "model does not match",
		"invalid json": "not a completion", "trailing json": "not a completion", "oversized": "too large",
		"error with completion": "contains an error",
	}
	for name, body := range map[string]string{
		"health response":       `{"status":"ok","secret":"private-marker"}`,
		"empty completion":      `{"model":"test-model","choices":[{"text":"  "}]}`,
		"missing model":         `{"choices":[{"text":"private-marker"}]}`,
		"wrong model":           `{"model":"wrong-model","choices":[{"text":"private-marker"}]}`,
		"invalid json":          `private-marker`,
		"trailing json":         `{"model":"test-model","choices":[{"text":"ready"}]}{}`,
		"oversized":             strings.Repeat("private-marker", 100000),
		"error with completion": `{"model":"test-model","choices":[{"text":"ready"}],"error":{"message":"private-marker"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			result := (probe.InferenceProber{RetryInterval: time.Millisecond}).Check(ctx, srv.URL, "test-model")
			if result.OK || !strings.Contains(result.Message, "deadline exceeded") || !strings.Contains(result.Message, wantDiagnostic[name]) || strings.Contains(result.Message, "private-marker") {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestSnapshotVLLMProtocol(t *testing.T) {
	for _, succeeds := range []bool{true, false} {
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if r.URL.Path != "/generate" || len(body) != 1 || body["prompt"] != "Say ready." {
				t.Errorf("unexpected Snapshot protocol request %s %v", r.URL.Path, body)
			}
			if attempts.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if succeeds {
				_, _ = w.Write([]byte(`{"text":"ready"}`))
			} else {
				_, _ = w.Write([]byte(`{"text":" "}`))
			}
		}))
		endpoint, err := probe.InferenceEndpointForProtocol("127.0.0.1", 8000, "snapshot-vllm")
		if err != nil || endpoint != "http://127.0.0.1:8000/generate" {
			t.Fatalf("endpoint=%q err=%v", endpoint, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		result := (probe.InferenceProber{Protocol: "snapshot-vllm", RetryInterval: time.Millisecond}).Check(ctx, srv.URL+"/generate", "")
		cancel()
		srv.Close()
		if result.OK != succeeds || attempts.Load() < 2 || result.Protocol != "snapshot-vllm" || strings.Contains(result.Message, "requested model") {
			t.Fatalf("result=%+v attempts=%d", result, attempts.Load())
		}
	}
}

func TestInferenceDoesNotFollowRedirects(t *testing.T) {
	var followed atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		followed.Store(true)
		_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"text":"ready"}]}`))
	}))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := (probe.InferenceProber{Client: srv.Client(), RetryInterval: time.Millisecond}).Check(ctx, srv.URL, "test-model")
	if result.OK || followed.Load() {
		t.Fatalf("result=%+v followed=%v", result, followed.Load())
	}
}

func TestInferenceCancelsHangingRequest(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := (probe.InferenceProber{}).Check(ctx, srv.URL, "test-model")
	if result.OK || !strings.Contains(result.Message, "deadline exceeded") {
		t.Fatalf("result=%+v", result)
	}
}

func TestInferenceRequiresDeadline(t *testing.T) {
	result := (probe.InferenceProber{}).Check(context.Background(), "http://127.0.0.1:8000", "model")
	if result.OK || result.Message != "inference probe requires a deadline" {
		t.Fatalf("result=%+v", result)
	}
}
