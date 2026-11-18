package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const cliVersion = "0.1.0-beta.1"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	base := strings.TrimRight(env("GVAULT_API", "http://localhost:8080"), "/")
	switch os.Args[1] {
	case "help", "-h", "--help":
		usage()
	case "version":
		if flagBool("--local") {
			fmt.Printf("gvaultctl %s\n", cliVersion)
			return
		}
		printJSON(mustDo(http.MethodGet, base+"/api/v1/version", nil, ""))
	case "health":
		printJSON(mustDo(http.MethodGet, base+"/healthz", nil, ""))
	case "ready":
		printJSON(mustDo(http.MethodGet, base+"/readyz", nil, ""))
	case "overview":
		printJSON(mustDo(http.MethodGet, base+"/api/v1/overview", nil, ""))
	case "providers":
		if len(os.Args) >= 3 && os.Args[2] == "nvidia-dry-run" {
			need(7)
			body := map[string]any{
				"namespace": os.Args[3], "name": os.Args[4],
				"podName": os.Args[5], "containerName": os.Args[6],
			}
			if len(os.Args) >= 8 {
				body["podUid"] = os.Args[7]
			}
			printJSON(mustDo(http.MethodPost, base+"/api/v1/providers/nvidia/dry-run", body, ""))
			return
		}
		printJSON(mustDo(http.MethodGet, base+"/api/v1/providers", nil, ""))
	case "audits":
		limit := "50"
		if len(os.Args) >= 3 {
			limit = os.Args[2]
		}
		printJSON(mustDo(http.MethodGet, base+"/api/v1/audits?limit="+limit, nil, ""))
	case "export":
		need(3)
		if os.Args[2] != "s3" {
			usage()
			os.Exit(2)
		}
		need(6)
		body := map[string]any{
			"snapshotId": os.Args[3], "bucket": os.Args[4], "key": os.Args[5],
		}
		printJSON(mustDo(http.MethodPost, base+"/api/v1/export/s3", body, ""))
	case "wait":
		need(3)
		waitOperation(base, os.Args[2])
	case "snapshot":
		cmdSnapshot(base)
	case "operation":
		cmdOperation(base)
	case "crd":
		cmdCRD(base)
	default:
		usage()
		os.Exit(2)
	}
}

