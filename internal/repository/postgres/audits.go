package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/gvault/gvault/internal/domain"
)

type AuditEvent struct {
	ID           string
	Actor        string
	Action       string
	ResourceType string
	ResourceID   string
	Decision     string
	RequestID    string
	Timestamp    time.Time
	Attributes   map[string]any
}

type AuditRepo struct {
	db *sql.DB
}

func NewAuditRepo(db *sql.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Insert(ctx context.Context, tx *sql.Tx, e AuditEvent) error {
	if e.ID == "" {
		e.ID = domain.NewID("aud")
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	if e.Attributes == nil {
		e.Attributes = map[string]any{}
	}
	raw, err := json.Marshal(e.Attributes)
	if err != nil {
		return err
	}
	var exec execer = r.db
	if tx != nil {
		exec = tx
	}
	_, err = exec.ExecContext(ctx, `
INSERT INTO audit_events (id, actor, action, resource_type, resource_id, decision, request_id, timestamp, attributes_jsonb)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		e.ID, e.Actor, e.Action, e.ResourceType, e.ResourceID, e.Decision, e.RequestID, e.Timestamp, raw,
	)
	return err
}
