package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1alpha1 "github.com/gvault/gvault/api/v1alpha1"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider"
)

const finalizerName = "vault.gvault.dev/finalizer"

// Reconciler drives VaultSnapshot / VaultRestore against a CheckpointProvider.
type Reconciler struct {
	Store    *MemoryStore
	Provider provider.CheckpointProvider
	refs     sync.Map // snap key -> ProviderRef
}

func NewReconciler(store *MemoryStore, p provider.CheckpointProvider) *Reconciler {
	return &Reconciler{Store: store, Provider: p}
}

// NewStore is an alias for NewMemoryStore (keeps existing call sites).
func NewStore() *MemoryStore { return NewMemoryStore() }

func (r *Reconciler) ReconcileSnapshot(ctx context.Context, ns, name string) (*v1alpha1.VaultSnapshot, error) {
	obj, ok := r.Store.GetSnapshot(ns, name)
	if !ok {
		return nil, domain.NewAppError(domain.CodeNotFound, "VaultSnapshot not found")
	}
	if obj.Spec.Provider == "" {
		obj.Spec.Provider = r.Provider.Name()
	}
	if obj.Spec.Provider != r.Provider.Name() && obj.Spec.Provider != "fake" {
		obj.Status.Phase = string(domain.SnapshotFailed)
		obj.Status.Message = fmt.Sprintf("provider %q not active", obj.Spec.Provider)
		r.Store.PutSnapshot(obj)
		return obj, nil
	}

	k := key(ns, name)
	var ref provider.ProviderRef
	if v, ok := r.refs.Load(k); ok {
		ref = v.(provider.ProviderRef)
	} else {
		created, err := r.Provider.RequestCapture(ctx, provider.CaptureRequest{
			SnapshotID:          "crd-" + name,
			Name:                name,
			ClusterID:           defaultString(obj.Spec.ClusterID, "local"),
			Namespace:           ns,
			SourcePodName:       obj.Spec.Source.PodName,
			SourceContainerName: obj.Spec.Source.ContainerName,
			Labels:              obj.Spec.Labels,
		})
		if err != nil {
			obj.Status.Phase = string(domain.SnapshotFailed)
			obj.Status.Message = err.Error()
			r.Store.PutSnapshot(obj)
			return obj, err
		}
		r.refs.Store(k, created)
		ref = created
		obj.Metadata.Finalizers = ensureFinalizer(obj.Metadata.Finalizers, finalizerName)
	}

	st, err := r.Provider.CaptureStatus(ctx, ref)
	if err != nil {
		obj.Status.Phase = string(domain.SnapshotFailed)
		obj.Status.Message = err.Error()
		r.Store.PutSnapshot(obj)
		return obj, err
	}
	obj.Status.Phase = string(st.Phase)
	obj.Status.Message = st.Message
	obj.Status.ProviderSnapshotName = ref.Name
	obj.Status.ProviderSnapshotUID = ref.UID
	obj.Status.ProviderContentName = st.ContentName
	obj.Status.Simulated = r.Provider.Name() == "fake"
	obj.Status.ObservedGeneration = obj.Metadata.Generation
	now := time.Now().UTC()
	obj.Status.LastTransitionTime = &now
	obj.Status.Conditions = []v1alpha1.Condition{{
		Type: "Ready", Status: boolStatus(st.Phase == domain.SnapshotReady),
		Reason: string(st.Phase), Message: st.Message, LastTransitionTime: now.Format(time.RFC3339),
	}}
	r.Store.PutSnapshot(obj)
	return obj, nil
}

func (r *Reconciler) ReconcileRestore(ctx context.Context, ns, name string) (*v1alpha1.VaultRestore, error) {
	obj, ok := r.Store.GetRestore(ns, name)
	if !ok {
		return nil, domain.NewAppError(domain.CodeNotFound, "VaultRestore not found")
	}
	snap, ok := r.Store.GetSnapshot(ns, obj.Spec.SnapshotRef)
	if !ok {
		obj.Status.Phase = string(domain.OperationFailed)
		obj.Status.Message = "snapshotRef not found"
		r.Store.PutRestore(obj)
		return obj, nil
	}
	if snap.Status.Phase != string(domain.SnapshotReady) {
		obj.Status.Phase = string(domain.OperationFailed)
		obj.Status.Message = "referenced snapshot is not READY"
		r.Store.PutRestore(obj)
		return obj, nil
	}

	node := ""
	if obj.Spec.Target.NodeSelector != nil {
		node = obj.Spec.Target.NodeSelector["kubernetes.io/hostname"]
	}
	info := domain.SnapshotInfo{
		ID: snap.Status.CatalogSnapshotID, Status: domain.SnapshotReady,
		ClusterID: defaultString(snap.Spec.ClusterID, "local"), Namespace: ns,
		GPUProduct: "NVIDIA-Fake-A100-80GB", DriverVersion: "550.00.fake",
		Architecture: "amd64", ImageDigest: "sha256:fake-image-digest", Provider: r.Provider.Name(),
		GPUCount: 1,
	}
	ref, err := r.Provider.RequestRestore(ctx, provider.RestoreRequest{
		SnapshotID: defaultString(snap.Status.CatalogSnapshotID, snap.Metadata.Name),
		ClusterID:  info.ClusterID, Namespace: ns, PodName: obj.Spec.Target.PodName,
		TargetNodeName: node, Snapshot: info,
		ProviderRef: provider.ProviderRef{
			Provider: r.Provider.Name(), Kind: "FakePodSnapshot", Namespace: ns,
			Name: snap.Status.ProviderSnapshotName, UID: snap.Status.ProviderSnapshotUID,
			ContentName: snap.Status.ProviderContentName,
		},
	})
	if err != nil {
		obj.Status.Phase = string(domain.OperationFailed)
		obj.Status.Message = err.Error()
		r.Store.PutRestore(obj)
		return obj, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		st, err := r.Provider.RestoreStatus(ctx, ref)
		if err != nil {
			obj.Status.Phase = string(domain.OperationFailed)
			obj.Status.Message = err.Error()
			r.Store.PutRestore(obj)
			return obj, err
		}
		obj.Status.Phase = string(st.Phase)
		obj.Status.Message = st.Message
		obj.Status.Simulated = r.Provider.Name() == "fake"
		if st.Phase == domain.OperationSucceeded || st.Phase == domain.OperationFailed {
			r.Store.PutRestore(obj)
			return obj, nil
		}
		if time.Now().After(deadline) {
			obj.Status.Phase = string(domain.OperationFailed)
			obj.Status.Message = "restore reconcile timed out"
			r.Store.PutRestore(obj)
			return obj, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func ensureFinalizer(in []string, f string) []string {
	for _, x := range in {
		if x == f {
			return in
		}
	}
	return append(append([]string{}, in...), f)
}

func boolStatus(ok bool) string {
	if ok {
		return "True"
	}
	return "False"
}

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
