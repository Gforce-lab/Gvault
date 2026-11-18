package nvidia

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/gvault/gvault/internal/compatibility"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/kube"
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
	mu      sync.Mutex
	enabled bool
	engine  *compatibility.Engine
	kube    kube.API
	caps    map[string]captureRec
	rests   map[string]restoreRec
}

type captureRec struct {
	ref provider.ProviderRef
	req provider.CaptureRequest
}

type restoreRec struct {
	ref provider.ProviderRef
	req provider.RestoreRequest
}

func New(enabled bool) *Provider {
	return &Provider{
		enabled: enabled,
		engine:  compatibility.New(),
		caps:    map[string]captureRec{},
		rests:   map[string]restoreRec{},
	}
}

// WithKubeClient wires a live (or test) Kubernetes API. Enables the real PodSnapshot path.
func (p *Provider) WithKubeClient(c kube.API) *Provider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.kube = c
	return p
}

func (p *Provider) Name() string { return "nvidia-snapshot" }

func (p *Provider) PinnedVersion() string { return PinnedRelease }

func (p *Provider) kubeLive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.kube != nil && p.kube.Live()
}

func (p *Provider) CheckPrerequisites(ctx context.Context, clusterID string) error {
	_ = ctx
	if clusterID == "" {
		return domain.NewAppError(domain.CodeValidation, "clusterID is required")
	}
	if !p.enabled {
		return domain.NewAppError(domain.CodeProvider, "nvidia provider disabled; set ENABLE_NVIDIA_PROVIDER=true after pinning "+PinnedRelease)
	}
	if !p.kubeLive() {
		return domain.NewAppError(domain.CodeProvider, "nvidia Snapshot operator/cluster client not connected; see docs/gpu-setup.md (prerequisites SKIPPED, not PASSED)")
	}
	return nil
}

func (p *Provider) RequestCapture(ctx context.Context, req provider.CaptureRequest) (provider.ProviderRef, error) {
	if err := p.CheckPrerequisites(ctx, req.ClusterID); err != nil {
		return provider.ProviderRef{}, err
	}
	if req.Namespace == "" || req.Name == "" || req.SourcePodName == "" || req.SourceContainerName == "" {
		return provider.ProviderRef{}, domain.NewAppError(domain.CodeValidation, "incomplete capture request")
	}

	manifest := BuildPodSnapshotManifest(req.Namespace, req.Name, req.SourcePodName, "", req.SourceContainerName)
	obj := &unstructured.Unstructured{Object: manifest}
	created, err := p.kube.CreatePodSnapshot(ctx, obj)
	if err != nil {
		return provider.ProviderRef{}, domain.WrapAppError(domain.CodeProvider, "create PodSnapshot failed", err)
	}

	ref := provider.ProviderRef{
		Provider:    p.Name(),
		Kind:        "PodSnapshot",
		Namespace:   created.GetNamespace(),
		Name:        created.GetName(),
		UID:         string(created.GetUID()),
		ContentName: created.GetName(),
	}
	p.mu.Lock()
	p.caps[ref.UID] = captureRec{ref: ref, req: req}
	p.mu.Unlock()
	return ref, nil
}

func (p *Provider) CaptureStatus(ctx context.Context, ref provider.ProviderRef) (provider.CaptureState, error) {
	if err := p.CheckPrerequisites(ctx, "local"); err != nil {
		return provider.CaptureState{}, err
	}
	obj, err := p.kube.GetPodSnapshot(ctx, ref.Namespace, ref.Name)
	if err != nil {
		return provider.CaptureState{}, domain.WrapAppError(domain.CodeProvider, "get PodSnapshot failed", err)
	}

	state := provider.CaptureState{
		ContentName: ref.ContentName,
		Artifact: domain.ArtifactRef{
			Backend: "nvidia-snapshot",
			StoreID: PinnedRelease,
			Handle:  ref.Name,
		},
	}

	switch {
	case kube.ConditionTrue(obj, "Ready"):
		state.Phase = domain.SnapshotReady
		state.Message = "PodSnapshot Ready (live cluster)"
		if digest, ok, _ := unstructured.NestedString(obj.Object, "status", "imageDigest"); ok {
			state.ImageDigest = digest
		}
		if gpu, ok, _ := unstructured.NestedString(obj.Object, "status", "gpuProduct"); ok {
			state.GPUProduct = gpu
		}
		if drv, ok, _ := unstructured.NestedString(obj.Object, "status", "driverVersion"); ok {
			state.DriverVersion = drv
		}
	case kube.ConditionTrue(obj, "Failed"):
		state.Phase = domain.SnapshotFailed
		state.Message = "PodSnapshot Failed"
	default:
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		switch phase {
		case "Ready", "Succeeded":
			state.Phase = domain.SnapshotReady
			state.Message = "PodSnapshot " + phase
		case "Failed":
			state.Phase = domain.SnapshotFailed
			state.Message = "PodSnapshot Failed"
		case "Capturing", "InProgress":
			state.Phase = domain.SnapshotCapturing
			state.Message = "PodSnapshot capturing"
		default:
			state.Phase = domain.SnapshotPending
			state.Message = "PodSnapshot pending"
		}
	}
	return state, nil
}

