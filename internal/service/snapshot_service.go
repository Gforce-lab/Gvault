package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/gvault/gvault/internal/authz"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/probe"
	"github.com/gvault/gvault/internal/provider"
	"github.com/gvault/gvault/internal/repository/postgres"
	"github.com/gvault/gvault/internal/telemetry"
)

// SnapshotService owns capture/list/get/delete and async status polling.
type SnapshotService struct {
	db             *sql.DB
	snaps          *postgres.SnapshotRepo
	ops            *postgres.OperationRepo
	audits         *postgres.AuditRepo
	tags           *postgres.TagRepo
	provider       provider.CheckpointProvider
	metrics        *telemetry.Metrics
	log            *slog.Logger
	poll           time.Duration
	nsPolicy       *authz.NamespacePolicy
	prober         probe.Prober
	restoreTimeout time.Duration

	mu       sync.Mutex
	capRefs  map[string]provider.ProviderRef // operationID -> ref
	restRefs map[string]provider.ProviderRef
}

func NewSnapshotService(
	db *sql.DB,
	snaps *postgres.SnapshotRepo,
	ops *postgres.OperationRepo,
	audits *postgres.AuditRepo,
	p provider.CheckpointProvider,
	log *slog.Logger,
	poll time.Duration,
) *SnapshotService {
	if poll <= 0 {
		poll = 100 * time.Millisecond
	}
	return &SnapshotService{
		db:             db,
		snaps:          snaps,
		ops:            ops,
		audits:         audits,
		tags:           postgres.NewTagRepo(db),
		provider:       p,
		log:            log,
		poll:           poll,
		prober:         probe.HTTPProber{},
		restoreTimeout: 5 * time.Minute,
		capRefs:        map[string]provider.ProviderRef{},
		restRefs:       map[string]provider.ProviderRef{},
	}
}

// WithRestoreTimeout bounds provider execution and application verification together.
func (s *SnapshotService) WithRestoreTimeout(timeout time.Duration) *SnapshotService {
	if timeout > 0 {
		s.restoreTimeout = timeout
	}
	return s
}

// WithMetrics attaches optional Prometheus metrics (beta).
func (s *SnapshotService) WithMetrics(m *telemetry.Metrics) *SnapshotService {
	s.metrics = m
	return s
}

// WithNamespacePolicy restricts capture/restore namespaces when allowlist is set.
func (s *SnapshotService) WithNamespacePolicy(p *authz.NamespacePolicy) *SnapshotService {
	s.nsPolicy = p
	return s
}

// WithProber sets the post-restore readiness checker (HTTPProber by default).
func (s *SnapshotService) WithProber(p probe.Prober) *SnapshotService {
	if p != nil {
		s.prober = p
	}
	return s
}

func (s *SnapshotService) authorizeNS(namespace string) error {
	if s.nsPolicy == nil {
		return nil
	}
	return s.nsPolicy.Authorize(namespace)
}

type CaptureResult struct {
	Snapshot  *domain.Snapshot
	Operation *domain.Operation
}

