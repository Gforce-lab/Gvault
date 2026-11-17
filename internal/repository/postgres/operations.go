package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/gvault/gvault/internal/domain"
)

type OperationRepo struct {
	db *sql.DB
}

func NewOperationRepo(db *sql.DB) *OperationRepo {
	return &OperationRepo{db: db}
}

func (r *OperationRepo) Create(ctx context.Context, tx *sql.Tx, op *domain.Operation) error {
	q := `
INSERT INTO operations (
  id, type, snapshot_id, cluster_id, namespace, status,
  requested_by, request_id, idempotency_key, target_pod_name, target_node_name,
  error_code, error_message, created_at, updated_at, finished_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`
	var exec execer = r.db
	if tx != nil {
		exec = tx
	}
	_, err := exec.ExecContext(ctx, q,
		op.ID, string(op.Type), op.SnapshotID, op.ClusterID, op.Namespace, string(op.Status),
		op.RequestedBy, op.RequestID, op.IdempotencyKey, op.TargetPodName, op.TargetNodeName,
		op.ErrorCode, op.ErrorMessage, op.CreatedAt, op.UpdatedAt, op.FinishedAt,
	)
	return err
}

func (r *OperationRepo) Get(ctx context.Context, id string) (*domain.Operation, error) {
	row := r.db.QueryRowContext(ctx, operationSelect+` WHERE id=$1`, id)
	op, err := scanOperation(row)
	if err == sql.ErrNoRows {
		return nil, domain.NewAppError(domain.CodeNotFound, "operation not found")
	}
	return op, err
}

func (r *OperationRepo) FindByIdempotency(ctx context.Context, clusterID, namespace string, typ domain.OperationType, key string) (*domain.Operation, error) {
	if key == "" {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, operationSelect+`
WHERE cluster_id=$1 AND namespace=$2 AND type=$3 AND idempotency_key=$4`,
		clusterID, namespace, string(typ), key)
	op, err := scanOperation(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return op, err
}

func (r *OperationRepo) UpdateStatus(ctx context.Context, op *domain.Operation) error {
	op.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
UPDATE operations SET status=$2, error_code=$3, error_message=$4, updated_at=$5, finished_at=$6
WHERE id=$1`,
		op.ID, string(op.Status), op.ErrorCode, op.ErrorMessage, op.UpdatedAt, op.FinishedAt,
	)
	return err
}

func (r *OperationRepo) ListOpen(ctx context.Context) ([]domain.Operation, error) {
	rows, err := r.db.QueryContext(ctx, operationSelect+`
WHERE status IN ('PENDING','RUNNING') ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Operation
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *op)
	}
	return out, rows.Err()
}

func (r *OperationRepo) ListRecent(ctx context.Context, limit int) ([]domain.Operation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, operationSelect+` ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Operation
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *op)
	}
	return out, rows.Err()
}

const operationSelect = `
SELECT id, type, snapshot_id, cluster_id, namespace, status,
  requested_by, request_id, idempotency_key, target_pod_name, target_node_name,
  error_code, error_message, created_at, updated_at, finished_at
FROM operations`

func scanOperation(row scannable) (*domain.Operation, error) {
	var op domain.Operation
	var typ, status string
	var targetPod, targetNode, errCode, errMsg sql.NullString
	var finished sql.NullTime
	err := row.Scan(
		&op.ID, &typ, &op.SnapshotID, &op.ClusterID, &op.Namespace, &status,
		&op.RequestedBy, &op.RequestID, &op.IdempotencyKey, &targetPod, &targetNode,
		&errCode, &errMsg, &op.CreatedAt, &op.UpdatedAt, &finished,
	)
	if err != nil {
		return nil, err
	}
	op.Type = domain.OperationType(typ)
	op.Status = domain.OperationStatus(status)
	if targetPod.Valid {
		op.TargetPodName = &targetPod.String
	}
	if targetNode.Valid {
		op.TargetNodeName = &targetNode.String
	}
	if errCode.Valid {
		op.ErrorCode = &errCode.String
	}
	if errMsg.Valid {
		op.ErrorMessage = &errMsg.String
	}
	if finished.Valid {
		t := finished.Time
		op.FinishedAt = &t
	}
	return &op, nil
}
