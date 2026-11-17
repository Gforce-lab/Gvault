package fake

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gvault/gvault/internal/compatibility"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider"
)

// Provider is a deterministic, GPU-free CheckpointProvider for local CI and demos.
// READY/SUCCEEDED here means simulated application state — never a physical GPU restore.
type Provider struct {
	mu      sync.Mutex
	step    time.Duration
	engine  *compatibility.Engine
	failCap map[string]bool
	failRes map[string]bool
	caps    map[string]*captureRec
	rests   map[string]*restoreRec
}

type captureRec struct {
	ref       provider.ProviderRef
	started   time.Time
	req       provider.CaptureRequest
	failed    bool
	sizeBytes int64
}

type restoreRec struct {
	ref     provider.ProviderRef
	started time.Time
	req     provider.RestoreRequest
	failed  bool
}

// New creates a FakeProvider. step controls simulated async transitions.
func New(step time.Duration) *Provider {
	if step <= 0 {
		step = 200 * time.Millisecond
	}
	return &Provider{
		step:    step,
		engine:  compatibility.New(),
		failCap: map[string]bool{},
		failRes: map[string]bool{},
		caps:    map[string]*captureRec{},
		rests:   map[string]*restoreRec{},
	}
}

func (p *Provider) Name() string { return "fake" }

// FailNextCapture forces the next capture for snapshotID to fail (test helper).
func (p *Provider) FailNextCapture(snapshotID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failCap[snapshotID] = true
}

// FailNextRestore forces the next restore for snapshotID to fail (test helper).
func (p *Provider) FailNextRestore(snapshotID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failRes[snapshotID] = true
}

func (p *Provider) CheckPrerequisites(ctx context.Context, clusterID string) error {
	_ = ctx
	if clusterID == "" {
		return domain.NewAppError(domain.CodeValidation, "clusterID is required")
	}
	return nil
}