func cmdSnapshot(base string) {
	if len(os.Args) < 3 {
		usage()
		os.Exit(2)
	}
	switch os.Args[2] {
	case "list":
		q := ""
		if ns := env("GVAULT_NAMESPACE", ""); ns != "" {
			q = "?namespace=" + ns
		}
		printJSON(mustDo(http.MethodGet, base+"/api/v1/snapshots"+q, nil, ""))
	case "get":
		need(4)
		printJSON(mustDo(http.MethodGet, base+"/api/v1/snapshots/"+os.Args[3], nil, ""))
	case "create":
		need(8)
		provider := env("GVAULT_PROVIDER", "fake")
		body := map[string]any{
			"name": os.Args[3], "clusterId": os.Args[4], "namespace": os.Args[5],
			"provider": provider,
			"source":   map[string]string{"podName": os.Args[6], "containerName": os.Args[7]},
		}
		if d := env("GVAULT_DIGEST", ""); d != "" {
			body["imageDigest"] = d
		}
		if g := env("GVAULT_GPU", ""); g != "" {
			body["gpuProduct"] = g
		}
		if drv := env("GVAULT_DRIVER", ""); drv != "" {
			body["driverVersion"] = drv
		}
		if arch := env("GVAULT_ARCH", ""); arch != "" {
			body["architecture"] = arch
		}
		idem := env("GVAULT_IDEMPOTENCY_KEY", "gvaultctl-capture-"+fmt.Sprint(time.Now().UnixNano()))
		printJSON(mustDo(http.MethodPost, base+"/api/v1/snapshots", body, idem))
	case "delete":
		need(4)
		idem := env("GVAULT_IDEMPOTENCY_KEY", "gvaultctl-delete-"+fmt.Sprint(time.Now().UnixNano()))
		printJSON(mustDo(http.MethodDelete, base+"/api/v1/snapshots/"+os.Args[3], nil, idem))
	case "check":
		need(5)
		body := map[string]any{
			"targetClusterId": os.Args[4],
			"gpuProduct":      env("GVAULT_GPU", "NVIDIA-Fake-A100-80GB"),
			"driverVersion":   env("GVAULT_DRIVER", "550.00.fake"),
			"architecture":    env("GVAULT_ARCH", "amd64"),
			"imageDigest":     env("GVAULT_DIGEST", "sha256:fake-image-digest"),
		}
		if len(os.Args) >= 6 {
			body["targetNodeName"] = os.Args[5]
		}
		printJSON(mustDo(http.MethodPost, base+"/api/v1/snapshots/"+os.Args[3]+"/compatibility", body, ""))
	case "restore":
		need(7)
		body := map[string]any{
			"targetClusterId": os.Args[4],
			"namespace":       os.Args[5],
			"podName":         os.Args[6],
		}
		if len(os.Args) >= 8 {
			body["targetNodeName"] = os.Args[7]
		}
		if probe := env("GVAULT_PROBE_ENDPOINT", ""); probe != "" {
			body["probeEndpoint"] = probe
		}
		// Optional hardware overrides for preflight (FakeProvider defaults to snapshot metadata).
		if g := env("GVAULT_GPU", ""); g != "" {
			body["gpuProduct"] = g
		}
		if drv := env("GVAULT_DRIVER", ""); drv != "" {
			body["driverVersion"] = drv
		}
		if arch := env("GVAULT_ARCH", ""); arch != "" {
			body["architecture"] = arch
		}
		if d := env("GVAULT_DIGEST", ""); d != "" {
			body["imageDigest"] = d
		}
		idem := env("GVAULT_IDEMPOTENCY_KEY", "gvaultctl-restore-"+fmt.Sprint(time.Now().UnixNano()))
		printJSON(mustDo(http.MethodPost, base+"/api/v1/snapshots/"+os.Args[3]+"/restore", body, idem))
	case "tags":
		need(4)
		if len(os.Args) >= 5 && os.Args[4] == "set" {
			need(6)
			tags := map[string]string{}
			for _, kv := range os.Args[5:] {
				k, v, ok := strings.Cut(kv, "=")
				if !ok || k == "" {
					fatal(fmt.Errorf("tag must be key=value, got %q", kv))
				}
				tags[k] = v
			}
			printJSON(mustDo(http.MethodPut, base+"/api/v1/snapshots/"+os.Args[3]+"/tags", map[string]any{"tags": tags}, ""))
			return
		}
		printJSON(mustDo(http.MethodGet, base+"/api/v1/snapshots/"+os.Args[3]+"/tags", nil, ""))
	case "promote":
		need(5)
		printJSON(mustDo(http.MethodPost, base+"/api/v1/snapshots/"+os.Args[3]+"/promote", map[string]any{
			"versionTag": os.Args[4],
		}, ""))
	case "hold":
		need(5)
		hold, err := strconv.ParseBool(os.Args[4])
		if err != nil {
			fatal(fmt.Errorf("hold value must be true|false: %w", err))
		}
		printJSON(mustDo(http.MethodPost, base+"/api/v1/snapshots/"+os.Args[3]+"/hold", map[string]any{
			"hold": hold,
		}, ""))
	case "wait":
		need(4)
		// Wait until snapshot reaches READY|FAILED|DELETED
		waitSnapshot(base, os.Args[3])
	default:
		usage()
		os.Exit(2)
	}
}

func cmdOperation(base string) {
	need(3)
	switch os.Args[2] {
	case "list":
		printJSON(mustDo(http.MethodGet, base+"/api/v1/operations", nil, ""))
	case "get":
		need(4)
		printJSON(mustDo(http.MethodGet, base+"/api/v1/operations/"+os.Args[3], nil, ""))
	case "wait":
		need(4)
		waitOperation(base, os.Args[3])
	default:
		usage()
		os.Exit(2)
	}
}

func cmdCRD(base string) {
	need(3)
	switch os.Args[2] {
	case "list":
		printJSON(mustDo(http.MethodGet, base+"/api/v1/crds/vaultsnapshots", nil, ""))
	case "get":
		need(5)
		printJSON(mustDo(http.MethodGet, fmt.Sprintf("%s/api/v1/crds/vaultsnapshots/%s/%s", base, os.Args[3], os.Args[4]), nil, ""))
	case "create":
		need(7)
		body := map[string]any{
			"name": os.Args[3], "namespace": os.Args[4],
			"podName": os.Args[5], "containerName": os.Args[6],
			"provider":  env("GVAULT_PROVIDER", "fake"),
			"clusterId": env("GVAULT_CLUSTER", "dev"),
		}
		printJSON(mustDo(http.MethodPost, base+"/api/v1/crds/vaultsnapshots", body, ""))
	case "reconcile":
		need(5)
		printJSON(mustDo(http.MethodPost, fmt.Sprintf("%s/api/v1/crds/vaultsnapshots/%s/%s/reconcile", base, os.Args[3], os.Args[4]), nil, ""))
	default:
		usage()
		os.Exit(2)
	}
}

