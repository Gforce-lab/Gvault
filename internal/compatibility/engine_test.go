package compatibility_test

import (
	"testing"

	"github.com/gvault/gvault/internal/compatibility"
	"github.com/gvault/gvault/internal/domain"
)

func TestEvaluateMatch(t *testing.T) {
	e := compatibility.New()
	res := e.Evaluate(domain.SnapshotInfo{
		Status: domain.SnapshotReady, GPUProduct: "A100", DriverVersion: "550",
		Architecture: "amd64", ImageDigest: "sha256:a", ClusterID: "c1", GPUCount: 1,
	}, domain.TargetInfo{
		GPUProduct: "A100", DriverVersion: "550", Architecture: "amd64",
		ImageDigest: "sha256:a", ClusterID: "c1", GPUCount: 1,
	})
	if !res.Eligible {
		t.Fatalf("expected eligible: %+v", res)
	}
}

func TestEvaluateMismatch(t *testing.T) {
	e := compatibility.New()
	res := e.Evaluate(domain.SnapshotInfo{
		Status: domain.SnapshotReady, GPUProduct: "A100", DriverVersion: "550",
		Architecture: "amd64", ImageDigest: "sha256:a", ClusterID: "c1",
	}, domain.TargetInfo{
		GPUProduct: "H100", DriverVersion: "550", Architecture: "amd64",
		ImageDigest: "sha256:a", ClusterID: "c1",
	})
	if res.Eligible {
		t.Fatal("expected ineligible")
	}
}
