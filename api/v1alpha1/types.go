package v1alpha1

import "time"

// VaultSnapshotSpec is desired capture intent owned by gvault.
type VaultSnapshotSpec struct {
	Provider        string              `json:"provider"`
	Source          VaultSnapshotSource `json:"source"`
	RetentionPolicy string              `json:"retentionPolicy,omitempty"`
	Labels          map[string]string   `json:"labels,omitempty"`
	ClusterID       string              `json:"clusterId,omitempty"`
}

type VaultSnapshotSource struct {
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
}

// VaultSnapshotStatus is observed capture state (authoritative for K8s path).
type VaultSnapshotStatus struct {
	Phase                string      `json:"phase,omitempty"`
	CatalogSnapshotID    string      `json:"catalogSnapshotId,omitempty"`
	ProviderSnapshotName string      `json:"providerSnapshotName,omitempty"`
	ProviderSnapshotUID  string      `json:"providerSnapshotUid,omitempty"`
	ProviderContentName  string      `json:"providerContentName,omitempty"`
	ObservedGeneration   int64       `json:"observedGeneration,omitempty"`
	Message              string      `json:"message,omitempty"`
	Conditions           []Condition `json:"conditions,omitempty"`
	LastTransitionTime   *time.Time  `json:"lastTransitionTime,omitempty"`
	Simulated            bool        `json:"simulated,omitempty"`
}

// VaultRestoreSpec is desired restore intent.
type VaultRestoreSpec struct {
	SnapshotRef string             `json:"snapshotRef"`
	Target      VaultRestoreTarget `json:"target"`
	ClusterID   string             `json:"clusterId,omitempty"`
}

type VaultRestoreTarget struct {
	PodName      string            `json:"podName"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// VaultRestoreStatus is observed restore state.
type VaultRestoreStatus struct {
	Phase              string      `json:"phase,omitempty"`
	CatalogOperationID string      `json:"catalogOperationId,omitempty"`
	Message            string      `json:"message,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
	Simulated          bool        `json:"simulated,omitempty"`
}

type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
}

// VaultSnapshot is the in-memory / YAML-serializable CRD object.
type VaultSnapshot struct {
	APIVersion string              `json:"apiVersion"`
	Kind       string              `json:"kind"`
	Metadata   ObjectMeta          `json:"metadata"`
	Spec       VaultSnapshotSpec   `json:"spec"`
	Status     VaultSnapshotStatus `json:"status,omitempty"`
}

// VaultRestore is the in-memory / YAML-serializable CRD object.
type VaultRestore struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Metadata   ObjectMeta         `json:"metadata"`
	Spec       VaultRestoreSpec   `json:"spec"`
	Status     VaultRestoreStatus `json:"status,omitempty"`
}

type ObjectMeta struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	UID             string            `json:"uid,omitempty"`
	ResourceVersion string            `json:"resourceVersion,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	Generation      int64             `json:"generation,omitempty"`
}
