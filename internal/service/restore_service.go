package service

import (
	"context"
	"time"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider"
	"github.com/gvault/gvault/internal/repository/postgres"
)

type RestoreResult struct {
	Operation *domain.Operation
}

func (s *SnapshotService) CheckCompatibility(ctx context.Context, snapshotID string, target domain.TargetInfo) (domain.CompatibilityResult, error) {
	snap, err := s.snaps.Get(ctx, snapshotID)
	if err != nil {
		return domain.CompatibilityResult{}, err
	}
	info := toSnapshotInfo(snap)
	return s.provider.EvaluateCompatibility(ctx, info, target), nil
}

func (s *SnapshotService) Restore(ctx context.Context, req domain.RestoreRequest, target domain.TargetInfo) (*RestoreResult, error) {
	if req.SnapshotID == "" || req.ClusterID == "" || req.Namespace == "" || req.PodName == "" {
		return nil, domain.NewAppError(domain.CodeValidation, "snapshotId, clusterId, namespace, and podName are required")
	}
	if err := s.authorizeNS(req.Namespace); err != nil {
		return nil, err
	}
	snap, err := s.snaps.Get(ctx, req.SnapshotID)
	if err != nil {
		return nil, err
	}
	if snap.Status != domain.SnapshotReady {
		return nil, domain.NewAppError(domain.CodeSnapshotNotReady, "snapshot is not READY")
	}
	if snap.ClusterID != req.ClusterID || snap.Namespace != req.Namespace {
		return nil, domain.NewAppError(domain.CodeIncompatible, "restore must target the same cluster and namespace as the snapshot in MVP")
	}

	if req.IdempotencyKey != "" {
		existing, err := s.ops.FindByIdempotency(ctx, req.ClusterID, req.Namespace, domain.OperationRestore, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return &RestoreResult{Operation: existing}, nil
		}
	}

	// Fill target defaults from snapshot when caller omits hardware facts (fake demo path).
	if target.GPUProduct == "" {
		target.GPUProduct = snap.GPUProduct
	}
	if target.DriverVersion == "" {
		target.DriverVersion = snap.DriverVersion
	}
	if target.Architecture == "" {
		target.Architecture = snap.Architecture
	}
	if target.ImageDigest == "" {
		target.ImageDigest = snap.ImageDigest
	}
	if target.ClusterID == "" {
		target.ClusterID = req.ClusterID
	}
	target.Namespace = req.Namespace
	target.NodeName = req.TargetNodeName

	compat := s.provider.EvaluateCompatibility(ctx, toSnapshotInfo(snap), target)
	if !compat.Eligible {
		if s.metrics != nil {
			reason := "INCOMPATIBLE"
			if len(compat.Reasons) > 0 {
				reason = string(compat.Reasons[0].Code)
			}
			s.metrics.IncCompatReject(reason)
		}
		return nil, &domain.AppError{
			Code:    domain.CodeIncompatible,
			Message: "destination is not eligible for restore",
			Details: map[string]any{"reasons": compat.Reasons},
		}
	}

	now := time.Now().UTC()
	pod := req.PodName
	var node *string
	if req.TargetNodeName != "" {
		node = &req.TargetNodeName
	}
	op := &domain.Operation{
		ID:             domain.NewID("op"),
		Type:           domain.OperationRestore,
		SnapshotID:     snap.ID,
		ClusterID:      req.ClusterID,
		Namespace:      req.Namespace,
		Status:         domain.OperationPending,
		RequestedBy:    req.RequestedBy,
		RequestID:      req.RequestID,
		IdempotencyKey: req.IdempotencyKey,
		TargetPodName:  &pod,
		TargetNodeName: node,
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
		Actor: req.RequestedBy, Action: "snapshot.restore", ResourceType: "snapshot",
		ResourceID: snap.ID, Decision: "accepted", RequestID: req.RequestID,
		Attributes: map[string]any{
			"operationId": op.ID, "podName": req.PodName,
			"probeEndpoint": req.ProbeEndpoint, "simulated": s.provider.Name() == "fake",
		},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	go s.runRestore(op.ID, snap, req, target)
	return &RestoreResult{Operation: op}, nil
}

func (s *SnapshotService) runRestore(opID string, snap *domain.Snapshot, req domain.RestoreRequest, target domain.TargetInfo) {
	ctx := context.Background()
	ref, err := s.provider.RequestRestore(ctx, provider.RestoreRequest{
		SnapshotID:     snap.ID,
		ClusterID:      req.ClusterID,
		Namespace:      req.Namespace,
		PodName:        req.PodName,
		TargetNodeName: req.TargetNodeName,
		Snapshot:       toSnapshotInfo(snap),
		ProviderRef: provider.ProviderRef{
			Provider:    snap.Provider,
			Kind:        snap.ProviderSnapshotRef.Kind,
			Namespace:   snap.ProviderSnapshotRef.Namespace,
			Name:        snap.ProviderSnapshotRef.Name,
			UID:         snap.ProviderSnapshotRef.UID,
			ContentName: snap.ProviderContentName,
		},
	})
	if err != nil {
		s.failOp(ctx, opID, snap.ID, err)
		return
	}
	_ = target
	s.mu.Lock()
	s.restRefs[opID] = ref
	s.mu.Unlock()

	started := time.Now().UTC()
	op, _ := s.ops.Get(ctx, opID)
	if op != nil {
		started = op.CreatedAt
		op.Status = domain.OperationRunning
		_ = s.ops.UpdateStatus(ctx, op)
	}

	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	deadline := time.Now().Add(30 * time.Second)
	for {
		<-ticker.C
		st, err := s.provider.RestoreStatus(ctx, ref)
		if err != nil {
			s.failOp(ctx, opID, snap.ID, err)
			return
		}
		switch st.Phase {
		case domain.OperationSucceeded:
			if req.ProbeEndpoint != "" && s.prober != nil {
				pr := s.prober.Check(ctx, req.ProbeEndpoint)
				_ = s.audits.Insert(ctx, nil, postgres.AuditEvent{
					Actor: "system", Action: "snapshot.restore.probe", ResourceType: "snapshot",
					ResourceID: snap.ID, Decision: map[bool]string{true: "passed", false: "failed"}[pr.OK],
					RequestID: opID, Attributes: map[string]any{
						"ok": pr.OK, "message": pr.Message, "endpoint": pr.Endpoint, "simulated": pr.Simulated,
					},
				})
				if !pr.OK {
					if s.metrics != nil {
						s.metrics.IncOperation(s.provider.Name(), "restore_probe_failed")
					}
					s.failOp(ctx, opID, snap.ID, domain.NewAppError(domain.CodeProvider, "post-restore probe failed: "+pr.Message))
					return
				}
			} else if s.provider.Name() == "fake" && s.prober != nil {
				// Always run FakeProber for FakeProvider restores so the probe path is exercised.
				_ = s.prober.Check(ctx, defaultString(req.ProbeEndpoint, "http://fake-restored/health"))
			}
			s.succeedOp(ctx, opID)
			if s.metrics != nil {
				s.metrics.IncOperation(s.provider.Name(), "restore_succeeded")
				s.metrics.ObserveRestoreSeconds(time.Since(started).Seconds())
			}
			s.log.Info("restore succeeded", "operationId", opID, "pod", st.PodName, "simulated", s.provider.Name() == "fake")
			return
		case domain.OperationFailed:
			if s.metrics != nil {
				s.metrics.IncOperation(s.provider.Name(), "restore_failed")
			}
			s.failOp(ctx, opID, snap.ID, domain.NewAppError(domain.CodeProvider, st.Message))
			return
		}
		if time.Now().After(deadline) {
			s.failOp(ctx, opID, snap.ID, domain.NewAppError(domain.CodeProvider, "restore timed out"))
			return
		}
	}
}

func toSnapshotInfo(s *domain.Snapshot) domain.SnapshotInfo {
	return domain.SnapshotInfo{
		ID:            s.ID,
		Status:        s.Status,
		ClusterID:     s.ClusterID,
		Namespace:     s.Namespace,
		GPUProduct:    s.GPUProduct,
		GPUCount:      s.GPUCount,
		DriverVersion: s.DriverVersion,
		Architecture:  s.Architecture,
		ImageDigest:   s.ImageDigest,
		Provider:      s.Provider,
	}
}

