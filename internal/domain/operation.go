package domain

import "time"

// OperationType classifies async work.
type OperationType string

const (
	OperationCapture OperationType = "CAPTURE"
	OperationRestore OperationType = "RESTORE"
	OperationDelete  OperationType = "DELETE"
)

// OperationStatus is monotonic per attempt.
type OperationStatus string

const (
	OperationPending   OperationStatus = "PENDING"
	OperationRunning   OperationStatus = "RUNNING"
	OperationSucceeded OperationStatus = "SUCCEEDED"
	OperationFailed    OperationStatus = "FAILED"
	OperationCancelled OperationStatus = "CANCELLED"
)

// Operation tracks durable async work.
type Operation struct {
	// Provider is projected from the snapshot when operations are read.
	Provider       string
	ID             string
	Type           OperationType
	SnapshotID     string
	ClusterID      string
	Namespace      string
	Status         OperationStatus
	RequestedBy    string
	RequestID      string
	IdempotencyKey string
	TargetPodName  *string
	TargetNodeName *string
	ErrorCode      *string
	ErrorMessage   *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FinishedAt     *time.Time
}

// RestoreRequest is the API/service intent to restore a snapshot.
type RestoreRequest struct {
	SnapshotID     string
	ClusterID      string
	Namespace      string
	PodName        string
	TargetNodeName string
	IdempotencyKey string
	RequestedBy    string
	RequestID      string
	// ProbeEndpoint is an optional legacy HTTP health check URL.
	ProbeEndpoint string
	// InferenceProbe verifies generation directly against the restored Pod IP.
	InferenceProbe *InferenceProbe
	// RestorePodSpec supplies the actual workload configuration, including GPU
	// resources, commands, and model mounts. Never copied into audit events.
	RestorePodSpec map[string]any
}

// InferenceProbe configures a bounded OpenAI /v1/completions or pinned
// Snapshot vLLM /generate check.
// The request uses a fixed synthetic prompt, never customer data.
type InferenceProbe struct {
	Protocol       string `json:"protocol,omitempty"`
	Model          string `json:"model"`
	Port           int    `json:"port,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}
