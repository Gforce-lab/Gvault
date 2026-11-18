package nvidia_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/kube"
	"github.com/gvault/gvault/internal/provider"
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

func TestLiveCaptureRestoreWithFakeKube(t *testing.T) {
	fk := kube.NewFakeAPI()
	fk.ReadyOnGet = true
	p := nvidia.New(true).WithKubeClient(fk)
	ctx := context.Background()
	if err := p.CheckPrerequisites(ctx, "cluster-1"); err != nil {
		t.Fatal(err)
	}
	ref, err := p.RequestCapture(ctx, provider.CaptureRequest{
		SnapshotID: "snap-1", Name: "snap-1", ClusterID: "cluster-1",
		Namespace: "inference", SourcePodName: "pod-a", SourceContainerName: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Kind != "PodSnapshot" {
		t.Fatalf("kind=%s", ref.Kind)
	}
	st, err := p.CaptureStatus(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != domain.SnapshotReady {
		t.Fatalf("phase=%s msg=%s", st.Phase, st.Message)
	}
	if st.GPUProduct == "" || st.ImageDigest == "" {
		t.Fatalf("expected discovered metadata: %+v", st)
	}

	digest := "sha256:" + strings.Repeat("a", 64)
	rref, err := p.RequestRestore(ctx, provider.RestoreRequest{
		RestorePodSpec: map[string]any{"containers": []any{map[string]any{
			"name": "main", "image": "example/vllm@" + digest,
			"resources": map[string]any{"limits": map[string]any{"nvidia.com/gpu": "1"}},
		}}},
		SnapshotID: "snap-1", ClusterID: "cluster-1", Namespace: "inference",
		PodName: "restored-1", TargetNodeName: "gpu-b",
		Snapshot: domain.SnapshotInfo{
			Status: domain.SnapshotReady, ClusterID: "cluster-1", Namespace: "inference",
			GPUProduct: st.GPUProduct, DriverVersion: st.DriverVersion,
			Architecture: "amd64", ImageDigest: digest, SourceContainerName: "main",
		},
		ProviderRef: ref,
	})
	if err != nil {
		t.Fatal(err)
	}
	rst, err := p.RestoreStatus(ctx, rref)
	if err != nil {
		t.Fatal(err)
	}
	if rst.Phase != domain.OperationSucceeded {
		t.Fatalf("restore phase=%s", rst.Phase)
	}
	if err := p.RequestDelete(ctx, ref); err != nil {
		t.Fatal(err)
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
	containers := podRef["containers"].([]any)
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
