package nvidia_test

import (
	"context"
	"testing"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider/nvidia"
)

func TestPrerequisitesFailClosed(t *testing.T) {
	p := nvidia.New(false)
	if err := p.CheckPrerequisites(context.Background(), "c1"); err == nil {
		t.Fatal("expected error when disabled")
	}
	p2 := nvidia.New(true)
	if err := p2.CheckPrerequisites(context.Background(), "c1"); err == nil {
		t.Fatal("expected error without kube client — must not PASS")
	}
}

func TestManifestShape(t *testing.T) {
	m := nvidia.BuildPodSnapshotManifest("inference", "snap1", "pod-a", "uid-1", "main")
	if m["apiVersion"] != nvidia.APIGroupVersion {
		t.Fatalf("apiVersion=%v", m["apiVersion"])
	}
	spec := m["spec"].(map[string]any)
	src := spec["source"].(map[string]any)
	podRef := src["podRef"].(map[string]any)
	containers := podRef["containers"].([]string)
	if containers[0] != "main" || podRef["uid"] != "uid-1" {
		t.Fatalf("podRef=%v", podRef)
	}
}

func TestRestoreContract(t *testing.T) {
	pod := nvidia.BuildRestorePodContract("snap1", "inference", "restored-1", "main", map[string]string{"kubernetes.io/hostname": "gpu-b"})
	meta := pod["metadata"].(map[string]any)
	ann := meta["annotations"].(map[string]string)
	if ann[nvidia.RestoreFromAnnotation] != "snap1" {
		t.Fatalf("annotations=%v", ann)
	}
	spec := pod["spec"].(map[string]any)
	if spec["restartPolicy"] != "Never" {
		t.Fatalf("restartPolicy=%v", spec["restartPolicy"])
	}
}

func TestCompatibilityDelegates(t *testing.T) {
	p := nvidia.New(true)
	res := p.EvaluateCompatibility(context.Background(), domain.SnapshotInfo{
		Status: domain.SnapshotReady, GPUProduct: "A", DriverVersion: "1",
		Architecture: "amd64", ImageDigest: "sha256:x", ClusterID: "c",
	}, domain.TargetInfo{
		GPUProduct: "B", DriverVersion: "1", Architecture: "amd64", ImageDigest: "sha256:x", ClusterID: "c",
	})
	if res.Eligible {
		t.Fatal("expected mismatch")
	}
}
