package export

import (
	"context"

	"github.com/gvault/gvault/internal/domain"
)

// S3Exporter is fail-closed until the pinned NVIDIA Snapshot release documents
// a verified object-store export path (post-beta).
type S3Exporter struct {
	Enabled bool
}

func (e S3Exporter) Export(_ context.Context, snapshotID, bucket, key string) error {
	_ = snapshotID
	_ = bucket
	_ = key
	if !e.Enabled {
		return domain.NewAppError(domain.CodeProvider, "S3/export is not enabled; deferred until upstream pin supports verified export")
	}
	return domain.NewAppError(domain.CodeProvider, "S3/export path is not verified for the pinned NVIDIA Snapshot release — refusing to advertise READY export")
}
