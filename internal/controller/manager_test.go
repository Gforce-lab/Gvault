package controller_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	v1alpha1 "github.com/gvault/gvault/api/v1alpha1"
	"github.com/gvault/gvault/internal/controller"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider/nvidia"
)

func TestManagerReconcilesWithLeaderElection(t *testing.T) {
	p, _ := nvidia.NewWithMemoryCluster()
	store := controller.NewStore()
	store.PutSnapshot(&v1alpha1.VaultSnapshot{
		Metadata: v1alpha1.ObjectMeta{Name: "m1", Namespace: "ns", Generation: 1},
		Spec: v1alpha1.VaultSnapshotSpec{
			Provider: "nvidia-snapshot", ClusterID: "gpu-cluster-a",
			Source: v1alpha1.VaultSnapshotSource{PodName: "p", ContainerName: "c"},
		},
	})
	mgr := controller.NewManager(store, p, slog.Default(), controller.ManagerConfig{
		LeaderElection: true, ReconcileInterval: 15 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)
	if !mgr.IsLeading() {
		t.Fatal("expected leading")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		obj, _ := store.GetSnapshot("ns", "m1")
		if obj.Status.Phase == string(domain.SnapshotReady) {
			mgr.Stop()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout phase=%s", obj.Status.Phase)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
