package dto_test

import (
	"testing"

	"github.com/gvault/gvault/internal/api/dto"
	"github.com/gvault/gvault/internal/domain"
)

func TestFromOperationKeepsProvider(t *testing.T) {
	view := dto.FromOperation(&domain.Operation{Provider: "nvidia-snapshot", ID: "op1"})
	if view.Provider != "nvidia-snapshot" || view.Simulated || view.ID != "op1" {
		t.Fatalf("%+v", view)
	}
}
