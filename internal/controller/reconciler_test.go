package controller_test

import (
	"context"
	"testing"
	"time"

	v1alpha1 "github.com/gvault/gvault/api/v1alpha1"
	"github.com/gvault/gvault/internal/controller"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider/nvidia"
)

func TestReconcileSnapshotIdempotent(t *testing.T) {
	p, _ := nvidia.NewWithMemoryCluster()
	store := controller.NewStore()
	rec := controller.NewReconciler(store, p)
	obj := &v1alpha1.VaultSnapshot{
		APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		Kind:       v1alpha1.KindSnapshot,
		Metadata:   v1alpha1.ObjectMeta{Name: "demo", Namespace: "inference", Generation: 1},
		Spec: v1alpha1.VaultSnapshotSpec{
			Provider: "nvidia-snapshot", ClusterID: "gpu-cluster-a",
			Source: v1alpha1.VaultSnapshotSource{PodName: "p0", ContainerName: "main"},
		},
	}
	store.PutSnapshot(obj)

	var last *v1alpha1.VaultSnapshot
	deadline := time.Now().Add(3 * time.Second)
	for {
		out, err := rec.ReconcileSnapshot(context.Background(), "inference", "demo")
		if err != nil {
			t.Fatal(err)
		}
		last = out
		if out.Status.Phase == string(domain.SnapshotReady) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout phase=%s msg=%s", out.Status.Phase, out.Status.Message)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if last.Status.ProviderSnapshotUID == "" {
		t.Fatal("expected provider uid")
	}
	again, err := rec.ReconcileSnapshot(context.Background(), "inference", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if again.Status.ProviderSnapshotUID != last.Status.ProviderSnapshotUID {
		t.Fatalf("idempotency broken: %s vs %s", again.Status.ProviderSnapshotUID, last.Status.ProviderSnapshotUID)
	}
}
