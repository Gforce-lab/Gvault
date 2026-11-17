package controller

import (
	"sync"

	v1alpha1 "github.com/gvault/gvault/api/v1alpha1"
)

// MemoryStore is a GPU-free stand-in for the Kubernetes API used by the gvault reconciler in tests and local demos.
type MemoryStore struct {
	mu        sync.RWMutex
	snapshots map[string]*v1alpha1.VaultSnapshot // ns/name
	restores  map[string]*v1alpha1.VaultRestore
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		snapshots: map[string]*v1alpha1.VaultSnapshot{},
		restores:  map[string]*v1alpha1.VaultRestore{},
	}
}

func key(ns, name string) string { return ns + "/" + name }

func (s *MemoryStore) PutSnapshot(obj *v1alpha1.VaultSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *obj
	s.snapshots[key(obj.Metadata.Namespace, obj.Metadata.Name)] = &cp
}

func (s *MemoryStore) GetSnapshot(ns, name string) (*v1alpha1.VaultSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, ok := s.snapshots[key(ns, name)]
	if !ok {
		return nil, false
	}
	cp := *obj
	return &cp, true
}

func (s *MemoryStore) ListSnapshots() []v1alpha1.VaultSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]v1alpha1.VaultSnapshot, 0, len(s.snapshots))
	for _, v := range s.snapshots {
		out = append(out, *v)
	}
	return out
}

func (s *MemoryStore) DeleteSnapshot(ns, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, key(ns, name))
}

func (s *MemoryStore) PutRestore(obj *v1alpha1.VaultRestore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *obj
	s.restores[key(obj.Metadata.Namespace, obj.Metadata.Name)] = &cp
}

func (s *MemoryStore) GetRestore(ns, name string) (*v1alpha1.VaultRestore, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, ok := s.restores[key(ns, name)]
	if !ok {
		return nil, false
	}
	cp := *obj
	return &cp, true
}

func (s *MemoryStore) ListRestores() []v1alpha1.VaultRestore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]v1alpha1.VaultRestore, 0, len(s.restores))
	for _, v := range s.restores {
		out = append(out, *v)
	}
	return out
}
