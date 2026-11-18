package export_test

import (
	"context"
	"testing"

	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/export"
)

func TestS3ExporterFailClosed(t *testing.T) {
	err := export.S3Exporter{}.Export(context.Background(), "snap", "bucket", "key")
	ae, ok := err.(*domain.AppError)
	if !ok || ae.Code != domain.CodeProvider {
		t.Fatalf("got %v", err)
	}
	err2 := export.S3Exporter{Enabled: true}.Export(context.Background(), "snap", "bucket", "key")
	if err2 == nil {
		t.Fatal("enabled but unverified must still fail closed")
	}
}
