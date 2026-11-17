package domain

import "time"

// SnapshotStatus is the catalog lifecycle of a registered snapshot.
type SnapshotStatus string

const (
	SnapshotPending   SnapshotStatus = "PENDING"
	SnapshotCapturing SnapshotStatus = "CAPTURING"
	SnapshotReady     SnapshotStatus = "READY"
	SnapshotFailed    SnapshotStatus = "FAILED"
	SnapshotDeleting  SnapshotStatus = "DELETING"
	SnapshotDeleted   SnapshotStatus = "DELETED"
)

// ProviderSnapshotRef is an opaque binding to an upstream provider resource.
type ProviderSnapshotRef struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

// ArtifactRef stores opaque provider artifact locators (never checkpoint bytes).
type ArtifactRef struct {
	Backend string         `json:"backend"`
	StoreID string         `json:"storeId,omitempty"`
	Handle  string         `json:"handle,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// Snapshot is the searchable catalog entity.
type Snapshot struct {
	ID                  string
	Name                string
	Namespace           string
	ClusterID           string
	Status              SnapshotStatus
	Provider            string
	ProviderVersion     string
	ProviderSnapshotRef ProviderSnapshotRef
	ProviderContentName string
	SourcePodName       string
	SourcePodUID        string
	SourceContainerName string
	ModelName           string
	ModelRevision       string
	Framework           string
	FrameworkVersion    string
	ImageDigest         string
	GPUProduct          string
	GPUCount            int
	DriverVersion       string
	Architecture        string
	KernelVersion       string
	Artifact            ArtifactRef
	SizeBytes           *int64
	Labels              map[string]string
	CreatedAt           time.Time
	ReadyAt             *time.Time
	DeletedAt           *time.Time
	ResourceVersion     int64
}

// CaptureRequest is the API/service intent to create a snapshot.
type CaptureRequest struct {
	Name                string
	ClusterID           string
	Namespace           string
	SourcePodName       string
	SourceContainerName string
	Provider            string
	Labels              map[string]string
	IdempotencyKey      string
	RequestedBy         string
	RequestID           string
	// Optional discovered metadata (FakeProvider fills defaults if empty).
	ImageDigest   string
	GPUProduct    string
	DriverVersion string
	Architecture  string
}
