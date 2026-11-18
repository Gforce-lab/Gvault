package authz

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gvault/gvault/internal/domain"
)

// OIDCConfig enables Bearer JWT auth when Issuer and Audience are set.
// For beta we support HS256 with a shared OIDC_CLIENT_SECRET (dev/staging).
// Production should swap in a JWKS verifier without changing the middleware contract.
type OIDCConfig struct {
	Issuer   string
	Audience string
	Secret   string // HS256 shared secret; empty disables bearer verification
}

func (c OIDCConfig) Enabled() bool {
	return c.Issuer != "" && c.Audience != "" && c.Secret != ""
}

type claims struct {
	Iss string `json:"iss"`
	Aud any    `json:"aud"`
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
	Nbf int64  `json:"nbf"`
}

// VerifyBearer validates a compact JWT (HS256) against OIDCConfig.
func (c OIDCConfig) VerifyBearer(_ context.Context, token string) (subject string, err error) {
	if !c.Enabled() {
		return "", domain.NewAppError(domain.CodeUnauthorized, "OIDC is not configured")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", domain.NewAppError(domain.CodeUnauthorized, "malformed bearer token")
	}
	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", domain.NewAppError(domain.CodeUnauthorized, "invalid token signature encoding")
	}
	mac := hmac.New(sha256.New, []byte(c.Secret))
	_, _ = mac.Write([]byte(signingInput))
	if !hmac.Equal(mac.Sum(nil), sig) {
		return "", domain.NewAppError(domain.CodeUnauthorized, "invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", domain.NewAppError(domain.CodeUnauthorized, "invalid token payload")
	}
	var cl claims
	if err := json.Unmarshal(payload, &cl); err != nil {
		return "", domain.NewAppError(domain.CodeUnauthorized, "invalid token claims")
	}
	if cl.Iss != c.Issuer {
		return "", domain.NewAppError(domain.CodeUnauthorized, "issuer mismatch")
	}
	if !audienceMatches(cl.Aud, c.Audience) {
		return "", domain.NewAppError(domain.CodeUnauthorized, "audience mismatch")
	}
	now := time.Now().Unix()
	if cl.Exp > 0 && now > cl.Exp {
		return "", domain.NewAppError(domain.CodeUnauthorized, "token expired")
	}
	if cl.Nbf > 0 && now < cl.Nbf {
		return "", domain.NewAppError(domain.CodeUnauthorized, "token not yet valid")
	}
	if cl.Sub == "" {
		return "", domain.NewAppError(domain.CodeUnauthorized, "missing subject")
	}
	return cl.Sub, nil
}

func audienceMatches(aud any, want string) bool {
	switch v := aud.(type) {
	case string:
		return v == want
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

// MintDevToken builds an HS256 JWT for local tests (not for production IdP).
func MintDevToken(cfg OIDCConfig, sub string, ttl time.Duration) (string, error) {
	if !cfg.Enabled() {
		return "", fmt.Errorf("OIDC not configured")
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now().Unix()
	body, _ := json.Marshal(map[string]any{
		"iss": cfg.Issuer,
		"aud": cfg.Audience,
		"sub": sub,
		"iat": now,
		"exp": now + int64(ttl.Seconds()),
	})
	payload := base64.RawURLEncoding.EncodeToString(body)
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(cfg.Secret))
	_, _ = mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}