func (s *SnapshotService) Capture(ctx context.Context, req domain.CaptureRequest) (*CaptureResult, error) {
	if err := validateCapture(req); err != nil {
		return nil, err
	}
	if err := s.authorizeNS(req.Namespace); err != nil {
		return nil, err
	}
	if req.Provider == "" {
		req.Provider = s.provider.Name()
	}
	if req.Provider != s.provider.Name() {
		return nil, domain.NewAppError(domain.CodeValidation, "provider does not match active runtime provider ("+s.provider.Name()+")")
	}

	if req.IdempotencyKey != "" {
		existing, err := s.ops.FindByIdempotency(ctx, req.ClusterID, req.Namespace, domain.OperationCapture, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			snap, err := s.snaps.Get(ctx, existing.SnapshotID)
			if err != nil {
				return nil, err
			}
			return &CaptureResult{Snapshot: snap, Operation: existing}, nil
		}
	}

	if err := s.provider.CheckPrerequisites(ctx, req.ClusterID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	snap := &domain.Snapshot{
		ID:                  domain.NewID("snap"),
		Name:                req.Name,
		Namespace:           req.Namespace,
		ClusterID:           req.ClusterID,
		Status:              domain.SnapshotPending,
		Provider:            req.Provider,
		ProviderVersion:     "fake-0.1",
		SourcePodName:       req.SourcePodName,
		SourceContainerName: req.SourceContainerName,
		ImageDigest:         req.ImageDigest,
		GPUProduct:          req.GPUProduct,
		DriverVersion:       req.DriverVersion,
		Architecture:        defaultString(req.Architecture, "amd64"),
		GPUCount:            1,
		Artifact:            domain.ArtifactRef{Backend: "fake"},
		Labels:              req.Labels,
		CreatedAt:           now,
		ResourceVersion:     1,
	}
	if snap.Labels == nil {
		snap.Labels = map[string]string{}
	}
	op := &domain.Operation{
		ID:             domain.NewID("op"),
		Type:           domain.OperationCapture,
		SnapshotID:     snap.ID,
		ClusterID:      req.ClusterID,
		Namespace:      req.Namespace,
		Status:         domain.OperationPending,
		RequestedBy:    req.RequestedBy,
		RequestID:      req.RequestID,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.snaps.Create(ctx, tx, snap); err != nil {
		return nil, wrapRepo(err)
	}
	if err := s.ops.Create(ctx, tx, op); err != nil {
		return nil, wrapRepo(err)
	}
	if err := s.audits.Insert(ctx, tx, postgres.AuditEvent{
		Actor:        req.RequestedBy,
		Action:       "snapshot.capture",
		ResourceType: "snapshot",
		ResourceID:   snap.ID,
		Decision:     "accepted",
		RequestID:    req.RequestID,
		Attributes: map[string]any{
			"operationId": op.ID,
			"provider":    req.Provider,
			"simulated":   true,
		},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	go s.runCapture(op.ID, snap.ID, req)
	return &CaptureResult{Snapshot: snap, Operation: op}, nil
}

func (s *SnapshotService) runCapture(opID, snapID string, req domain.CaptureRequest) {
	ctx := context.Background()
	ref, err := s.provider.RequestCapture(ctx, provider.CaptureRequest{
		SnapshotID:          snapID,
		Name:                req.Name,
		ClusterID:           req.ClusterID,
		Namespace:           req.Namespace,
		SourcePodName:       req.SourcePodName,
		SourceContainerName: req.SourceContainerName,
		Labels:              req.Labels,
		ImageDigest:         req.ImageDigest,
		GPUProduct:          req.GPUProduct,
		DriverVersion:       req.DriverVersion,
		Architecture:        req.Architecture,
	})
	if err != nil {
		s.failOp(ctx, opID, snapID, err)
		return
	}
	s.mu.Lock()
	s.capRefs[opID] = ref
	s.mu.Unlock()

	op, _ := s.ops.Get(ctx, opID)
	if op != nil {
		op.Status = domain.OperationRunning
		_ = s.ops.UpdateStatus(ctx, op)
	}

	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	deadline := time.Now().Add(30 * time.Second)
	for {
		<-ticker.C
		st, err := s.provider.CaptureStatus(ctx, ref)
		if err != nil {
			s.failOp(ctx, opID, snapID, err)
			return
		}
		snap, err := s.snaps.Get(ctx, snapID)
		if err != nil {
			return
		}
		snap.Status = st.Phase
		snap.SourcePodUID = st.SourcePodUID
		snap.ProviderSnapshotRef = domain.ProviderSnapshotRef{
			Kind: ref.Kind, Namespace: ref.Namespace, Name: ref.Name, UID: ref.UID,
		}
		snap.ProviderContentName = st.ContentName
		snap.ImageDigest = st.ImageDigest
		snap.GPUProduct = st.GPUProduct
		snap.GPUCount = st.GPUCount
		snap.DriverVersion = st.DriverVersion
		snap.Architecture = st.Architecture
		snap.KernelVersion = st.KernelVersion
		snap.Artifact = st.Artifact
		snap.SizeBytes = st.SizeBytes
		if st.Phase == domain.SnapshotReady {
			now := time.Now().UTC()
			snap.ReadyAt = &now
			_ = s.snaps.UpdateStatusAndMeta(ctx, snap)
			s.succeedOp(ctx, opID)
			if s.metrics != nil {
				s.metrics.IncOperation(s.provider.Name(), "capture_succeeded")
				s.metrics.ObserveCaptureSeconds(time.Since(snap.CreatedAt).Seconds())
			}
			s.log.Info("capture ready", "snapshotId", snapID, "operationId", opID, "provider", s.provider.Name())
			return
		}
		if st.Phase == domain.SnapshotFailed {
			_ = s.snaps.UpdateStatusAndMeta(ctx, snap)
			if s.metrics != nil {
				s.metrics.IncOperation(s.provider.Name(), "capture_failed")
			}
			s.failOp(ctx, opID, snapID, domain.NewAppError(domain.CodeProvider, st.Message))
			return
		}
		_ = s.snaps.UpdateStatusAndMeta(ctx, snap)
		if time.Now().After(deadline) {
			s.failOp(ctx, opID, snapID, domain.NewAppError(domain.CodeProvider, "capture timed out"))
			return
		}
	}
}

func (s *SnapshotService) Get(ctx context.Context, id string) (*domain.Snapshot, error) {
	return s.snaps.Get(ctx, id)
}

func (s *SnapshotService) List(ctx context.Context, f postgres.ListFilter) ([]domain.Snapshot, error) {
	return s.snaps.List(ctx, f)
}

func (s *SnapshotService) GetOperation(ctx context.Context, id string) (*domain.Operation, error) {
	return s.ops.Get(ctx, id)
}

func (s *SnapshotService) Delete(ctx context.Context, id, actor, requestID, idempotencyKey string) (*domain.Operation, error) {
	snap, err := s.snaps.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if snap.DeletedAt != nil || snap.Status == domain.SnapshotDeleted {
		return nil, domain.NewAppError(domain.CodeConflict, "snapshot already deleted")
	}
	if idempotencyKey != "" {
		existing, err := s.ops.FindByIdempotency(ctx, snap.ClusterID, snap.Namespace, domain.OperationDelete, idempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	now := time.Now().UTC()
	op := &domain.Operation{
		ID:             domain.NewID("op"),
		Type:           domain.OperationDelete,
		SnapshotID:     snap.ID,
		ClusterID:      snap.ClusterID,
		Namespace:      snap.Namespace,
		Status:         domain.OperationPending,
		RequestedBy:    actor,
		RequestID:      requestID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.ops.Create(ctx, tx, op); err != nil {
		return nil, wrapRepo(err)
	}
	if err := s.audits.Insert(ctx, tx, postgres.AuditEvent{
		Actor: actor, Action: "snapshot.delete", ResourceType: "snapshot",
		ResourceID: snap.ID, Decision: "accepted", RequestID: requestID,
		Attributes: map[string]any{"operationId": op.ID, "simulated": true},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	go s.runDelete(op.ID, snap)
	return op, nil
}

func (s *SnapshotService) runDelete(opID string, snap *domain.Snapshot) {
	ctx := context.Background()
	_ = s.snaps.SoftDelete(ctx, snap.ID, domain.SnapshotDeleting, nil)
	ref := provider.ProviderRef{
		Provider:  snap.Provider,
		Kind:      snap.ProviderSnapshotRef.Kind,
		Namespace: snap.ProviderSnapshotRef.Namespace,
		Name:      snap.ProviderSnapshotRef.Name,
		UID:       snap.ProviderSnapshotRef.UID,
	}
	if err := s.provider.RequestDelete(ctx, ref); err != nil {
		s.failOp(ctx, opID, snap.ID, err)
		return
	}
	now := time.Now().UTC()
	_ = s.snaps.SoftDelete(ctx, snap.ID, domain.SnapshotDeleted, &now)
	s.succeedOp(ctx, opID)
}

func (s *SnapshotService) succeedOp(ctx context.Context, opID string) {
	op, err := s.ops.Get(ctx, opID)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	op.Status = domain.OperationSucceeded
	op.FinishedAt = &now
	_ = s.ops.UpdateStatus(ctx, op)
}

func (s *SnapshotService) failOp(ctx context.Context, opID, snapID string, err error) {
	code := string(domain.CodeInternal)
	msg := err.Error()
	if ae, ok := err.(*domain.AppError); ok {
		code = string(ae.Code)
		msg = ae.Message
	}
	op, getErr := s.ops.Get(ctx, opID)
	if getErr == nil {
		now := time.Now().UTC()
		op.Status = domain.OperationFailed
		op.ErrorCode = &code
		op.ErrorMessage = &msg
		op.FinishedAt = &now
		_ = s.ops.UpdateStatus(ctx, op)
	}
	if snap, err := s.snaps.Get(ctx, snapID); err == nil && snap.Status != domain.SnapshotReady && snap.Status != domain.SnapshotDeleted {
		snap.Status = domain.SnapshotFailed
		_ = s.snaps.UpdateStatusAndMeta(ctx, snap)
	}
	s.log.Error("operation failed", "operationId", opID, "snapshotId", snapID, "error", msg)
}

func validateCapture(req domain.CaptureRequest) error {
	if req.Name == "" || req.ClusterID == "" || req.Namespace == "" || req.SourcePodName == "" || req.SourceContainerName == "" {
		return domain.NewAppError(domain.CodeValidation, "name, clusterId, namespace, source.podName, and source.containerName are required")
	}
	return nil
}

func wrapRepo(err error) error {
	if err == nil {
		return nil
	}
	// Unique violations etc. — keep generic for M1.
	return domain.WrapAppError(domain.CodeConflict, "persistence conflict", err)
}

func defaultString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
