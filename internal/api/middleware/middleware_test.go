package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gvault/gvault/internal/api/middleware"
	"github.com/gvault/gvault/internal/authz"
)

func TestAuthAPIKeyAndPublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Auth(false, "secret-key", authz.OIDCConfig{}))
	r.GET("/healthz", func(c *gin.Context) { c.Status(200) })
	r.GET("/api/v1/overview", func(c *gin.Context) {
		c.JSON(200, gin.H{"actor": c.GetString(middleware.CtxActor)})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != 200 {
		t.Fatalf("healthz=%d", w.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
	if w2.Code != 401 {
		t.Fatalf("expected 401 without key, got %d", w2.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	req.Header.Set(middleware.HeaderAPIKey, "secret-key")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req)
	if w3.Code != 200 {
		t.Fatalf("expected 200 with key, got %d body=%s", w3.Code, w3.Body.String())
	}
}

func TestAuthOIDCBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oidc := authz.OIDCConfig{Issuer: "https://iss.example", Audience: "gvault", Secret: "tokensecret"}
	tok, err := authz.MintDevToken(oidc, "alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(middleware.Auth(false, "", oidc))
	r.GET("/api/v1/overview", func(c *gin.Context) {
		c.JSON(200, gin.H{"actor": c.GetString(middleware.CtxActor), "method": c.GetString(middleware.CtxAuthMethod)})
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "alice") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestRequireNamespace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy := authz.NewNamespacePolicy("inference")
	r := gin.New()
	r.Use(middleware.RequireNamespace(policy))
	r.GET("/api/v1/crds/vaultsnapshots/:namespace/:name", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/crds/vaultsnapshots/other/x", nil))
	if w.Code != 403 {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/crds/vaultsnapshots/inference/x", nil))
	if w2.Code != 200 {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
}
