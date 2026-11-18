package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// InferenceEndpoint addresses the restored Pod itself, never a Service or a
// caller-supplied URL that could route the probe to a different worker.
func InferenceEndpoint(podIP string, port int) (string, error) {
	return InferenceEndpointForProtocol(podIP, port, "")
}

// InferenceEndpointForProtocol supports both an OpenAI server and the small
// /generate server shipped in NVIDIA Snapshot v0.3.0's vLLM workload guide.
func InferenceEndpointForProtocol(podIP string, port int, protocol string) (string, error) {
	if net.ParseIP(podIP) == nil {
		return "", fmt.Errorf("restored Pod has no valid IP address")
	}
	if port == 0 {
		port = 8000
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("inference port must be between 1 and 65535")
	}
	path := "/v1/completions"
	switch protocol {
	case "", "openai":
	case "snapshot-vllm":
		path = "/generate"
	default:
		return "", fmt.Errorf("unsupported inference protocol")
	}
	return "http://" + net.JoinHostPort(podIP, strconv.Itoa(port)) + path, nil
}

// InferenceProber verifies generation using OpenAI completions or the pinned
// Snapshot vLLM /generate API.
// It retries until the caller's deadline. Response bodies and generated text
// never appear in results, logs, or audit events.
type InferenceProber struct {
	Protocol      string
	Client        *http.Client
	RetryInterval time.Duration
}

func (p InferenceProber) Check(ctx context.Context, endpoint, model string) Result {
	protocol := p.Protocol
	if protocol == "" {
		protocol = "openai"
	}
	result := Result{Type: "inference", Endpoint: endpoint, Protocol: protocol}
	if _, ok := ctx.Deadline(); !ok {
		result.Message = "inference probe requires a deadline"
		return result
	}
	if p.Protocol != "" && p.Protocol != "openai" && p.Protocol != "snapshot-vllm" {
		result.Message = "unsupported inference protocol"
		return result
	}
	if p.Protocol != "snapshot-vllm" && strings.TrimSpace(model) == "" {
		result.Message = "inference model is required"
		return result
	}
	// The context bounds the whole attempt. A separate short HTTP timeout would
	// repeatedly cancel a valid but slow first generation before it can finish.
	client := http.Client{}
	if p.Client != nil {
		client = *p.Client
	} else {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil // a proxy could direct the request away from the Pod
		defer transport.CloseIdleConnections()
		client.Transport = transport
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	interval := p.RetryInterval
	if interval <= 0 {
		interval = time.Second
	}
	payload := map[string]any{
		"model": model, "prompt": "Say ready.", "max_tokens": 8,
		"temperature": 0, "stream": false,
	}
	if p.Protocol == "snapshot-vllm" {
		payload = map[string]any{"prompt": "Say ready."}
	}
	body, _ := json.Marshal(payload)
	last := "generation not attempted"
	for {
		if ctx.Err() != nil {
			result.Message = "inference probe stopped: " + ctx.Err().Error() + "; last result: " + last
			return result
		}
		ok, message := generation(ctx, &client, endpoint, model, p.Protocol, body)
		if ctx.Err() != nil {
			// Preserve the last completed attempt's diagnostic when cancellation
			// interrupts an HTTP request, and never pass after the deadline.
			continue
		}
		if ok {
			result.OK = true
			result.Message = "restored worker generated a non-empty completion for the requested model"
			if p.Protocol == "snapshot-vllm" {
				result.Message = "restored worker generated non-empty text via the Snapshot vLLM API (model identity not reported by this protocol)"
			}
			return result
		}
		last = message
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func generation(ctx context.Context, client *http.Client, endpoint, model, protocol string, body []byte) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false, "invalid inference endpoint"
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return false, "inference request failed"
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return false, fmt.Sprintf("inference returned HTTP %d", res.StatusCode)
	}
	const maxResponseBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return false, "inference response unreadable or too large"
	}
	var completion struct {
		Text    string          `json:"text"`
		Error   json.RawMessage `json:"error"`
		Model   string          `json:"model"`
		Choices []struct {
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &completion); err != nil {
		return false, "inference response is not a completion"
	}
	if len(completion.Error) > 0 && string(completion.Error) != "null" {
		return false, "inference response contains an error"
	}
	if protocol == "snapshot-vllm" {
		if strings.TrimSpace(completion.Text) != "" {
			return true, ""
		}
		return false, "inference response contains no generated text"
	}
	if completion.Model != model {
		return false, "inference response model does not match the requested model"
	}
	for _, choice := range completion.Choices {
		if strings.TrimSpace(choice.Text) != "" {
			return true, ""
		}
	}
	return false, "inference response contains no generated text"
}
