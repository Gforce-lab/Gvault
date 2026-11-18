package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gvault/gvault/internal/api/dto"
	"github.com/gvault/gvault/internal/api/middleware"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/export"
	"github.com/gvault/gvault/internal/provider/nvidia"
	"github.com/gvault/gvault/internal/repository/postgres"
	"github.com/gvault/gvault/internal/version"
)

func (a *API) ListOperations(c *gin.Context) {
	items, err := a.Svc.ListOperations(c.Request.Context(), queryInt(c, "limit", 50))
	if err != nil {
		writeErr(c, err)
		return
	}
	out := make([]dto.OperationView, 0, len(items))
	for i := range items {
		out = append(out, dto.FromOperation(&items[i]))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (a *API) GetTags(c *gin.Context) {
	tags, err := a.Svc.GetTags(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

func (a *API) PutTags(c *gin.Context) {
	var body struct {
		Tags map[string]string `json:"tags" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	tags, err := a.Svc.SetTags(c.Request.Context(), c.Param("id"), body.Tags)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

func (a *API) Promote(c *gin.Context) {
	var body struct {
		VersionTag string `json:"versionTag" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	if err := a.Svc.PromoteVersion(c.Request.Context(), c.Param("id"), body.VersionTag); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id"), "versionTag": body.VersionTag})
}

func (a *API) SetHold(c *gin.Context) {
	var body struct {
		Hold bool `json:"hold"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	if err := a.Svc.SetHold(c.Request.Context(), c.Param("id"), body.Hold); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id"), "retentionHold": body.Hold})
}

func (a *API) Overview(c *gin.Context) {
	snaps, err := a.Svc.List(c.Request.Context(), postgres.ListFilter{
		Limit: queryInt(c, "limit", 100),
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	ops, err := a.Svc.ListOperations(c.Request.Context(), 20)
	if err != nil {
		writeErr(c, err)
		return
	}
	ready := 0
	for _, s := range snaps {
		if s.Status == domain.SnapshotReady {
			ready++
		}
	}
	if a.Metrics != nil {
		a.Metrics.SetReady(int64(ready))
	}
	c.JSON(http.StatusOK, gin.H{
		"product":          "Gvault",
		"version":          version.Version,
		"channel":          "beta",
		"phase":            "1-nvidia-only",
		"provider":         a.Provider.Name(),
		"simulated":        a.Provider.Name() != "nvidia-snapshot",
		"snapshotCount":    len(snaps),
		"readyCount":       ready,
		"recentOperations": len(ops),
		"nvidiaPinned":     nvidia.PinnedRelease,
		"amd":              "out-of-scope",
		"storage":          "postgres-catalog-metadata-only; checkpoint-bytes-on-nvidia-rwx",
		"note":             "Phase 1 is NVIDIA GPU only. Catalog stores metadata/refs — never checkpoint bytes. See docs/storage.md and docs/scope.md.",
	})
}

func (a *API) NVIDIADryRun(c *gin.Context) {
	var body struct {
		Namespace     string `json:"namespace" binding:"required"`
		Name          string `json:"name" binding:"required"`
		PodName       string `json:"podName" binding:"required"`
		PodUID        string `json:"podUid"`
		ContainerName string `json:"containerName" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"pinnedRelease": nvidia.PinnedRelease,
		"podSnapshot":   nvidia.BuildPodSnapshotManifest(body.Namespace, body.Name, body.PodName, body.PodUID, body.ContainerName),
		"restorePod":    nvidia.BuildRestorePodContract(body.Name, body.Namespace, body.PodName+"-restored", body.ContainerName, nil),
		"note":          "Dry-run only. Live capture/restore requires NVIDIA Snapshot " + nvidia.PinnedRelease + " on a GPU cluster.",
	})
}

func (a *API) ListAudits(c *gin.Context) {
	items, err := a.Svc.ListAudits(c.Request.Context(), queryInt(c, "limit", 50))
	if err != nil {
		writeErr(c, err)
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, e := range items {
		out = append(out, gin.H{
			"id": e.ID, "actor": e.Actor, "action": e.Action,
			"resourceType": e.ResourceType, "resourceId": e.ResourceID,
			"decision": e.Decision, "requestId": e.RequestID,
			"timestamp": e.Timestamp, "attributes": e.Attributes,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (a *API) ExportS3(c *gin.Context) {
	var body struct {
		SnapshotID string `json:"snapshotId" binding:"required"`
		Bucket     string `json:"bucket" binding:"required"`
		Key        string `json:"key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	ex := export.S3Exporter{Enabled: a.EnableS3Export}
	if err := ex.Export(c.Request.Context(), body.SnapshotID, body.Bucket, body.Key); err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
}

// DeleteSnapshotGuarded uses retention/active-restore safety (Milestone 5).
func (a *API) DeleteSnapshotGuarded(c *gin.Context) {
	op, err := a.Svc.GuardedDelete(
		c.Request.Context(),
		c.Param("id"),
		c.GetString(middleware.CtxActor),
		c.GetString(middleware.CtxRequestID),
		c.GetHeader(middleware.HeaderIdempotencyKey),
	)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, dto.AcceptedBody{
		SnapshotID: c.Param("id"), OperationID: op.ID, Status: string(op.Status),
		Note: "Guarded deletion — provider lifecycle + catalog DELETED",
	})
}
