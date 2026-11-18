package authz_test

import (
	"testing"

	"github.com/gvault/gvault/internal/authz"
	"github.com/gvault/gvault/internal/domain"
)

func TestNamespacePolicy(t *testing.T) {
	p := authz.NewNamespacePolicy("inference, platform")
	if err := p.Authorize("inference"); err != nil {
		t.Fatal(err)
	}
	err := p.Authorize("other")
	if err == nil {
		t.Fatal("expected forbidden")
	}
	ae, ok := err.(*domain.AppError)
	if !ok || ae.Code != domain.CodeForbidden {
		t.Fatalf("got %v", err)
	}
	empty := authz.NewNamespacePolicy("")
	if err := empty.Authorize("any"); err != nil {
		t.Fatal("empty policy should allow all")
	}
}
