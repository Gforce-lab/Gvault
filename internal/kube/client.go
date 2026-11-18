package kube

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client is a thin dynamic client used by the NVIDIA adapter (optional).
type Client struct {
	dyn dynamic.Interface
}

// MaybeFromEnv loads an in-cluster or kubeconfig client.
// Returns (nil, nil) when KUBECONFIG/in-cluster are unavailable — callers stay fail-closed.
func MaybeFromEnv() (*Client, error) {
	cfg, err := restConfig()
	if err != nil {
		return nil, nil // not an error: no cluster configured
	}
	if cfg == nil {
		return nil, nil
	}
	return NewForConfig(cfg)
}

// NewForConfig constructs the same dynamic client used by the live adapter.
// Tests can point it at a local Kubernetes HTTP fixture without kubeconfig changes.
func NewForConfig(cfg *rest.Config) (*Client, error) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}
	return &Client{dyn: dyn}, nil
}

func restConfig() (*rest.Config, error) {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if home, _ := os.UserHomeDir(); home != "" {
		path := home + "/.kube/config"
		if _, err := os.Stat(path); err == nil {
			return clientcmd.BuildConfigFromFlags("", path)
		}
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil
	}
	return cfg, nil
}

func (c *Client) Live() bool { return c != nil && c.dyn != nil }

func podSnapshotGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "nvidia.com", Version: "v1alpha1", Resource: "podsnapshots"}
}

func podsGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
}

// CreatePodSnapshot creates a namespaced PodSnapshot from an unstructured object.
func (c *Client) CreatePodSnapshot(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if !c.Live() {
		return nil, fmt.Errorf("kubernetes client not connected")
	}
	ns := obj.GetNamespace()
	return c.dyn.Resource(podSnapshotGVR()).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
}

// GetPodSnapshot fetches a PodSnapshot by name.
func (c *Client) GetPodSnapshot(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	if !c.Live() {
		return nil, fmt.Errorf("kubernetes client not connected")
	}
	return c.dyn.Resource(podSnapshotGVR()).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// DeletePodSnapshot deletes a PodSnapshot (provider lifecycle).
func (c *Client) DeletePodSnapshot(ctx context.Context, namespace, name string) error {
	if !c.Live() {
		return fmt.Errorf("kubernetes client not connected")
	}
	return c.dyn.Resource(podSnapshotGVR()).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// CreatePod creates a corev1 Pod (restore contract).
func (c *Client) CreatePod(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if !c.Live() {
		return nil, fmt.Errorf("kubernetes client not connected")
	}
	ns := obj.GetNamespace()
	return c.dyn.Resource(podsGVR()).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
}

// GetPod fetches a Pod.
func (c *Client) GetPod(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	if !c.Live() {
		return nil, fmt.Errorf("kubernetes client not connected")
	}
	return c.dyn.Resource(podsGVR()).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ConditionTrue reports whether an unstructured status.conditions entry is True.
func ConditionTrue(obj *unstructured.Unstructured, condType string) bool {
	conds, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, raw := range conds {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == condType && m["status"] == "True" {
			return true
		}
	}
	return false
}
