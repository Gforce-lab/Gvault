package service

import (
	"context"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/repository/postgres"
)

func (s *SnapshotService) ListOperations(ctx context.Context, limit int) ([]domain.Operation, error) {
	return s.ops.ListRecent(ctx, limit)
}

func (s *SnapshotService) ListAudits(ctx context.Context, limit int) ([]postgres.AuditEvent, error) {
	return s.audits.ListRecent(ctx, limit)
}

func (s *SnapshotService) SetTags(ctx context.Context, snapshotID string, tags map[string]string) (map[string]string, error) {
	if _, err := s.snaps.Get(ctx, snapshotID); err != nil {
		return nil, err
	}
	for k, v := range tags {
		if k == "" {
			return nil, domain.NewAppError(domain.CodeValidation, "tag key is required")
		}
		if err := s.tags.Upsert(ctx, snapshotID, k, v); err != nil {
			return nil, err
		}
	}
	return s.tags.List(ctx, snapshotID)
}

func (s *SnapshotService) GetTags(ctx context.Context, snapshotID string) (map[string]string, error) {
	if _, err := s.snaps.Get(ctx, snapshotID); err != nil {
		return nil, err
	}
	return s.tags.List(ctx, snapshotID)
}

func (s *SnapshotService) PromoteVersion(ctx context.Context, snapshotID, versionTag string) error {
	if versionTag == "" {
		return domain.NewAppError(domain.CodeValidation, "versionTag is required")
	}
	if _, err := s.snaps.Get(ctx, snapshotID); err != nil {
		return err
	}
	return s.snaps.SetVersionTag(ctx, snapshotID, versionTag)
}

func (s *SnapshotService) SetHold(ctx context.Context, snapshotID string, hold bool) error {
	if _, err := s.snaps.Get(ctx, snapshotID); err != nil {
		return err
	}
	return s.snaps.SetRetentionHold(ctx, snapshotID, hold)
}

// GuardedDelete refuses deletion when retention hold is set or active restores exist.
func (s *SnapshotService) GuardedDelete(ctx context.Context, id, actor, requestID, idempotencyKey string) (*domain.Operation, error) {
	hold, err := s.snaps.GetRetentionHold(ctx, id)
	if err != nil {
		return nil, err
	}
	if hold {
		return nil, domain.NewAppError(domain.CodeConflict, "snapshot has retention hold; clear hold before delete")
	}
	ops, err := s.ops.ListOpen(ctx)
	if err != nil {
		return nil, err
	}
	for _, op := range ops {
		if op.SnapshotID == id && op.Type == domain.OperationRestore {
			return nil, domain.NewAppError(domain.CodeConflict, "active restore operations block deletion")
		}
	}
	return s.Delete(ctx, id, actor, requestID, idempotencyKey)
}

// WithTagsRepo wires optional tag repository (Milestone 5).
func (s *SnapshotService) WithTagsRepo(tags *postgres.TagRepo) *SnapshotService {
	s.tags = tags
	return s
}
