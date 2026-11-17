package middleware

import (
	"crypto/subtle"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gvault/gvault/internal/domain"
)

const (
	HeaderRequestID      = "X-Request-Id"
	HeaderAPIKey         = "X-API-Key"
	HeaderIdempotencyKey = "Idempotency-Key"
	CtxRequestID         = "requestId"
	CtxActor             = "actor"
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
		)
	}
}

// Auth enforces API key unless DevAuthBypass is enabled.
// Static UI routes are always readable so the flow dashboard can load.
func Auth(devBypass bool, apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/healthz" || path == "/readyz" || path == "/api/v1/version" || path == "/" || strings.HasPrefix(path, "/assets/") {
			c.Next()
			return
		}
		if devBypass {
			c.Set(CtxActor, "dev-bypass")
			c.Next()
			return
		}
		got := c.GetHeader(HeaderAPIKey)
		if subtle.ConstantTimeCompare([]byte(got), []byte(apiKey)) != 1 {
			c.AbortWithStatusJSON(401, gin.H{
				"code":      string(domain.CodeUnauthorized),
				"message":   "invalid or missing API key",
				"requestId": c.GetString(CtxRequestID),
			})
			return
		}
		c.Set(CtxActor, "api-key")
		c.Next()
	}
}
