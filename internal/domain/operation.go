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
}
