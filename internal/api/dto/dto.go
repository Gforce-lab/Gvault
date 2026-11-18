package dto

import (
	"time"

	"github.com/gvault/gvault/internal/domain"
)

type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"requestId"`
}

type CaptureBody struct {
	Name      string            `json:"name" binding:"required"`
	ClusterID string            `json:"clusterId" binding:"required"`
	Namespace string            `json:"namespace" binding:"required"`
	Source    SourceBody        `json:"source" binding:"required"`
	Provider  string            `json:"provider"`
	Labels    map[string]string `json:"labels"`
	// Optional hardware hints for FakeProvider demos.
	ImageDigest   string `json:"imageDigest"`
	GPUProduct    string `json:"gpuProduct"`
	DriverVersion string `json:"driverVersion"`
	Architecture  string `json:"architecture"`
}

type SourceBody struct {
	PodName       string `json:"podName" binding:"required"`
	ContainerName string `json:"containerName" binding:"required"`
}

type AcceptedBody struct {
	SnapshotID  string `json:"snapshotId,omitempty"`
	OperationID string `json:"operationId"`
	Status      string `json:"status"`
	Note        string `json:"note,omitempty"`
}

type RestoreBody struct {
	TargetClusterID string `json:"targetClusterId" binding:"required"`
	Namespace       string `json:"namespace" binding:"required"`
	PodName         string `json:"podName" binding:"required"`
	TargetNodeName  string `json:"targetNodeName"`
	// Optional post-restore inference/health probe URL.
	ProbeEndpoint string `json:"probeEndpoint"`
	// Optional explicit target facts for preflight; defaults to snapshot metadata in fake demos.
	GPUProduct    string `json:"gpuProduct"`
	DriverVersion string `json:"driverVersion"`
	Architecture  string `json:"architecture"`
	ImageDigest   string `json:"imageDigest"`
}

type CompatibilityBody struct {
	TargetClusterID string `json:"targetClusterId" binding:"required"`
	TargetNodeName  string `json:"targetNodeName"`
	GPUProduct      string `json:"gpuProduct"`
	DriverVersion   string `json:"driverVersion"`
	Architecture    string `json:"architecture"`
	ImageDigest     string `json:"imageDigest"`
}

type SnapshotView struct {
	ID                  string                     `json:"id"`
	Name                string                     `json:"name"`
	Namespace           string                     `json:"namespace"`
	ClusterID           string                     `json:"clusterId"`
	Status              string                     `json:"status"`
	Provider            string                     `json:"provider"`
	ProviderVersion     string                     `json:"providerVersion"`
	ProviderSnapshotRef domain.ProviderSnapshotRef `json:"providerSnapshotRef"`
	ProviderContentName string                     `json:"providerContentName"`
	Source              SourceBody                 `json:"source"`
	SourcePodUID        string                     `json:"sourcePodUid"`
	Hardware            HardwareView               `json:"hardware"`
	Container           ContainerView              `json:"container"`
	Artifact            domain.ArtifactRef         `json:"artifact"`
	SizeBytes           *int64                     `json:"sizeBytes"`
	Labels              map[string]string          `json:"labels"`
	CreatedAt           time.Time                  `json:"createdAt"`
	ReadyAt             *time.Time                 `json:"readyAt"`
	Simulated           bool                       `json:"simulated"`
}

type HardwareView struct {
	GPUProduct    string `json:"gpuProduct"`
	GPUCount      int    `json:"gpuCount"`
	DriverVersion string `json:"driverVersion"`
	Architecture  string `json:"architecture"`
	KernelVersion string `json:"kernelVersion"`
}

type ContainerView struct {
	ImageDigest         string `json:"imageDigest"`
	SourceContainerName string `json:"sourceContainerName"`
}

type OperationView struct {
	ID             string     `json:"id"`
	Type           string     `json:"type"`
	SnapshotID     string     `json:"snapshotId"`
	ClusterID      string     `json:"clusterId"`
	Namespace      string     `json:"namespace"`
	Status         string     `json:"status"`
	RequestedBy    string     `json:"requestedBy"`
	RequestID      string     `json:"requestId"`
	IdempotencyKey string     `json:"idempotencyKey,omitempty"`
	TargetPodName  *string    `json:"targetPodName,omitempty"`
	TargetNodeName *string    `json:"targetNodeName,omitempty"`
	ErrorCode      *string    `json:"errorCode,omitempty"`
	ErrorMessage   *string    `json:"errorMessage,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
	Simulated      bool       `json:"simulated"`
}

type ProviderView struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Note    string `json:"note"`
}

func FromSnapshot(s *domain.Snapshot) SnapshotView {
	return SnapshotView{
		ID: s.ID, Name: s.Name, Namespace: s.Namespace, ClusterID: s.ClusterID,
		Status: string(s.Status), Provider: s.Provider, ProviderVersion: s.ProviderVersion,
		ProviderSnapshotRef: s.ProviderSnapshotRef, ProviderContentName: s.ProviderContentName,
		Source:       SourceBody{PodName: s.SourcePodName, ContainerName: s.SourceContainerName},
		SourcePodUID: s.SourcePodUID,
		Hardware: HardwareView{
			GPUProduct: s.GPUProduct, GPUCount: s.GPUCount, DriverVersion: s.DriverVersion,
			Architecture: s.Architecture, KernelVersion: s.KernelVersion,
		},
		Container: ContainerView{ImageDigest: s.ImageDigest, SourceContainerName: s.SourceContainerName},
		Artifact:  s.Artifact, SizeBytes: s.SizeBytes, Labels: s.Labels,
		CreatedAt: s.CreatedAt, ReadyAt: s.ReadyAt, Simulated: s.Provider == "fake",
	}
}

func FromOperation(op *domain.Operation) OperationView {
	return OperationView{
		ID: op.ID, Type: string(op.Type), SnapshotID: op.SnapshotID, ClusterID: op.ClusterID,
		Namespace: op.Namespace, Status: string(op.Status), RequestedBy: op.RequestedBy,
		RequestID: op.RequestID, IdempotencyKey: op.IdempotencyKey,
		TargetPodName: op.TargetPodName, TargetNodeName: op.TargetNodeName,
		ErrorCode: op.ErrorCode, ErrorMessage: op.ErrorMessage,
		CreatedAt: op.CreatedAt, UpdatedAt: op.UpdatedAt, FinishedAt: op.FinishedAt,
		Simulated: true,
	}
}
