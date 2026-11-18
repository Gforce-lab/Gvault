package kube

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// FakeAPI is an in-memory ClusterAPI for unit tests (not a real GPU path).
type FakeAPI struct {
	mu           sync.Mutex
	snapshots    map[string]*unstructured.Unstructured
	pods         map[string]*unstructured.Unstructured
	ReadyOnGet   bool
	FailCreate   bool
	uidCounter   int
}

func NewFakeAPI() *FakeAPI {
	return &FakeAPI{
		snapshots: map[string]*unstructured.Unstructured{},
		pods:      map[string]*unstructured.Unstructured{},
	}
}

func (f *FakeAPI) Live() bool { return true }

func (f *FakeAPI) key(ns, name string) string { return ns + "/" + name }

func (f *FakeAPI) CreatePodSnapshot(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailCreate {
		return nil, fmt.Errorf("fake create PodSnapshot failed")
	}
	f.uidCounter++
	out := cloneUnstructured(obj)
	out.SetUID(types.UID(fmt.Sprintf("ps-uid-%d", f.uidCounter)))
	_ = unstructured.SetNestedField(out.Object, "Pending", "status", "phase")
	f.snapshots[f.key(out.GetNamespace(), out.GetName())] = out
	return cloneUnstructured(out), nil
}

func (f *FakeAPI) GetPodSnapshot(_ context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.snapshots[f.key(namespace, name)]
	if !ok {
		return nil, fmt.Errorf("PodSnapshot %s/%s not found", namespace, name)
	}
	out := cloneUnstructured(obj)
	if f.ReadyOnGet {
		_ = unstructured.SetNestedSlice(out.Object, []any{
			map[string]any{"type": "Ready", "status": "True"},
		}, "status", "conditions")
		_ = unstructured.SetNestedField(out.Object, "Ready", "status", "phase")
		_ = unstructured.SetNestedField(out.Object, "sha256:live-test-digest", "status", "imageDigest")
		_ = unstructured.SetNestedField(out.Object, "NVIDIA-A100-80GB", "status", "gpuProduct")
		_ = unstructured.SetNestedField(out.Object, "550.54.15", "status", "driverVersion")
	}
	return out, nil
}

func (f *FakeAPI) DeletePodSnapshot(_ context.Context, namespace, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.snapshots, f.key(namespace, name))
	return nil
}

func (f *FakeAPI) CreatePod(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailCreate {
		return nil, fmt.Errorf("fake create Pod failed")
	}
	f.uidCounter++
	out := cloneUnstructured(obj)
	out.SetUID(types.UID(fmt.Sprintf("pod-uid-%d", f.uidCounter)))
	_ = unstructured.SetNestedField(out.Object, "Pending", "status", "phase")
	f.pods[f.key(out.GetNamespace(), out.GetName())] = out
	return cloneUnstructured(out), nil
}

func (f *FakeAPI) GetPod(_ context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.pods[f.key(namespace, name)]
	if !ok {
		return nil, fmt.Errorf("Pod %s/%s not found", namespace, name)
	}
	out := cloneUnstructured(obj)
	if f.ReadyOnGet {
		_ = unstructured.SetNestedField(out.Object, "Running", "status", "phase")
		_ = unstructured.SetNestedField(out.Object, "gpu-node-b", "spec", "nodeName")
	}
	return out, nil
}

func cloneUnstructured(in *unstructured.Unstructured) *unstructured.Unstructured {
	if in == nil {
		return nil
	}
	return &unstructured.Unstructured{Object: deepCopyMap(in.Object)}
}

func deepCopyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopyAny(v)
	}
	return out
}

func deepCopyAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepCopyMap(t)
	case map[string]string:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			m[k] = vv
		}
		return m
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = deepCopyAny(t[i])
		}
		return out
	case []string:
		out := make([]any, len(t))
		for i := range t {
			out[i] = t[i]
		}
		return out
	default:
		return v
	}
}
