package nvidia

import "github.com/gvault/gvault/internal/kube"

// NewWithMemoryCluster returns an enabled NVIDIA provider backed by an in-memory
// Kubernetes API. ReadyOnGet is enabled so CI can exercise capture/restore
// without a physical GPU. This is still the nvidia-snapshot provider — not a
// separate "fake" product path.
func NewWithMemoryCluster() (*Provider, *kube.MemoryAPI) {
	mem := kube.NewMemoryAPI()
	mem.ReadyOnGet = true
	return New(true).WithKubeClient(mem), mem
}