func waitOperation(base, id string) {
	timeout := durationEnv("GVAULT_WAIT_TIMEOUT", 30*time.Second)
	poll := durationEnv("GVAULT_WAIT_POLL", 200*time.Millisecond)
	deadline := time.Now().Add(timeout)
	for {
		b := mustDo(http.MethodGet, base+"/api/v1/operations/"+id, nil, "")
		var op map[string]any
		_ = json.Unmarshal(b, &op)
		status, _ := op["status"].(string)
		fmt.Fprintf(os.Stderr, "operation %s status=%s\n", id, status)
		switch status {
		case "SUCCEEDED", "FAILED", "CANCELLED":
			printJSON(b)
			if status != "SUCCEEDED" {
				os.Exit(1)
			}
			return
		}
		if time.Now().After(deadline) {
			fatal(fmt.Errorf("timeout waiting for operation %s (last status=%s)", id, status))
		}
		time.Sleep(poll)
	}
}

func waitSnapshot(base, id string) {
	timeout := durationEnv("GVAULT_WAIT_TIMEOUT", 30*time.Second)
	poll := durationEnv("GVAULT_WAIT_POLL", 200*time.Millisecond)
	deadline := time.Now().Add(timeout)
	for {
		b := mustDo(http.MethodGet, base+"/api/v1/snapshots/"+id, nil, "")
		var snap map[string]any
		_ = json.Unmarshal(b, &snap)
		status, _ := snap["status"].(string)
		fmt.Fprintf(os.Stderr, "snapshot %s status=%s\n", id, status)
		switch status {
		case "READY", "FAILED", "DELETED":
			printJSON(b)
			if status != "READY" {
				os.Exit(1)
			}
			return
		}
		if time.Now().After(deadline) {
			fatal(fmt.Errorf("timeout waiting for snapshot %s (last status=%s)", id, status))
		}
		time.Sleep(poll)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `gvaultctl — Gvault CLI (%s)

Usage:
  gvaultctl <command> [args]

Core:
  gvaultctl version [--local]
  gvaultctl health
  gvaultctl ready
  gvaultctl overview
  gvaultctl providers
  gvaultctl providers nvidia-dry-run <namespace> <name> <pod> <container> [podUid]
  gvaultctl audits [limit]
  gvaultctl export s3 <snapshotId> <bucket> <key>
  gvaultctl wait <operationId>

Snapshots:
  gvaultctl snapshot list
  gvaultctl snapshot get <id>
  gvaultctl snapshot create <name> <clusterId> <namespace> <pod> <container>
  gvaultctl snapshot wait <id>
  gvaultctl snapshot check <id> <targetClusterId> [targetNode]
  gvaultctl snapshot restore <id> <clusterId> <namespace> <pod> [node]
  gvaultctl snapshot tags <id>
  gvaultctl snapshot tags <id> set key=value [key=value...]
  gvaultctl snapshot promote <id> <versionTag>
  gvaultctl snapshot hold <id> true|false
  gvaultctl snapshot delete <id>

Operations:
  gvaultctl operation list
  gvaultctl operation get <id>
  gvaultctl operation wait <id>

CRD bridge (GPU-free in-memory):
  gvaultctl crd list
  gvaultctl crd get <namespace> <name>
  gvaultctl crd create <name> <namespace> <pod> <container>
  gvaultctl crd reconcile <namespace> <name>

Environment:
  GVAULT_API                 API base URL (default http://localhost:8080)
  GVAULT_API_KEY             sent as X-API-Key when set
  GVAULT_BEARER_TOKEN        sent as Authorization: Bearer when set
  GVAULT_PROVIDER            provider for create/crd (default fake)
  GVAULT_NAMESPACE           filter for snapshot list
  GVAULT_CLUSTER             clusterId for CRD create (default dev)
  GVAULT_GPU / GVAULT_DRIVER / GVAULT_ARCH / GVAULT_DIGEST
  GVAULT_PROBE_ENDPOINT      optional post-restore probe URL
  GVAULT_IDEMPOTENCY_KEY     override Idempotency-Key on mutations
  GVAULT_WAIT_TIMEOUT        wait timeout (default 30s)
  GVAULT_WAIT_POLL           wait poll interval (default 200ms)

Note: FakeProvider READY/SUCCEEDED are simulated — not real GPU checkpoints.
`, cliVersion)
}

func need(n int) {
	if len(os.Args) < n {
		usage()
		os.Exit(2)
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func durationEnv(k string, d time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return d
	}
	return parsed
}

func flagBool(name string) bool {
	for _, a := range os.Args[2:] {
		if a == name {
			return true
		}
	}
	return false
}

func mustDo(method, url string, body any, idem string) []byte {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			fatal(err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	if key := os.Getenv("GVAULT_API_KEY"); key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if tok := os.Getenv("GVAULT_BEARER_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		fatal(fmt.Errorf("%s %s: %s\n%s", method, url, res.Status, b))
	}
	return b
}

func printJSON(b []byte) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		fmt.Println(string(b))
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
