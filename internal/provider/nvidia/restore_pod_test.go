package nvidia

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func testRestoreRequest() provider.RestoreRequest {
	digest := "sha256:" + strings.Repeat("a", 64)
	return provider.RestoreRequest{
		Namespace: "inference", PodName: "restored", TargetNodeName: "node-b",
		ProviderRef: provider.ProviderRef{Name: "snapshot"},
		Snapshot:    domain.SnapshotInfo{SourceContainerName: "engine", ImageDigest: digest, GPUCount: 1},
		RestorePodSpec: map[string]any{
			"runtimeClassName": "nvidia",
			"nodeSelector":     map[string]any{"nvidia.com/gpu.product": "NVIDIA-B200"},
			"containers": []any{map[string]any{
				"name": "engine", "image": "example/vllm@" + digest,
				"command":      []any{"python3", "/app/worker.py"},
				"resources":    map[string]any{"limits": map[string]any{"nvidia.com/gpu": "1"}},
				"volumeMounts": []any{map[string]any{"name": "model", "mountPath": "/model"}},
			}},
			"volumes": []any{map[string]any{"name": "model", "persistentVolumeClaim": map[string]any{"claimName": "model-cache"}}},
		},
	}
}

func TestDocumentedVLLMPodSpecImplementsRestoreContract(t *testing.T) {
	raw, err := os.ReadFile("../../../examples/nvidia/restore-pod-spec.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	containers := spec["containers"].([]any)
	image := containers[0].(map[string]any)["image"].(string)
	req := testRestoreRequest()
	req.Snapshot.SourceContainerName = "main"
	req.Snapshot.ImageDigest = image
	req.RestorePodSpec = spec
	if _, err := restorePod(req); err != nil {
		t.Fatalf("documented vLLM spec is unusable: %v", err)
	}
}

func TestRestorePodPreservesWorkloadAndAddsControlContract(t *testing.T) {
	req := testRestoreRequest()
	before, _ := json.Marshal(req.RestorePodSpec)
	pod, err := restorePod(req)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(req.RestorePodSpec)
	if string(before) != string(after) {
		t.Fatal("restore mutated caller's Pod spec")
	}
	if pod.GetName() != "restored" || pod.GetNamespace() != "inference" || pod.GetAnnotations()[RestoreFromAnnotation] != "snapshot" {
		t.Fatalf("metadata=%v", pod.Object["metadata"])
	}
	spec := pod.Object["spec"].(map[string]any)
	selector, _, _ := unstructured.NestedStringMap(spec, "nodeSelector")
	if selector["kubernetes.io/hostname"] != "node-b" || selector["nvidia.com/gpu.product"] != "NVIDIA-B200" || spec["runtimeClassName"] != "nvidia" || spec["restartPolicy"] != "Never" {
		t.Fatalf("spec=%v", spec)
	}
	containers, _, _ := unstructured.NestedSlice(spec, "containers")
	container := containers[0].(map[string]any)
	env, _, _ := unstructured.NestedSlice(container, "env")
	mounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
	volumes, _, _ := unstructured.NestedSlice(spec, "volumes")
	command, _, _ := unstructured.NestedStringSlice(container, "command")
	if len(env) != 1 || len(mounts) != 2 || len(volumes) != 2 || len(command) != 2 {
		t.Fatalf("protocol or workload fields missing: %v", spec)
	}
	// Applying the protocol again is idempotent.
	req.RestorePodSpec = spec
	again, err := restorePod(req)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := json.Marshal(pod.Object)
	two, _ := json.Marshal(again.Object)
	if string(one) != string(two) {
		t.Fatal("restore protocol transformation is not idempotent")
	}
}

func TestRestorePodRejectsInvalidOrConflictingWorkloads(t *testing.T) {
	for _, name := range []string{"missing spec", "missing container", "mutable image", "wrong digest", "no GPU", "wrong GPU count", "restart policy", "node override", "selector conflict", "control env", "control volume", "control mount"} {
		t.Run(name, func(t *testing.T) {
			req := testRestoreRequest()
			container := req.RestorePodSpec["containers"].([]any)[0].(map[string]any)
			switch name {
			case "missing spec":
				req.RestorePodSpec = nil
			case "missing container":
				req.Snapshot.SourceContainerName = "other"
			case "mutable image":
				container["image"] = "example/vllm:latest"
			case "wrong digest":
				req.Snapshot.ImageDigest = "sha256:" + strings.Repeat("b", 64)
			case "no GPU":
				delete(container, "resources")
			case "wrong GPU count":
				req.Snapshot.GPUCount = 2
			case "restart policy":
				req.RestorePodSpec["restartPolicy"] = "Always"
			case "node override":
				req.RestorePodSpec["nodeName"] = "node-a"
			case "selector conflict":
				req.RestorePodSpec["nodeSelector"] = map[string]any{"kubernetes.io/hostname": "node-a"}
			case "control env":
				container["env"] = []any{map[string]any{"name": "SNAPSHOT_CONTROL_DIR", "value": "/wrong"}}
			case "control volume":
				req.RestorePodSpec["volumes"] = []any{map[string]any{"name": "snapshot-control", "hostPath": map[string]any{"path": "/host"}}}
			case "control mount":
				container["volumeMounts"] = []any{map[string]any{"name": "model", "mountPath": "/snapshot-control"}}
			}
			if _, err := restorePod(req); err == nil {
				t.Fatal("expected invalid workload to fail before Pod creation")
			}
		})
	}
}
