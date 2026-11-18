package authz

import (
	"strings"

	"github.com/gvault/gvault/internal/domain"
)

// NamespacePolicy restricts mutations to an allowlist when configured.
type NamespacePolicy struct {
	Allowed map[string]struct{}
}

func NewNamespacePolicy(csv string) *NamespacePolicy {
	p := &NamespacePolicy{Allowed: map[string]struct{}{}}
	for _, ns := range strings.Split(csv, ",") {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			p.Allowed[ns] = struct{}{}
		}
	}
	return p
}

func (p *NamespacePolicy) Enabled() bool {
	return p != nil && len(p.Allowed) > 0
}

func (p *NamespacePolicy) Authorize(namespace string) error {
	if !p.Enabled() {
		return nil
	}
	if namespace == "" {
		return domain.NewAppError(domain.CodeValidation, "namespace is required")
	}
	if _, ok := p.Allowed[namespace]; !ok {
		return domain.NewAppError(domain.CodeForbidden, "namespace is not allowed by policy")
	}
	return nil
}
