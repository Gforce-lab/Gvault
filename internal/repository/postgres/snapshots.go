package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gvault/gvault/internal/domain"
)

type SnapshotRepo struct {
	db *sql.DB
}

func NewSnapshotRepo(db *sql.DB) *SnapshotRepo {
	return &SnapshotRepo{db: db}
}

func (r *SnapshotRepo) Create(ctx context.Context, tx *sql.Tx, s *domain.Snapshot) error {
	art, err := json.Marshal(s.Artifact)
	if err != nil {
		return err
	}
	labels, err := json.Marshal(s.Labels)
	if err != nil {
		return err
	}
	q := `
INSERT INTO snapshots (
  id, name, cluster_id, namespace, provider, status,
  source_pod_name, source_pod_uid, source_container_name,
  provider_snapshot_name, provider_snapshot_uid, provider_content_name, provider_version,
  model_name, model_revision, framework, framework_version, image_digest,
  gpu_product, gpu_count, driver_version, architecture, kernel_version,
  artifact_backend, artifact_ref_jsonb, size_bytes, labels_jsonb,
  created_at, ready_at, deleted_at, resource_version
) VALUES (
  $1,$2,$3,$4,$5,$6,
  $7,$8,$9,
  $10,$11,$12,$13,
  $14,$15,$16,$17,$18,
  $19,$20,$21,$22,$23,
  $24,$25,$26,$27,
  $28,$29,$30,$31
)`
	exec := r.exec(tx)
	_, err = exec.ExecContext(ctx, q,
		s.ID, s.Name, s.ClusterID, s.Namespace, s.Provider, string(s.Status),
		s.SourcePodName, s.SourcePodUID, s.SourceContainerName,
		s.ProviderSnapshotRef.Name, s.ProviderSnapshotRef.UID, s.ProviderContentName, s.ProviderVersion,
		s.ModelName, s.ModelRevision, s.Framework, s.FrameworkVersion, s.ImageDigest,
		s.GPUProduct, s.GPUCount, s.DriverVersion, s.Architecture, s.KernelVersion,
		s.Artifact.Backend, art, s.SizeBytes, labels,
		s.CreatedAt, s.ReadyAt, s.DeletedAt, s.ResourceVersion,
	)
	return err
}

