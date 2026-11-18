package api

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gvault/gvault/internal/api/handlers"
	"github.com/gvault/gvault/internal/api/middleware"
	"github.com/gvault/gvault/internal/config"
)

func NewRouter(cfg config.Config, log *slog.Logger, h *handlers.API) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(
		gin.Recovery(),
		middleware.RequestID(),
		middleware.AccessLog(log),
		middleware.Auth(cfg.DevAuthBypass, cfg.APIKey, cfg.OIDC()),
		middleware.RequireNamespace(cfg.NamespacePolicy()),
	)

	r.GET("/healthz", h.Healthz)
	r.GET("/readyz", h.Readyz)
	r.GET("/api/v1/version", h.Version)
	if h.Metrics != nil {
		r.GET("/metrics", gin.WrapH(h.Metrics.Handler()))
	}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/overview", h.Overview)
		v1.POST("/snapshots", h.CreateSnapshot)
		v1.GET("/snapshots", h.ListSnapshots)
		v1.GET("/snapshots/:id", h.GetSnapshot)
		v1.DELETE("/snapshots/:id", h.DeleteSnapshot)
		v1.POST("/snapshots/:id/compatibility", h.Compatibility)
		v1.POST("/snapshots/:id/restore", h.Restore)
		v1.GET("/snapshots/:id/tags", h.GetTags)
		v1.PUT("/snapshots/:id/tags", h.PutTags)
		v1.POST("/snapshots/:id/promote", h.Promote)
		v1.POST("/snapshots/:id/hold", h.SetHold)
		v1.GET("/operations", h.ListOperations)
		v1.GET("/operations/:id", h.GetOperation)
		v1.GET("/audits", h.ListAudits)
		v1.GET("/providers", h.ListProviders)
		v1.POST("/providers/nvidia/dry-run", h.NVIDIADryRun)
		v1.POST("/export/s3", h.ExportS3)

		// Beta CRD bridge (in-memory reconciler; GPU-free)
		v1.POST("/crds/vaultsnapshots", h.CreateVaultSnapshotCRD)
		v1.GET("/crds/vaultsnapshots", h.ListVaultSnapshotCRDs)
		v1.GET("/crds/vaultsnapshots/:namespace/:name", h.GetVaultSnapshotCRD)
		v1.POST("/crds/vaultsnapshots/:namespace/:name/reconcile", h.ReconcileVaultSnapshotCRD)
		v1.POST("/crds/vaultrestores", h.CreateVaultRestoreCRD)
	}

	mountUI(r, cfg.UIDir)
	return r
}

func mountUI(r *gin.Engine, uiDir string) {
	if uiDir == "" {
		uiDir = "web"
	}
	index := filepath.Join(uiDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		r.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"product": "Gvault",
				"message": "UI not found; API is available under /api/v1",
			})
		})
		return
	}
	r.Static("/assets", filepath.Join(uiDir, "assets"))
	r.GET("/", func(c *gin.Context) {
		c.File(index)
	})
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/metrics" {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "route not found"})
			return
		}
		c.File(index)
	})
}
