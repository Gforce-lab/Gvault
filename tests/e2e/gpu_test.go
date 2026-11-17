//go:build gpu

package e2e_test

import "testing"

// Real GPU A → GPU B restore tests require NVIDIA Snapshot v0.3.0, GPU Operator,
// RWX PVC, and a checkpoint-ready workload. Without those prerequisites this
// package must be SKIPPED — never marked PASSED with simulated data.
func TestRealGPUCrossNodeRestore(t *testing.T) {
	t.Skip("SKIPPED: real GPU cluster / NVIDIA Snapshot v0.3.0 prerequisites not present — see docs/gpu-setup.md")
}

func TestRealInferenceAfterRestore(t *testing.T) {
	t.Skip("SKIPPED: real GPU inference probe requires live restore — see docs/gpu-setup.md")
}
