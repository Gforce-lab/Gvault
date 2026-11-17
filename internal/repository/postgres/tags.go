package postgres

import (
	"context"
	"database/sql"
)

type TagRepo struct {
	db *sql.DB
}

func NewTagRepo(db *sql.DB) *TagRepo { return &TagRepo{db: db} }

func (r *TagRepo) Upsert(ctx context.Context, snapshotID, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO snapshot_tags (snapshot_id, tag_key, tag_value)
VALUES ($1,$2,$3)
ON CONFLICT (snapshot_id, tag_key) DO UPDATE SET tag_value=EXCLUDED.tag_value`,
		snapshotID, key, value)
	return err
}

func (r *TagRepo) List(ctx context.Context, snapshotID string) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT tag_key, tag_value FROM snapshot_tags WHERE snapshot_id=$1`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (r *TagRepo) Delete(ctx context.Context, snapshotID, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM snapshot_tags WHERE snapshot_id=$1 AND tag_key=$2`, snapshotID, key)
	return err
}
