package domain_test

import (
	"errors"
	"testing"

	"github.com/gvault/gvault/internal/domain"
)

func TestAppErrorUnwrap(t *testing.T) {
	root := errors.New("root")
	err := domain.WrapAppError(domain.CodeInternal, "boom", root)
	if !errors.Is(err, root) {
		t.Fatal("expected unwrap to root")
	}
	if !domain.IsNotFound(domain.NewAppError(domain.CodeNotFound, "x")) {
		t.Fatal("expected not found")
	}
	if domain.IsNotFound(err) {
		t.Fatal("internal should not be not found")
	}
}

func TestNewID(t *testing.T) {
	a := domain.NewID("snap")
	b := domain.NewID("snap")
	if a == "" || a == b {
		t.Fatalf("expected unique ids, got %q %q", a, b)
	}
}