func (p *Provider) EvaluateCompatibility(ctx context.Context, snapshot domain.SnapshotInfo, target domain.TargetInfo) domain.CompatibilityResult {
	_ = ctx
	return p.engine.Evaluate(snapshot, target)
}

func (p *Provider) RequestRestore(ctx context.Context, req provider.RestoreRequest) (provider.ProviderRef, error) {
	if err := p.CheckPrerequisites(ctx, req.ClusterID); err != nil {
		return provider.ProviderRef{}, err
	}
	if req.Namespace == "" || req.PodName == "" || req.ProviderRef.Name == "" {
		return provider.ProviderRef{}, domain.NewAppError(domain.CodeValidation, "incomplete restore request")
	}
	if req.Snapshot.Status != domain.SnapshotReady {
		return provider.ProviderRef{}, domain.NewAppError(domain.CodeSnapshotNotReady, "snapshot is not READY")
	}

	obj, err := restorePod(req)
	if err != nil {
		return provider.ProviderRef{}, domain.NewAppError(domain.CodeValidation, err.Error())
	}
	created, err := p.kube.CreatePod(ctx, obj)
	if err != nil {
		return provider.ProviderRef{}, domain.WrapAppError(domain.CodeProvider, "create restore Pod failed", err)
	}
	ref := provider.ProviderRef{
		Provider:  p.Name(),
		Kind:      "Pod",
		Namespace: created.GetNamespace(),
		Name:      created.GetName(),
		UID:       string(created.GetUID()),
	}
	p.mu.Lock()
	p.rests[ref.UID] = restoreRec{ref: ref, req: req}
	p.mu.Unlock()
	return ref, nil
}

func (p *Provider) RestoreStatus(ctx context.Context, ref provider.ProviderRef) (provider.RestoreState, error) {
	if err := p.CheckPrerequisites(ctx, "local"); err != nil {
		return provider.RestoreState{}, err
	}
	obj, err := p.kube.GetPod(ctx, ref.Namespace, ref.Name)
	if err != nil {
		return provider.RestoreState{}, domain.WrapAppError(domain.CodeProvider, "get restore Pod failed", err)
	}
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
	node, _, _ := unstructured.NestedString(obj.Object, "spec", "nodeName")
	podIP, _, _ := unstructured.NestedString(obj.Object, "status", "podIP")
	state := provider.RestoreState{PodName: ref.Name, PodIP: podIP, Node: node, Phase: domain.OperationRunning}
	if ref.UID == "" || string(obj.GetUID()) != ref.UID {
		return state, domain.NewAppError(domain.CodeProvider, "restore Pod UID no longer matches the requested worker")
	}
	if obj.GetDeletionTimestamp() != nil {
		state.Phase = domain.OperationFailed
		state.Message = "restore Pod is terminating"
		return state, nil
	}
	if phase == "Failed" || phase == "Succeeded" {
		state.Phase = domain.OperationFailed
		state.Message = "restore Pod exited before serving readiness (" + phase + ")"
		return state, nil
	}
	// Snapshot v0.3.0 owns this condition. Pod Running alone only means the
	// restore proxy has started, not that checkpoint restoration has completed.
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if !ok || condition["type"] != "nvidia.com/Restored" {
			continue
		}
		if condition["status"] == "False" {
			switch condition["reason"] {
			case "RestoreFailed", "RestoreIncompatible", "RestorePartiallySucceeded":
				state.Phase = domain.OperationFailed
				state.Message = "Snapshot restore did not succeed: " + fmt.Sprint(condition["reason"])
				return state, nil
			}
		}
		if condition["status"] == "True" && phase == "Running" && kube.ConditionTrue(obj, "Ready") {
			state.Phase = domain.OperationSucceeded
			state.Message = "Snapshot restore completed and Pod is Ready"
			return state, nil
		}
	}
	state.Message = "waiting for Snapshot restore completion and Pod readiness"
	return state, nil
}

func (p *Provider) RequestDelete(ctx context.Context, ref provider.ProviderRef) error {
	if err := p.CheckPrerequisites(ctx, "local"); err != nil {
		return err
	}
	if ref.Kind == "PodSnapshot" || ref.Kind == "" {
		if err := p.kube.DeletePodSnapshot(ctx, ref.Namespace, ref.Name); err != nil {
			return domain.WrapAppError(domain.CodeProvider, "delete PodSnapshot failed", err)
		}
	}
	p.mu.Lock()
	delete(p.caps, ref.UID)
	delete(p.rests, ref.UID)
	p.mu.Unlock()
	return nil
}

// BuildPodSnapshotManifest returns the pinned upstream capture YAML shape (documentation + dry-run).
func BuildPodSnapshotManifest(namespace, name, podName, podUID, containerName string) map[string]any {
	podRef := map[string]any{
		"name":       podName,
		"containers": []any{containerName},
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
		"containers": []any{
			map[string]any{
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

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
