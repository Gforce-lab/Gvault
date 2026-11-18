package provider

import (
	"context"

	"github.com/gvault/gvault/internal/domain"
)

// ProviderRef is an opaque handle returned by a checkpoint provider.
type ProviderRef struct {
	Provider    string
	Kind        string
	Namespace   string
	Name        string
	UID         string
	ContentName string
}

// CaptureState is provider-reported capture progress.
type CaptureState struct {
	Phase       domain.SnapshotStatus
	Message     string
	ContentName string
	Artifact    domain.ArtifactRef
	// Discovered hardware/runtime metadata (authoritative for Fake/NVIDIA adapters).
	SourcePodUID  string
	ImageDigest   string
	GPUProduct    string
	GPUCount      int
	DriverVersion string
	Architecture  string
	KernelVersion string
	SizeBytes     *int64
}

// RestoreState is provider-reported restore progress.
type RestoreState struct {
	Phase   domain.OperationStatus
	Message string
	PodName string
	PodIP   string
	Node    string
}

// CaptureRequest is provider-facing capture intent.
type CaptureRequest struct {
	SnapshotID          string
	Name                string
	ClusterID           string
	Namespace           string
	SourcePodName       string
	SourceContainerName string
	Labels              map[string]string
	// Optional hints; provider should discover authoritative values when possible.
	ImageDigest   string
	GPUProduct    string
	DriverVersion string
	Architecture  string
}

// RestoreRequest is provider-facing restore intent.
type RestoreRequest struct {
	RestorePodSpec map[string]any
	SnapshotID     string
	ClusterID      string
	Namespace      string
	PodName        string
	TargetNodeName string
	Snapshot       domain.SnapshotInfo
	ProviderRef    ProviderRef
}

// CheckpointProvider abstracts external checkpoint systems (Fake, NVIDIA, …).
type CheckpointProvider interface {
	Name() string
	CheckPrerequisites(ctx context.Context, clusterID string) error
	RequestCapture(ctx context.Context, req CaptureRequest) (ProviderRef, error)
	CaptureStatus(ctx context.Context, ref ProviderRef) (CaptureState, error)
	EvaluateCompatibility(ctx context.Context, snapshot domain.SnapshotInfo, target domain.TargetInfo) domain.CompatibilityResult
	RequestRestore(ctx context.Context, req RestoreRequest) (ProviderRef, error)
	RestoreStatus(ctx context.Context, ref ProviderRef) (RestoreState, error)
	RequestDelete(ctx context.Context, ref ProviderRef) error
}
