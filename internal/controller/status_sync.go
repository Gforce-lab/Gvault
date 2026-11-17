package controller

import (
	"context"
	"time"

	v1alpha1 "github.com/gvault/gvault/api/v1alpha1"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/repository/postgres"
)

// StatusSync projects reconciled CRD status into the Postgres catalog (beta).
// CRD status remains authoritative for the Kubernetes path; Postgres is searchable projection.
type StatusSync struct {
	Snaps *postgres.SnapshotRepo
}

func (s *StatusSync) UpsertFromVaultSnapshot(ctx context.Context, obj *v1alpha1.VaultSnapshot, catalogID string) error {
	if s == nil || s.Snaps == nil || obj == nil {
		return nil
	}
	existing, err := s.Snaps.Get(ctx, catalogID)
	if err != nil && !domain.IsNotFound(err) {
		return err
	}
	now := time.Now().UTC()
	if existing == nil {
		snap := &domain.Snapshot{
			ID: catalogID, Name: obj.Metadata.Name, Namespace: obj.Metadata.Namespace,
			ClusterID:     defaultString(obj.Spec.ClusterID, "local"),
			Status:        domain.SnapshotStatus(obj.Status.Phase),
			Provider:      defaultString(obj.Spec.Provider, "fake"),
			SourcePodName: obj.Spec.Source.PodName, SourceContainerName: obj.Spec.Source.ContainerName,
			ProviderSnapshotRef: domain.ProviderSnapshotRef{
				Kind: "FakePodSnapshot", Namespace: obj.Metadata.Namespace,
				Name: obj.Status.ProviderSnapshotName, UID: obj.Status.ProviderSnapshotUID,
			},
			ProviderContentName: obj.Status.ProviderContentName,
			Labels:              obj.Spec.Labels,
			CreatedAt:           now, ResourceVersion: 1,
			Artifact: domain.ArtifactRef{Backend: "fake"},
		}
		if snap.Status == domain.SnapshotReady {
			snap.ReadyAt = &now
		}
		return s.Snaps.Create(ctx, nil, snap)
	}
	existing.Status = domain.SnapshotStatus(obj.Status.Phase)
	existing.ProviderSnapshotRef.Name = obj.Status.ProviderSnapshotName
	existing.ProviderSnapshotRef.UID = obj.Status.ProviderSnapshotUID
	existing.ProviderContentName = obj.Status.ProviderContentName
	if existing.Status == domain.SnapshotReady && existing.ReadyAt == nil {
		existing.ReadyAt = &now
	}
	return s.Snaps.UpdateStatusAndMeta(ctx, existing)
}
