package compatibility

import (
	"fmt"

	"github.com/gvault/gvault/internal/domain"
)

// Engine performs fail-closed preflight checks shared by Fake and NVIDIA paths.
type Engine struct{}

func New() *Engine { return &Engine{} }

func (e *Engine) Evaluate(snapshot domain.SnapshotInfo, target domain.TargetInfo) domain.CompatibilityResult {
	res := domain.CompatibilityResult{
		Eligible:       true,
		Reasons:        []domain.CompatibilityReason{},
		VerifiedFields: []string{},
		UnknownFields:  []string{},
	}

	require := func(field, snapVal, tgtVal string, code domain.ErrorCode, msg string) {
		if snapVal == "" || tgtVal == "" {
			res.Eligible = false
			res.Reasons = append(res.Reasons, domain.CompatibilityReason{
				Code:    domain.CodeMissingMetadata,
				Message: fmt.Sprintf("missing required field %s", field),
			})
			res.UnknownFields = append(res.UnknownFields, field)
			return
		}
		res.VerifiedFields = append(res.VerifiedFields, field)
		if snapVal != tgtVal {
			res.Eligible = false
			res.Reasons = append(res.Reasons, domain.CompatibilityReason{Code: code, Message: msg})
		}
	}

	if snapshot.Status != domain.SnapshotReady {
		res.Eligible = false
		res.Reasons = append(res.Reasons, domain.CompatibilityReason{
			Code:    domain.CodeSnapshotNotReady,
			Message: "snapshot is not READY",
		})
	}

	require("gpuProduct", snapshot.GPUProduct, target.GPUProduct, domain.CodeGPUModelMismatch, "source and destination GPU products differ")
	require("driverVersion", snapshot.DriverVersion, target.DriverVersion, domain.CodeDriverMismatch, "source and destination driver versions differ")
	require("architecture", snapshot.Architecture, target.Architecture, domain.CodeArchMismatch, "source and destination CPU architectures differ")
	require("imageDigest", snapshot.ImageDigest, target.ImageDigest, domain.CodeImageDigestMismatch, "source and destination image digests differ")

	if snapshot.GPUCount > 0 && target.GPUCount > 0 && snapshot.GPUCount != target.GPUCount {
		res.Eligible = false
		res.VerifiedFields = append(res.VerifiedFields, "gpuCount")
		res.Reasons = append(res.Reasons, domain.CompatibilityReason{
			Code:    domain.CodeIncompatible,
			Message: fmt.Sprintf("gpu count mismatch: snapshot=%d target=%d", snapshot.GPUCount, target.GPUCount),
		})
	}

	if snapshot.ClusterID != "" && target.ClusterID != "" && snapshot.ClusterID != target.ClusterID {
		res.Eligible = false
		res.Reasons = append(res.Reasons, domain.CompatibilityReason{
			Code:    domain.CodeIncompatible,
			Message: "cross-cluster restore is not supported in MVP",
		})
	}
	return res
}
