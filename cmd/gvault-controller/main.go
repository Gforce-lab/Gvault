package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1alpha1 "github.com/gvault/gvault/api/v1alpha1"
	"github.com/gvault/gvault/internal/controller"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider/fake"
)

// Local controller demo uses the CI harness provider for GPU-free reconcile loops.
// Product deployments use nvidia-snapshot against a live cluster (see docs/scope.md).
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	fp := fake.New(50 * time.Millisecond)
	store := controller.NewStore()

	obj := &v1alpha1.VaultSnapshot{
		APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		Kind:       v1alpha1.KindSnapshot,
		Metadata:   v1alpha1.ObjectMeta{Name: "demo", Namespace: "inference", Generation: 1},
		Spec: v1alpha1.VaultSnapshotSpec{
			Provider: "fake", ClusterID: "dev",
			Source: v1alpha1.VaultSnapshotSource{PodName: "llama-vllm-0", ContainerName: "main"},
		},
	}
	store.PutSnapshot(obj)

	mgr := controller.NewManager(store, fp, log, controller.ManagerConfig{
		LeaderElection:    true,
		LeaderElectionID:  "gvault-controller-demo",
		ReconcileInterval: 40 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)
	log.Info("gvault-controller demo", "leading", mgr.IsLeading(), "note", "in-memory FakeProvider reconcile — not a real GPU path")

	deadline := time.Now().Add(5 * time.Second)
	for {
		out, ok := store.GetSnapshot("inference", "demo")
		if !ok {
			log.Error("missing snapshot")
			os.Exit(1)
		}
		enc, _ := json.MarshalIndent(out.Status, "", "  ")
		fmt.Println(string(enc))
		if out.Status.Phase == string(domain.SnapshotReady) || out.Status.Phase == string(domain.SnapshotFailed) {
			mgr.Stop()
			return
		}
		if time.Now().After(deadline) {
			log.Error("timeout")
			os.Exit(1)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func init() {
	// Allow SIGINT during longer demos when run interactively.
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		os.Exit(0)
	}()
}
