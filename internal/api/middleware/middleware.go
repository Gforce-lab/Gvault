package middleware

import (
	"crypto/subtle"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gvault/gvault/internal/authz"
	"github.com/gvault/gvault/internal/domain"
)

const (
	HeaderRequestID      = "X-Request-Id"
	HeaderAPIKey         = "X-API-Key"
	HeaderIdempotencyKey = "Idempotency-Key"
	CtxRequestID         = "requestId"
	CtxActor             = "actor"
	CtxAuthMethod        = "authMethod"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = domain.NewID("req")
		}
		c.Set(CtxRequestID, id)
		c.Writer.Header().Set(HeaderRequestID, id)
		c.Next()
	}
}

func AccessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"requestId", c.GetString(CtxRequestID),
			"actor", c.GetString(CtxActor),
		)
	}
}

// Auth enforces API key and/or OIDC Bearer JWT unless DevAuthBypass is enabled.
// Static UI routes are always readable so the flow dashboard can load.
func Auth(devBypass bool, apiKey string, oidc authz.OIDCConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/healthz" || path == "/readyz" || path == "/api/v1/version" || path == "/" || strings.HasPrefix(path, "/assets/") {
			c.Next()
			return
		}
		if devBypass {
			c.Set(CtxActor, "dev-bypass")
			c.Set(CtxAuthMethod, "dev-bypass")
			c.Next()
			return
		}

		authzHeader := c.GetHeader("Authorization")
		if oidc.Enabled() && strings.HasPrefix(strings.ToLower(authzHeader), "bearer ") {
			token := strings.TrimSpace(authzHeader[7:])
			sub, err := oidc.VerifyBearer(c.Request.Context(), token)
			if err != nil {
				c.AbortWithStatusJSON(401, gin.H{
					"code":      string(domain.CodeUnauthorized),
					"message":   err.Error(),
					"requestId": c.GetString(CtxRequestID),
				})
				return
			}
			c.Set(CtxActor, sub)
			c.Set(CtxAuthMethod, "oidc")
			c.Next()
			return
		}

		if apiKey != "" {
			got := c.GetHeader(HeaderAPIKey)
			if subtle.ConstantTimeCompare([]byte(got), []byte(apiKey)) == 1 {
				c.Set(CtxActor, "api-key")
				c.Set(CtxAuthMethod, "api-key")
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(401, gin.H{
			"code":      string(domain.CodeUnauthorized),
			"message":   "invalid or missing credentials",
			"requestId": c.GetString(CtxRequestID),
		})
	}
}

// RequireNamespace enforces ALLOWED_NAMESPACES when configured.
// It checks JSON body "namespace" query, or :namespace path param.
func RequireNamespace(policy *authz.NamespacePolicy) gin.HandlerFunc {
	return func(c *gin.Context) {
		if policy == nil || !policy.Enabled() {
			c.Next()
			return
		}
		ns := c.Param("namespace")
		if ns == "" {
			ns = c.Query("namespace")
		}
		if ns == "" {
			// Body may be bound later; handlers also call Authorize explicitly.
			c.Next()
			return
		}
		if err := policy.Authorize(ns); err != nil {
			ae := err.(*domain.AppError)
			c.AbortWithStatusJSON(403, gin.H{
				"code": string(ae.Code), "message": ae.Message, "requestId": c.GetString(CtxRequestID),
			})
			return
		}
		c.Next()
	}
}
