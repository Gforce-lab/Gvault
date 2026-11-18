package nvidia

import (
	"context"
	"fmt"
	"sync"

	"github.com/gvault/gvault/internal/compatibility"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider"
)

// PinnedRelease is the verified NVIDIA Snapshot release for this adapter.
// Inspect CRDs/contracts against this tag before changing fields.
const PinnedRelease = "v0.3.0"

// RestoreFromAnnotation matches ai-dynamo/snapshot v0.3.0 podcontract.RestoreFromAnnotation.
const RestoreFromAnnotation = "nvidia.com/restore-from"

// APIGroupVersion is the upstream API group/version for PodSnapshot.
const APIGroupVersion = "nvidia.com/v1alpha1"

// Provider maps gvault intent onto NVIDIA Snapshot CRDs (feature-flagged).
// Without a live Kubernetes client it reports prerequisites unmet — never fakes GPU READY.
type Provider struct {
	mu       sync.Mutex
	enabled  bool
	engine   *compatibility.Engine
	caps     map[string]captureRec
	kubeLive bool // true only when a real client is wired (Milestone 3 cluster path)
}

type captureRec struct {
	ref provider.ProviderRef
	req provider.CaptureRequest
}

func New(enabled bool) *Provider {
	return &Provider{
		enabled: enabled,
		engine:  compatibility.New(),
		caps:    map[string]captureRec{},
	}
}

func (p *Provider) Name() string { return "nvidia-snapshot" }

func (p *Provider) PinnedVersion() string { return PinnedRelease }

func (p *Provider) CheckPrerequisites(ctx context.Context, clusterID string) error {
	_ = ctx
	if clusterID == "" {
		return domain.NewAppError(domain.CodeValidation, "clusterID is required")
	}
	if !p.enabled {
		return domain.NewAppError(domain.CodeProvider, "nvidia provider disabled; set ENABLE_NVIDIA_PROVIDER=true after pinning "+PinnedRelease)
	}
	if !p.kubeLive {
		return domain.NewAppError(domain.CodeProvider, "nvidia Snapshot operator/cluster client not connected; see docs/gpu-setup.md (prerequisites SKIPPED, not PASSED)")
	}
	return nil
}

func (p *Provider) RequestCapture(ctx context.Context, req provider.CaptureRequest) (provider.ProviderRef, error) {
	if err := p.CheckPrerequisites(ctx, req.ClusterID); err != nil {
		return provider.ProviderRef{}, err
	}
	// Real path would create nvidia.com/v1alpha1 PodSnapshot with:
	// spec.source.podRef.{name,uid,containers:[containerName]}
	return provider.ProviderRef{}, domain.NewAppError(domain.CodeProvider, "live PodSnapshot creation requires a connected kube client")
}

func (p *Provider) CaptureStatus(ctx context.Context, ref provider.ProviderRef) (provider.CaptureState, error) {
	_ = ctx
	_ = ref
	return provider.CaptureState{}, domain.NewAppError(domain.CodeProvider, "nvidia CaptureStatus requires live cluster")
}

func (p *Provider) EvaluateCompatibility(ctx context.Context, snapshot domain.SnapshotInfo, target domain.TargetInfo) domain.CompatibilityResult {
	_ = ctx
	return p.engine.Evaluate(snapshot, target)
}

func (p *Provider) RequestRestore(ctx context.Context, req provider.RestoreRequest) (provider.ProviderRef, error) {
	if err := p.CheckPrerequisites(ctx, req.ClusterID); err != nil {
		return provider.ProviderRef{}, err
	}
	return provider.ProviderRef{}, domain.NewAppError(domain.CodeProvider, "live restore Pod creation requires a connected kube client")
}

func (p *Provider) RestoreStatus(ctx context.Context, ref provider.ProviderRef) (provider.RestoreState, error) {
	_ = ctx
	_ = ref
	return provider.RestoreState{}, domain.NewAppError(domain.CodeProvider, "nvidia RestoreStatus requires live cluster")
}

func (p *Provider) RequestDelete(ctx context.Context, ref provider.ProviderRef) error {
	if err := p.CheckPrerequisites(ctx, "local"); err != nil {
		return err
	}
	_ = ref
	return domain.NewAppError(domain.CodeProvider, "nvidia delete requires live cluster / provider lifecycle")
}

// BuildPodSnapshotManifest returns the pinned upstream capture YAML shape (documentation + dry-run).
func BuildPodSnapshotManifest(namespace, name, podName, podUID, containerName string) map[string]any {
	podRef := map[string]any{
		"name":       podName,
		"containers": []string{containerName},
	}
	if podUID != "" {
		podRef["uid"] = podUID
	}
	return map[string]any{
		"apiVersion": APIGroupVersion,
		"kind":       "PodSnapshot",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]string{
				"app.kubernetes.io/managed-by": "Gvault",
				"gvault.dev/pinned-release":    PinnedRelease,
			},
		},
		"spec": map[string]any{
			"source": map[string]any{"podRef": podRef},
		},
	}
}

// BuildRestorePodContract returns the minimal restore Pod annotations/contract for v0.3.0.
// Callers must still supply a full workload template (image digest, volumes, security).
func BuildRestorePodContract(podSnapshotName, namespace, podName, containerName string, nodeSelector map[string]string) map[string]any {
	meta := map[string]any{
		"name":      podName,
		"namespace": namespace,
		"annotations": map[string]string{
			RestoreFromAnnotation: podSnapshotName,
		},
		"labels": map[string]string{
			"app.kubernetes.io/managed-by": "Gvault",
		},
	}
	spec := map[string]any{
		"restartPolicy": "Never", // required by Snapshot v0.3.0 restore proxy contract
		"containers": []map[string]any{
			{
				"name": containerName,
				// Command is replaced by NRI restore proxy on capable nodes; keep source command.
				"image": "PINNED_IMAGE_DIGEST_REQUIRED",
			},
		},
	}
	if len(nodeSelector) > 0 {
		spec["nodeSelector"] = nodeSelector
	}
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   meta,
		"spec":       spec,
		"note":       fmt.Sprintf("Pinned to NVIDIA Snapshot %s — not executed without live cluster", PinnedRelease),
	}
}
