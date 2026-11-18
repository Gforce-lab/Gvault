package nvidia_test

import (
	"context"
	"testing"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/kube"
	"github.com/gvault/gvault/internal/provider"
	"github.com/gvault/gvault/internal/provider/nvidia"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type observedPodAPI struct {
	kube.API
	pod *unstructured.Unstructured
}

func (f observedPodAPI) Live() bool { return true }
func (f observedPodAPI) GetPod(context.Context, string, string) (*unstructured.Unstructured, error) {
	return f.pod, nil
}

func TestRestoreWaitsForProviderAndPodReadiness(t *testing.T) {
	for _, tc := range []struct {
		name, phase, restored, reason, ready string
		want                                 domain.OperationStatus
	}{
		{"proxy running", "Running", "", "", "True", domain.OperationRunning},
		{"restoring", "Running", "False", "RestoreInProgress", "True", domain.OperationRunning},
		{"unknown condition", "Running", "Unknown", "", "True", domain.OperationRunning},
		{"unknown reason", "Running", "False", "FutureReason", "True", domain.OperationRunning},
		{"application warming", "Running", "True", "RestoreSucceeded", "False", domain.OperationRunning},
		{"ready condition absent", "Running", "True", "RestoreSucceeded", "", domain.OperationRunning},
		{"still pending", "Pending", "True", "RestoreSucceeded", "True", domain.OperationRunning},
		{"serving", "Running", "True", "RestoreSucceeded", "True", domain.OperationSucceeded},
		{"restore failed", "Running", "False", "RestoreFailed", "True", domain.OperationFailed},
		{"incompatible", "Running", "False", "RestoreIncompatible", "True", domain.OperationFailed},
		{"partial restore", "Running", "False", "RestorePartiallySucceeded", "True", domain.OperationFailed},
		{"process failed", "Failed", "", "", "", domain.OperationFailed},
		{"process exited", "Succeeded", "True", "RestoreSucceeded", "True", domain.OperationFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conditions := []any{}
			if tc.restored != "" {
				conditions = append(conditions, map[string]any{"type": "nvidia.com/Restored", "status": tc.restored, "reason": tc.reason})
			}
			if tc.ready != "" {
				conditions = append(conditions, map[string]any{"type": "Ready", "status": tc.ready})
			}
			pod := &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{"uid": "worker-uid"},
				"status":   map[string]any{"phase": tc.phase, "conditions": conditions, "podIP": "10.0.0.2"},
			}}
			p := nvidia.New(true).WithKubeClient(observedPodAPI{pod: pod})
			state, err := p.RestoreStatus(context.Background(), provider.ProviderRef{Name: "worker", UID: "worker-uid"})
			if err != nil || state.Phase != tc.want || state.PodIP != "10.0.0.2" {
				t.Fatalf("state=%+v err=%v", state, err)
			}
		})
	}
}

func TestRestoreRejectsReplacedPod(t *testing.T) {
	pod := &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"uid": "replacement"}}}
	p := nvidia.New(true).WithKubeClient(observedPodAPI{pod: pod})
	for _, uid := range []string{"original", ""} {
		if _, err := p.RestoreStatus(context.Background(), provider.ProviderRef{Name: "worker", UID: uid}); err == nil {
			t.Fatal("expected missing or mismatched UID to fail closed")
		}
	}
}

func TestRestoreRejectsTerminatingPod(t *testing.T) {
	pod := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"uid": "worker", "deletionTimestamp": "2026-11-18T00:00:00Z"},
	}}
	p := nvidia.New(true).WithKubeClient(observedPodAPI{pod: pod})
	state, err := p.RestoreStatus(context.Background(), provider.ProviderRef{Name: "worker", UID: "worker"})
	if err != nil || state.Phase != domain.OperationFailed {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}
