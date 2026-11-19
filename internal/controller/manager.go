package controller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gvault/gvault/internal/provider"
)

// ManagerConfig controls optional leader-aware reconcile loops.
// Full controller-runtime + envtest remains a post-CI upgrade; this manager
// matches the intended ops shape over MemoryStore + CheckpointProvider.
type ManagerConfig struct {
	LeaderElection    bool
	LeaderElectionID  string
	ReconcileInterval time.Duration
	EnableLeaderElect bool // alias clarity for helm values
}

// Manager runs periodic reconciliation against the in-memory CRD store.
type Manager struct {
	Reconciler *Reconciler
	Store      *MemoryStore
	Log        *slog.Logger
	Cfg        ManagerConfig

	mu      sync.Mutex
	leading bool
	cancel  context.CancelFunc
}

func NewManager(store *MemoryStore, p provider.CheckpointProvider, log *slog.Logger, cfg ManagerConfig) *Manager {
	if cfg.ReconcileInterval <= 0 {
		cfg.ReconcileInterval = 2 * time.Second
	}
	if cfg.LeaderElectionID == "" {
		cfg.LeaderElectionID = "gvault-controller"
	}
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		Reconciler: NewReconciler(store, p),
		Store:      store,
		Log:        log,
		Cfg:        cfg,
		leading:    !cfg.LeaderElection && !cfg.EnableLeaderElect,
	}
}

// Start begins the reconcile loop. With leader election enabled, only the
// elected instance reconciles (single-process demo elects itself immediately).
func (m *Manager) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)
	if m.Cfg.LeaderElection || m.Cfg.EnableLeaderElect {
		m.mu.Lock()
		m.leading = true // single-process: self-elect; multi-pod needs lease API later
		m.mu.Unlock()
		m.Log.Info("controller manager leader elected (local)", "id", m.Cfg.LeaderElectionID)
	}
	go m.loop(ctx)
}

func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Manager) IsLeading() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.leading
}

func (m *Manager) loop(ctx context.Context) {
	t := time.NewTicker(m.Cfg.ReconcileInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !m.IsLeading() {
				continue
			}
			m.reconcileAll(ctx)
		}
	}
}

func (m *Manager) reconcileAll(ctx context.Context) {
	for _, snap := range m.Store.ListSnapshots() {
		ns, name := snap.Metadata.Namespace, snap.Metadata.Name
		if _, err := m.Reconciler.ReconcileSnapshot(ctx, ns, name); err != nil {
			m.Log.Warn("reconcile snapshot", "namespace", ns, "name", name, "error", err)
		}
	}
}
