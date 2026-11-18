package dto_test

import (
	"testing"

	"github.com/gvault/gvault/internal/api/dto"
	"github.com/gvault/gvault/internal/domain"
)

func TestOperationSimulationComesFromSnapshotProvider(t *testing.T) {
	for _, name := range []string{"fake", "nvidia-snapshot", ""} {
		view := dto.FromOperation(&domain.Operation{Provider: name})
		if view.Provider != name || view.Simulated != (name != "nvidia-snapshot") {
			t.Fatalf("provider=%q view=%+v", name, view)
		}
	}
}
