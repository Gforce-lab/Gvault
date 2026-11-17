package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	v1alpha1 "github.com/gvault/gvault/api/v1alpha1"
	"github.com/gvault/gvault/internal/controller"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider/fake"
)

// Local GPU-free controller demo: reconciles in-memory VaultSnapshot CRDs via FakeProvider.
// Full controller-runtime manager + kube client is the production packaging target.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	fp := fake.New(50 * time.Millisecond)
	store := controller.NewStore()
	rec := controller.NewReconciler(store, fp)

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
	log.Info("gvault-controller demo", "note", "in-memory FakeProvider reconcile — not a real GPU path")

	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, err := rec.ReconcileSnapshot(ctx, "inference", "demo")
		if err != nil {
			log.Error("reconcile", "error", err)
			os.Exit(1)
		}
		enc, _ := json.MarshalIndent(out.Status, "", "  ")
		fmt.Println(string(enc))
		if out.Status.Phase == string(domain.SnapshotReady) || out.Status.Phase == string(domain.SnapshotFailed) {
			return
		}
		if time.Now().After(deadline) {
			log.Error("timeout")
			os.Exit(1)
		}
		time.Sleep(40 * time.Millisecond)
	}
}