func (r *SnapshotRepo) UpdateStatusAndMeta(ctx context.Context, s *domain.Snapshot) error {
	art, err := json.Marshal(s.Artifact)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
UPDATE snapshots SET
  status=$2,
  source_pod_uid=$3,
  provider_snapshot_name=$4,
  provider_snapshot_uid=$5,
  provider_content_name=$6,
  image_digest=$7,
  gpu_product=$8,
  gpu_count=$9,
  driver_version=$10,
  architecture=$11,
  kernel_version=$12,
  artifact_backend=$13,
  artifact_ref_jsonb=$14,
  size_bytes=$15,
  ready_at=$16,
  deleted_at=$17,
  resource_version=resource_version+1
WHERE id=$1`,
		s.ID, string(s.Status), s.SourcePodUID,
		s.ProviderSnapshotRef.Name, s.ProviderSnapshotRef.UID, s.ProviderContentName,
		s.ImageDigest, s.GPUProduct, s.GPUCount, s.DriverVersion, s.Architecture, s.KernelVersion,
		s.Artifact.Backend, art, s.SizeBytes, s.ReadyAt, s.DeletedAt,
	)
	return err
}

func (r *SnapshotRepo) Get(ctx context.Context, id string) (*domain.Snapshot, error) {
	row := r.db.QueryRowContext(ctx, snapshotSelect+` WHERE id=$1`, id)
	s, err := scanSnapshot(row)
	if err == sql.ErrNoRows {
		return nil, domain.NewAppError(domain.CodeNotFound, "snapshot not found")
	}
	return s, err
}

type ListFilter struct {
	ClusterID  string
	Namespace  string
	Status     string
	GPUProduct string
	Limit      int
	Offset     int
}

func (r *SnapshotRepo) List(ctx context.Context, f ListFilter) ([]domain.Snapshot, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	where := []string{"deleted_at IS NULL"}
	args := []any{}
	add := func(expr string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	if f.ClusterID != "" {
		add("cluster_id=$%d", f.ClusterID)
	}
	if f.Namespace != "" {
		add("namespace=$%d", f.Namespace)
	}
	if f.Status != "" {
		add("status=$%d", f.Status)
	}
	if f.GPUProduct != "" {
		add("gpu_product=$%d", f.GPUProduct)
	}
	args = append(args, f.Limit, f.Offset)
	q := snapshotSelect + ` WHERE ` + strings.Join(where, " AND ") +
		fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Snapshot
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

const snapshotSelect = `
SELECT id, name, cluster_id, namespace, provider, status,
  source_pod_name, source_pod_uid, source_container_name,
  provider_snapshot_name, provider_snapshot_uid, provider_content_name, provider_version,
  model_name, model_revision, framework, framework_version, image_digest,
  gpu_product, gpu_count, driver_version, architecture, kernel_version,
  artifact_backend, artifact_ref_jsonb, size_bytes, labels_jsonb,
  created_at, ready_at, deleted_at, resource_version
FROM snapshots`

type scannable interface {
	Scan(dest ...any) error
}

func scanSnapshot(row scannable) (*domain.Snapshot, error) {
	var s domain.Snapshot
	var status string
	var artRaw, labelsRaw []byte
	var readyAt, deletedAt sql.NullTime
	var sizeBytes sql.NullInt64
	err := row.Scan(
		&s.ID, &s.Name, &s.ClusterID, &s.Namespace, &s.Provider, &status,
		&s.SourcePodName, &s.SourcePodUID, &s.SourceContainerName,
		&s.ProviderSnapshotRef.Name, &s.ProviderSnapshotRef.UID, &s.ProviderContentName, &s.ProviderVersion,
		&s.ModelName, &s.ModelRevision, &s.Framework, &s.FrameworkVersion, &s.ImageDigest,
		&s.GPUProduct, &s.GPUCount, &s.DriverVersion, &s.Architecture, &s.KernelVersion,
		&s.Artifact.Backend, &artRaw, &sizeBytes, &labelsRaw,
		&s.CreatedAt, &readyAt, &deletedAt, &s.ResourceVersion,
	)
	if err != nil {
		return nil, err
	}
	s.Status = domain.SnapshotStatus(status)
	s.ProviderSnapshotRef.Kind = "FakePodSnapshot"
	if s.Provider == "nvidia-snapshot" {
		s.ProviderSnapshotRef.Kind = "PodSnapshot"
	}
	s.ProviderSnapshotRef.Namespace = s.Namespace
	if len(artRaw) > 0 {
		_ = json.Unmarshal(artRaw, &s.Artifact)
	}
	s.Labels = map[string]string{}
	if len(labelsRaw) > 0 {
		_ = json.Unmarshal(labelsRaw, &s.Labels)
	}
	if sizeBytes.Valid {
		v := sizeBytes.Int64
		s.SizeBytes = &v
	}
	if readyAt.Valid {
		t := readyAt.Time
		s.ReadyAt = &t
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		s.DeletedAt = &t
	}
	return &s, nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (r *SnapshotRepo) exec(tx *sql.Tx) execer {
	if tx != nil {
		return tx
	}
	return r.db
}

// SoftDelete marks a snapshot deleting/deleted.
func (r *SnapshotRepo) SoftDelete(ctx context.Context, id string, status domain.SnapshotStatus, deletedAt *time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE snapshots SET status=$2, deleted_at=$3, resource_version=resource_version+1 WHERE id=$1`,
		id, string(status), deletedAt)
	return err
}

func (r *SnapshotRepo) SetRetentionHold(ctx context.Context, id string, hold bool) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE snapshots SET retention_hold=$2, resource_version=resource_version+1 WHERE id=$1`, id, hold)
	return err
}

func (r *SnapshotRepo) SetVersionTag(ctx context.Context, id, tag string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE snapshots SET version_tag=$2, resource_version=resource_version+1 WHERE id=$1`, id, tag)
	return err
}

func (r *SnapshotRepo) GetRetentionHold(ctx context.Context, id string) (bool, error) {
	var hold bool
	err := r.db.QueryRowContext(ctx, `SELECT retention_hold FROM snapshots WHERE id=$1`, id).Scan(&hold)
	if err == sql.ErrNoRows {
		return false, domain.NewAppError(domain.CodeNotFound, "snapshot not found")
	}
	return hold, err
}
