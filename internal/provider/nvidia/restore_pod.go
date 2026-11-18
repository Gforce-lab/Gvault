package nvidia

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gvault/gvault/internal/provider"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// restorePod preserves the caller's workload spec and supplies only the pinned
// Snapshot control protocol. It never mutates the request or audits its contents.
func restorePod(req provider.RestoreRequest) (*unstructured.Unstructured, error) {
	if req.RestorePodSpec == nil {
		return nil, fmt.Errorf("restorePodSpec is required for a live NVIDIA restore")
	}
	raw, err := json.Marshal(req.RestorePodSpec)
	if err != nil {
		return nil, fmt.Errorf("restorePodSpec must be JSON serializable")
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("invalid restorePodSpec")
	}
	if policy, exists := spec["restartPolicy"]; exists && policy != "Never" {
		return nil, fmt.Errorf("restorePodSpec.restartPolicy must be Never")
	}
	if node, _ := spec["nodeName"].(string); node != "" {
		return nil, fmt.Errorf("use targetNodeName instead of restorePodSpec.nodeName")
	}
	spec["restartPolicy"] = "Never"
	if req.TargetNodeName != "" {
		selector, _, err := unstructured.NestedStringMap(spec, "nodeSelector")
		if err != nil {
			return nil, fmt.Errorf("invalid restorePodSpec.nodeSelector")
		}
		if selector == nil {
			selector = map[string]string{}
		}
		if node := selector["kubernetes.io/hostname"]; node != "" && node != req.TargetNodeName {
			return nil, fmt.Errorf("restorePodSpec.nodeSelector conflicts with targetNodeName")
		}
		selector["kubernetes.io/hostname"] = req.TargetNodeName
		_ = unstructured.SetNestedStringMap(spec, selector, "nodeSelector")
	}
	name := req.Snapshot.SourceContainerName
	if name == "" {
		return nil, fmt.Errorf("snapshot source container name is required")
	}
	containers, _, err := unstructured.NestedSlice(spec, "containers")
	if err != nil {
		return nil, fmt.Errorf("invalid restorePodSpec.containers")
	}
	found := false
	for _, rawContainer := range containers {
		container, ok := rawContainer.(map[string]any)
		if !ok || container["name"] != name {
			continue
		}
		if found {
			return nil, fmt.Errorf("duplicate restore container name")
		}
		found = true
		image, _ := container["image"].(string)
		_, digest, pinned := strings.Cut(image, "@")
		if !pinned || !strings.HasPrefix(digest, "sha256:") || len(digest) != 71 {
			return nil, fmt.Errorf("restore container image must be pinned by sha256 digest")
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(digest, "sha256:")); err != nil {
			return nil, fmt.Errorf("restore container sha256 digest must be hexadecimal")
		}
		expected := req.Snapshot.ImageDigest
		if _, suffix, ok := strings.Cut(expected, "@"); ok {
			expected = suffix
		}
		if digest != expected {
			return nil, fmt.Errorf("restore container image differs from snapshot image digest")
		}
		gpu, exists, err := unstructured.NestedFieldNoCopy(container, "resources", "limits", "nvidia.com/gpu")
		quantity, quantityErr := resource.ParseQuantity(fmt.Sprint(gpu))
		count, integral := quantity.AsInt64()
		if err != nil || !exists || quantityErr != nil || !integral || count < 1 || (req.Snapshot.GPUCount > 0 && count != int64(req.Snapshot.GPUCount)) {
			return nil, fmt.Errorf("restore container must request the snapshot's NVIDIA GPU count")
		}
		if err := ensureNamedEntry(container, "env", map[string]any{"name": "SNAPSHOT_CONTROL_DIR", "value": "/snapshot-control"}); err != nil {
			return nil, err
		}
		if err := ensureNamedEntry(container, "volumeMounts", map[string]any{
			"name": "snapshot-control", "mountPath": "/snapshot-control", "subPath": name,
		}); err != nil {
			return nil, err
		}
	}
	if !found {
		return nil, fmt.Errorf("restorePodSpec must contain the snapshot source container")
	}
	if err := ensureNamedEntry(spec, "volumes", map[string]any{"name": "snapshot-control", "emptyDir": map[string]any{}}); err != nil {
		return nil, err
	}
	spec["containers"] = containers
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{
			"name": req.PodName, "namespace": req.Namespace,
			"annotations": map[string]any{RestoreFromAnnotation: req.ProviderRef.Name},
			"labels":      map[string]any{"app.kubernetes.io/managed-by": "Gvault"},
		},
		"spec": spec,
	}}, nil
}

// Require canonical protocol entries instead of silently replacing conflicting
// mounts or environment. Other entries and sidecars remain caller-owned.
func ensureNamedEntry(object map[string]any, field string, canonical map[string]any) error {
	entries, _, err := unstructured.NestedSlice(object, field)
	if err != nil {
		return fmt.Errorf("invalid restorePodSpec %s", field)
	}
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid restorePodSpec %s entry", field)
		}
		if field == "volumeMounts" && entry["mountPath"] == "/snapshot-control" && entry["name"] != "snapshot-control" {
			return fmt.Errorf("restorePodSpec mount conflicts with Snapshot control directory")
		}
		if entry["name"] == canonical["name"] {
			actual, _ := json.Marshal(entry)
			expected, _ := json.Marshal(canonical)
			if string(actual) != string(expected) {
				return fmt.Errorf("restorePodSpec %s conflicts with Snapshot control protocol", field)
			}
			return nil
		}
	}
	object[field] = append(entries, canonical)
	return nil
}
