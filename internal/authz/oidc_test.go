package authz_test

import (
	"context"
	"testing"
	"time"

	"github.com/gvault/gvault/internal/authz"
)

func TestOIDCVerifyBearer(t *testing.T) {
	cfg := authz.OIDCConfig{
		Issuer: "https://issuer.example", Audience: "gvault", Secret: "test-secret",
	}
	tok, err := authz.MintDevToken(cfg, "user-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := cfg.VerifyBearer(context.Background(), tok)
	if err != nil || sub != "user-1" {
		t.Fatalf("sub=%q err=%v", sub, err)
	}
	if _, err := cfg.VerifyBearer(context.Background(), tok+"x"); err == nil {
		t.Fatal("expected bad sig")
	}
}
