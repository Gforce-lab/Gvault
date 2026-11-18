package kube_test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/gvault/gvault/internal/kube"
)

func TestMemoryAPILifecycle(t *testing.T) {
	mem := kube.NewMemoryAPI()
	mem.ReadyOnGet = true
	ctx := context.Background()
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nvidia.com/v1alpha1",
		"kind":       "PodSnapshot",
		"metadata":   map[string]any{"name": "s1", "namespace": "ns"},
		"spec":       map[string]any{},
	}}
	created, err := mem.CreatePodSnapshot(ctx, obj)
	if err != nil || created.GetUID() == "" {
		t.Fatalf("create=%v err=%v", created, err)
	}
	got, err := mem.GetPodSnapshot(ctx, "ns", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if !kube.ConditionTrue(got, "Ready") {
		t.Fatal("expected Ready condition")
	}
	digest, _, _ := unstructured.NestedString(got.Object, "status", "imageDigest")
	if digest != kube.TestDigest {
		t.Fatalf("digest=%s", digest)
	}
	pod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": "p1", "namespace": "ns"},
		"spec":     map[string]any{},
	}}
	cp, err := mem.CreatePod(ctx, pod)
	if err != nil {
		t.Fatal(err)
	}
	gp, err := mem.GetPod(ctx, "ns", cp.GetName())
	if err != nil {
		t.Fatal(err)
	}
	phase, _, _ := unstructured.NestedString(gp.Object, "status", "phase")
	if phase != "Running" {
		t.Fatalf("phase=%s", phase)
	}
	if !kube.ConditionTrue(gp, "Ready") {
		t.Fatal("expected Pod Ready")
	}
	if err := mem.DeletePodSnapshot(ctx, "ns", "s1"); err != nil {
		t.Fatal(err)
	}
}

func TestConditionTrue(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False"},
				map[string]any{"type": "Ready", "status": "True"},
			},
		},
	}}
	if !kube.ConditionTrue(obj, "Ready") {
		t.Fatal("expected true")
	}
	if kube.ConditionTrue(obj, "Missing") {
		t.Fatal("expected false")
	}
}
