package kube

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// API is the subset of Kubernetes operations the NVIDIA adapter needs.
// Production uses *Client; tests inject fakes.
type API interface {
	Live() bool
	CreatePodSnapshot(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	GetPodSnapshot(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error)
	DeletePodSnapshot(ctx context.Context, namespace, name string) error
	CreatePod(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	GetPod(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error)
}

// Ensure *Client implements API.
var _ API = (*Client)(nil)
