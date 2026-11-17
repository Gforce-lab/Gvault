package fake_test

import (
	"context"
	"testing"
	"time"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider"
	"github.com/gvault/gvault/internal/provider/fake"
)

func TestCaptureTransitionsToReady(t *testing.T) {
	p := fake.New(20 * time.Millisecond)
	ctx := context.Background()
	ref, err := p.RequestCapture(ctx, provider.CaptureRequest{
		SnapshotID:          "snap_1",
		Name:                "demo",
		Namespace:           "inference",
		SourcePodName:       "pod-a",
		SourceContainerName: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		st, err := p.CaptureStatus(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}
		if st.Phase == domain.SnapshotReady {
			if st.GPUProduct == "" || st.ImageDigest == "" {
				t.Fatalf("expected discovered metadata, got %+v", st)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out in phase %s", st.Phase)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCompatibilityFailClosed(t *testing.T) {
	p := fake.New(time.Millisecond)
	snap := domain.SnapshotInfo{
		Status:        domain.SnapshotReady,
		GPUProduct:    "NVIDIA-Fake-A100-80GB",
		DriverVersion: "550.00.fake",
		Architecture:  "amd64",
		ImageDigest:   "sha256:abc",
		ClusterID:     "c1",
	}
	res := p.EvaluateCompatibility(context.Background(), snap, domain.TargetInfo{
		ClusterID:     "c1",
		GPUProduct:    "NVIDIA-H100",
		DriverVersion: "550.00.fake",
		Architecture:  "amd64",
		ImageDigest:   "sha256:abc",
	})
	if res.Eligible {
		t.Fatal("expected ineligible on GPU mismatch")
	}
	found := false
	for _, r := range res.Reasons {
		if r.Code == domain.CodeGPUModelMismatch {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected GPU_MODEL_MISMATCH, got %+v", res.Reasons)
	}
}

func TestCompatibilityMissingMetadata(t *testing.T) {
	p := fake.New(time.Millisecond)
	res := p.EvaluateCompatibility(context.Background(), domain.SnapshotInfo{
		Status: domain.SnapshotReady,
	}, domain.TargetInfo{})
	if res.Eligible {
		t.Fatal("expected ineligible when metadata missing")
	}
}