func (p *Provider) RequestCapture(ctx context.Context, req provider.CaptureRequest) (provider.ProviderRef, error) {
	_ = ctx
	if req.SnapshotID == "" || req.Name == "" || req.Namespace == "" || req.SourcePodName == "" {
		return provider.ProviderRef{}, domain.NewAppError(domain.CodeValidation, "incomplete capture request")
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	ref := provider.ProviderRef{
		Provider:    p.Name(),
		Kind:        "FakePodSnapshot",
		Namespace:   req.Namespace,
		Name:        fmt.Sprintf("fake-%s", req.Name),
		UID:         domain.NewID("fuid"),
		ContentName: fmt.Sprintf("fake-content-%s", req.SnapshotID),
	}
	rec := &captureRec{
		ref:       ref,
		started:   time.Now().UTC(),
		req:       req,
		sizeBytes: 1024 * 1024,
	}
	if p.failCap[req.SnapshotID] {
		rec.failed = true
		delete(p.failCap, req.SnapshotID)
	}
	p.caps[ref.UID] = rec
	return ref, nil
}

func (p *Provider) CaptureStatus(ctx context.Context, ref provider.ProviderRef) (provider.CaptureState, error) {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.caps[ref.UID]
	if !ok {
		return provider.CaptureState{}, domain.NewAppError(domain.CodeNotFound, "fake capture not found")
	}
	elapsed := time.Since(rec.started)
	state := provider.CaptureState{
		ContentName:   rec.ref.ContentName,
		SourcePodUID:  "fake-pod-uid-" + rec.req.SourcePodName,
		ImageDigest:   defaultString(rec.req.ImageDigest, "sha256:fake-image-digest"),
		GPUProduct:    defaultString(rec.req.GPUProduct, "NVIDIA-Fake-A100-80GB"),
		GPUCount:      1,
		DriverVersion: defaultString(rec.req.DriverVersion, "550.00.fake"),
		Architecture:  defaultString(rec.req.Architecture, "amd64"),
		KernelVersion: "6.1.0-fake",
		SizeBytes:     &rec.sizeBytes,
		Artifact: domain.ArtifactRef{
			Backend: "fake",
			StoreID: "fake-store",
			Handle:  rec.ref.ContentName,
		},
	}
	if rec.failed {
		state.Phase = domain.SnapshotFailed
		state.Message = "simulated capture failure"
		return state, nil
	}
	switch {
	case elapsed < p.step:
		state.Phase = domain.SnapshotPending
		state.Message = "queued (fake)"
	case elapsed < 2*p.step:
		state.Phase = domain.SnapshotCapturing
		state.Message = "capturing (fake) — not a real GPU checkpoint"
	default:
		state.Phase = domain.SnapshotReady
		state.Message = "ready (fake) — simulated only"
	}
	return state, nil
}

func (p *Provider) EvaluateCompatibility(_ context.Context, snapshot domain.SnapshotInfo, target domain.TargetInfo) domain.CompatibilityResult {
	return p.engine.Evaluate(snapshot, target)
}

func (p *Provider) RequestRestore(ctx context.Context, req provider.RestoreRequest) (provider.ProviderRef, error) {
	_ = ctx
	if req.SnapshotID == "" || req.PodName == "" || req.Namespace == "" {
		return provider.ProviderRef{}, domain.NewAppError(domain.CodeValidation, "incomplete restore request")
	}
	compat := p.EvaluateCompatibility(ctx, req.Snapshot, domain.TargetInfo{
		ClusterID:     req.ClusterID,
		NodeName:      req.TargetNodeName,
		Namespace:     req.Namespace,
		GPUProduct:    req.Snapshot.GPUProduct,
		DriverVersion: req.Snapshot.DriverVersion,
		Architecture:  req.Snapshot.Architecture,
		ImageDigest:   req.Snapshot.ImageDigest,
		GPUCount:      req.Snapshot.GPUCount,
	})
	// RequestRestore itself does not re-check target hardware beyond snapshot readiness;
	// callers must run EvaluateCompatibility with real target facts first.
	if req.Snapshot.Status != domain.SnapshotReady {
		return provider.ProviderRef{}, domain.NewAppError(domain.CodeSnapshotNotReady, "snapshot is not READY")
	}
	_ = compat

	p.mu.Lock()
	defer p.mu.Unlock()
	ref := provider.ProviderRef{
		Provider:  p.Name(),
		Kind:      "FakeRestore",
		Namespace: req.Namespace,
		Name:      req.PodName,
		UID:       domain.NewID("fruid"),
	}
	rec := &restoreRec{ref: ref, started: time.Now().UTC(), req: req}
	if p.failRes[req.SnapshotID] {
		rec.failed = true
		delete(p.failRes, req.SnapshotID)
	}
	p.rests[ref.UID] = rec
	return ref, nil
}

func (p *Provider) RestoreStatus(ctx context.Context, ref provider.ProviderRef) (provider.RestoreState, error) {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.rests[ref.UID]
	if !ok {
		return provider.RestoreState{}, domain.NewAppError(domain.CodeNotFound, "fake restore not found")
	}
	elapsed := time.Since(rec.started)
	state := provider.RestoreState{
		PodName: rec.req.PodName,
		Node:    defaultString(rec.req.TargetNodeName, "fake-node-a"),
	}
	if rec.failed {
		state.Phase = domain.OperationFailed
		state.Message = "simulated restore failure"
		return state, nil
	}
	switch {
	case elapsed < p.step:
		state.Phase = domain.OperationPending
		state.Message = "queued (fake)"
	case elapsed < 2*p.step:
		state.Phase = domain.OperationRunning
		state.Message = "restoring (fake) — not a real GPU restore"
	default:
		state.Phase = domain.OperationSucceeded
		state.Message = "succeeded (fake) — simulated only"
	}
	return state, nil
}

func (p *Provider) RequestDelete(ctx context.Context, ref provider.ProviderRef) error {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.caps, ref.UID)
	delete(p.rests, ref.UID)
	return nil
}

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
