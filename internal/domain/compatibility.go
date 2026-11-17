package domain

// TargetInfo describes a candidate restore destination.
type TargetInfo struct {
	ClusterID     string
	NodeName      string
	Namespace     string
	GPUProduct    string
	DriverVersion string
	Architecture  string
	ImageDigest   string
	GPUCount      int
}

// CompatibilityReason explains a single preflight finding.
type CompatibilityReason struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// CompatibilityResult is a fail-closed preflight filter, not a restore guarantee.
type CompatibilityResult struct {
	Eligible       bool                  `json:"eligible"`
	Reasons        []CompatibilityReason `json:"reasons"`
	VerifiedFields []string              `json:"verifiedFields"`
	UnknownFields  []string              `json:"unknownFields"`
}

// SnapshotInfo is the subset of catalog metadata used for compatibility checks.
type SnapshotInfo struct {
	ID            string
	Status        SnapshotStatus
	ClusterID     string
	Namespace     string
	GPUProduct    string
	GPUCount      int
	DriverVersion string
	Architecture  string
	ImageDigest   string
	Provider      string
}
