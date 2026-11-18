package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	base := env("GVAULT_API", "http://localhost:8080")
	switch os.Args[1] {
	case "version":
		printJSON(mustGet(base + "/api/v1/version"))
	case "overview":
		printJSON(mustGet(base + "/api/v1/overview"))
	case "snapshot":
		if len(os.Args) < 3 {
			usage()
			os.Exit(2)
		}
		switch os.Args[2] {
		case "list":
			printJSON(mustGet(base + "/api/v1/snapshots"))
		case "get":
			need(4)
			printJSON(mustGet(base + "/api/v1/snapshots/" + os.Args[3]))
		case "create":
			need(8)
			body := map[string]any{
				"name": os.Args[3], "clusterId": os.Args[4], "namespace": os.Args[5],
				"provider": "fake",
				"source":   map[string]string{"podName": os.Args[6], "containerName": os.Args[7]},
			}
			printJSON(mustPost(base+"/api/v1/snapshots", body, "gvaultctl-capture-"+fmt.Sprint(time.Now().UnixNano())))
		case "check":
			need(5)
			body := map[string]any{
				"targetClusterId": os.Args[4],
				"gpuProduct":      env("GVAULT_GPU", "NVIDIA-Fake-A100-80GB"),
				"driverVersion":   env("GVAULT_DRIVER", "550.00.fake"),
				"architecture":    env("GVAULT_ARCH", "amd64"),
				"imageDigest":     env("GVAULT_DIGEST", "sha256:fake-image-digest"),
			}
			printJSON(mustPost(base+"/api/v1/snapshots/"+os.Args[3]+"/compatibility", body, ""))
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
			printJSON(mustPost(base+"/api/v1/snapshots/"+os.Args[3]+"/restore", body, "gvaultctl-restore-"+fmt.Sprint(time.Now().UnixNano())))
		default:
			usage()
			os.Exit(2)
		}
	case "operation":
		need(3)
		if os.Args[2] == "list" {
			printJSON(mustGet(base + "/api/v1/operations"))
			return
		}
		need(4)
		if os.Args[2] == "get" {
			printJSON(mustGet(base + "/api/v1/operations/" + os.Args[3]))
			return
		}
		usage()
		os.Exit(2)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `gvaultctl — Gvault CLI (0.1.0-beta)

  gvaultctl version
  gvaultctl overview
  gvaultctl snapshot list
  gvaultctl snapshot get <id>
  gvaultctl snapshot create <name> <clusterId> <namespace> <pod> <container>
  gvaultctl snapshot check <id> <targetClusterId>
  gvaultctl snapshot restore <id> <clusterId> <namespace> <pod> [node]
  gvaultctl operation list
  gvaultctl operation get <id>

Env: GVAULT_API (default http://localhost:8080)
`)
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

func mustGet(url string) []byte {
	res, err := http.Get(url)
	if err != nil {
		fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		fatal(fmt.Errorf("GET %s: %s\n%s", url, res.Status, b))
	}
	return b
}

func mustPost(url string, body any, idem string) []byte {
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		fatal(fmt.Errorf("POST %s: %s\n%s", url, res.Status, b))
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
