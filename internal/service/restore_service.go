package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/probe"
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
	if err := validateRestoreProbe(req); err != nil {
		return nil, err
	}
	if req.InferenceProbe != nil {
		spec := *req.InferenceProbe
		req.InferenceProbe = &spec // async work owns its verification settings
	}
	if s.provider.Name() == "nvidia-snapshot" && req.RestorePodSpec == nil {
		return nil, domain.NewAppError(domain.CodeValidation, "restorePodSpec is required for NVIDIA restores")
	}
	if req.RestorePodSpec != nil {
		raw, err := json.Marshal(req.RestorePodSpec)
		if err != nil {
			return nil, domain.NewAppError(domain.CodeValidation, "restorePodSpec must be JSON serializable")
		}
		var spec map[string]any
		_ = json.Unmarshal(raw, &spec)
		req.RestorePodSpec = spec
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

	// Only the simulated harness may use source facts as destination defaults.
	if s.provider.Name() == "fake" {
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
			"probeEndpoint": req.ProbeEndpoint, "inferenceProbe": req.InferenceProbe, "simulated": s.provider.Name() == "fake",
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
	ctx, cancel := context.WithTimeout(context.Background(), s.restoreTimeout)
	defer cancel()
	// Persistence must remain possible after the execution deadline expires.
	persistCtx := context.Background()
	ref, err := s.provider.RequestRestore(ctx, provider.RestoreRequest{
		RestorePodSpec: req.RestorePodSpec,
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
		if ctx.Err() != nil {
			err = domain.NewAppError(domain.CodeRestoreTimeout, "restore submission deadline exceeded")
		}
		s.failOp(persistCtx, opID, snap.ID, err)
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

	result, err := s.awaitRestore(ctx, ref, req)
	if result != nil {
		_ = s.audits.Insert(persistCtx, nil, postgres.AuditEvent{
			Actor: "system", Action: "snapshot.restore.probe", ResourceType: "snapshot",
			ResourceID: snap.ID, Decision: map[bool]string{true: "passed", false: "failed"}[result.OK],
			RequestID: opID, Attributes: map[string]any{
				"ok": result.OK, "message": result.Message, "endpoint": result.Endpoint,
				"type": result.Type, "simulated": result.Simulated || s.provider.Name() == "fake",
				"protocol": result.Protocol,
			},
		})
	}
	if err != nil {
		if s.metrics != nil {
			outcome := "restore_failed"
			if result != nil {
				outcome = "restore_probe_failed"
			}
			s.metrics.IncOperation(s.provider.Name(), outcome)
		}
		s.failOp(persistCtx, opID, snap.ID, err)
		return
	}
	s.succeedOp(persistCtx, opID)
	if s.metrics != nil {
		s.metrics.IncOperation(s.provider.Name(), "restore_succeeded")
		s.metrics.ObserveRestoreSeconds(time.Since(started).Seconds())
	}
	s.log.Info("restore succeeded", "operationId", opID, "pod", ref.Name, "simulated", s.provider.Name() == "fake")
}

// awaitRestore keeps the operation RUNNING through provider and application checks.
// It has no persistence side effects, so timeouts and ordering can be tested without a DB.
func (s *SnapshotService) awaitRestore(ctx context.Context, ref provider.ProviderRef, req domain.RestoreRequest) (*probe.Result, error) {
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, domain.NewAppError(domain.CodeRestoreTimeout, "timed out waiting for Snapshot completion and Pod readiness")
		case <-ticker.C:
		}
		st, err := s.provider.RestoreStatus(ctx, ref)
		if ctx.Err() != nil {
			return nil, domain.NewAppError(domain.CodeRestoreTimeout, "timed out waiting for Snapshot completion and Pod readiness")
		}
		if err != nil {
			return nil, err
		}
		switch st.Phase {
		case domain.OperationFailed:
			return nil, domain.NewAppError(domain.CodeProvider, st.Message)
		case domain.OperationSucceeded:
			result := s.checkRestoredWorker(ctx, st, req)
			if result != nil && !result.OK {
				return result, domain.NewAppError(domain.CodeProbeFailed, "post-restore probe failed: "+result.Message)
			}
			if result != nil && s.provider.Name() != "fake" {
				// Do not accept a probe of a replaced Pod or a worker that lost readiness.
				current, err := s.provider.RestoreStatus(ctx, ref)
				if err != nil || current.Phase != domain.OperationSucceeded || current.PodIP != st.PodIP || ctx.Err() != nil {
					result.OK = false
					result.Message = "restored worker changed or lost readiness during application verification"
					return result, domain.NewAppError(domain.CodeProbeFailed, result.Message)
				}
			}
			return result, nil
		}
	}
}

func (s *SnapshotService) checkRestoredWorker(ctx context.Context, state provider.RestoreState, req domain.RestoreRequest) *probe.Result {
	if spec := req.InferenceProbe; spec != nil {
		endpoint, err := probe.InferenceEndpointForProtocol(state.PodIP, spec.Port, spec.Protocol)
		if err != nil {
			return &probe.Result{Type: "inference", Message: err.Error()}
		}
		timeout := spec.TimeoutSeconds
		if timeout == 0 {
			timeout = 60
		}
		probeCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		result := (probe.InferenceProber{Protocol: spec.Protocol}).Check(probeCtx, endpoint, spec.Model)
		return &result
	}
	if req.ProbeEndpoint == "" && s.provider.Name() != "fake" {
		return nil // provider + Pod readiness only; no inference claim
	}
	if s.prober == nil {
		return &probe.Result{Type: "http", Message: "application prober is not configured"}
	}
	for {
		result := s.prober.Check(ctx, defaultString(req.ProbeEndpoint, "http://fake-restored/health"))
		if result.Simulated && s.provider.Name() != "fake" {
			result.OK = false
			result.Message = "simulated probe cannot verify a live restore"
			return &result
		}
		if result.OK && ctx.Err() == nil {
			return &result
		}
		select {
		case <-ctx.Done():
			result.OK = false
			result.Message = "application readiness deadline exceeded: " + result.Message
			return &result
		case <-time.After(s.poll):
		}
	}
}

func validateRestoreProbe(req domain.RestoreRequest) error {
	if spec := req.InferenceProbe; spec != nil {
		if spec.Protocol != "" && spec.Protocol != "openai" && spec.Protocol != "snapshot-vllm" {
			return domain.NewAppError(domain.CodeValidation, "inferenceProbe.protocol must be openai or snapshot-vllm")
		}
		if req.ProbeEndpoint != "" {
			return domain.NewAppError(domain.CodeValidation, "use either inferenceProbe or probeEndpoint, not both")
		}
		if (spec.Protocol != "snapshot-vllm" && strings.TrimSpace(spec.Model) == "") || len(spec.Model) > 256 {
			return domain.NewAppError(domain.CodeValidation, "inferenceProbe.model must contain 1 to 256 characters")
		}
		if spec.Port < 0 || spec.Port > 65535 {
			return domain.NewAppError(domain.CodeValidation, "inferenceProbe.port must be between 1 and 65535 (or 0 for 8000)")
		}
		if spec.TimeoutSeconds < 0 || spec.TimeoutSeconds > 300 {
			return domain.NewAppError(domain.CodeValidation, "inferenceProbe.timeoutSeconds must be between 1 and 300 (or 0 for 60)")
		}
	}
	return nil
}

func toSnapshotInfo(s *domain.Snapshot) domain.SnapshotInfo {
	return domain.SnapshotInfo{
		SourceContainerName: s.SourceContainerName,
		ID:                  s.ID,
		Status:              s.Status,
		ClusterID:           s.ClusterID,
		Namespace:           s.Namespace,
		GPUProduct:          s.GPUProduct,
		GPUCount:            s.GPUCount,
		DriverVersion:       s.DriverVersion,
		Architecture:        s.Architecture,
		ImageDigest:         s.ImageDigest,
		Provider:            s.Provider,
	}
}
