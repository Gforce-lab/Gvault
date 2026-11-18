package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gvault/gvault/internal/api/dto"
	"github.com/gvault/gvault/internal/api/middleware"
	"github.com/gvault/gvault/internal/controller"
	"github.com/gvault/gvault/internal/domain"
	"github.com/gvault/gvault/internal/provider"
	"github.com/gvault/gvault/internal/repository/postgres"
	"github.com/gvault/gvault/internal/service"
	"github.com/gvault/gvault/internal/telemetry"
	"github.com/gvault/gvault/internal/version"
)

type API struct {
	DB             *sql.DB
	Svc            *service.SnapshotService
	Provider       provider.CheckpointProvider
	Metrics        *telemetry.Metrics
	CRDStore       *controller.MemoryStore
	Reconciler     *controller.Reconciler
	StatusSync     *controller.StatusSync
	EnableS3Export bool
}

func (a *API) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "product": "Gvault", "version": version.Version})
}

func (a *API) Version(c *gin.Context) {
	c.JSON(http.StatusOK, version.Info())
}

func (a *API) Readyz(c *gin.Context) {
	if err := a.DB.PingContext(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "error": "database unavailable"})
		return
	}
	if err := a.Provider.CheckPrerequisites(c.Request.Context(), "local"); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "provider": a.Provider.Name()})
}

func (a *API) CreateSnapshot(c *gin.Context) {
	var body dto.CaptureBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	res, err := a.Svc.Capture(c.Request.Context(), domain.CaptureRequest{
		Name: body.Name, ClusterID: body.ClusterID, Namespace: body.Namespace,
		SourcePodName: body.Source.PodName, SourceContainerName: body.Source.ContainerName,
		Provider: body.Provider, Labels: body.Labels,
		ImageDigest: body.ImageDigest, GPUProduct: body.GPUProduct,
		DriverVersion: body.DriverVersion, Architecture: body.Architecture,
		IdempotencyKey: c.GetHeader(middleware.HeaderIdempotencyKey),
		RequestedBy:    c.GetString(middleware.CtxActor),
		RequestID:      c.GetString(middleware.CtxRequestID),
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, dto.AcceptedBody{
		SnapshotID: res.Snapshot.ID, OperationID: res.Operation.ID, Status: string(res.Operation.Status),
		Note: "NVIDIA Snapshot path — READY only after provider reports success on a live cluster",
	})
}

func (a *API) ListSnapshots(c *gin.Context) {
	items, err := a.Svc.List(c.Request.Context(), postgres.ListFilter{
		ClusterID:  c.Query("clusterId"),
		Namespace:  c.Query("namespace"),
		Status:     c.Query("status"),
		GPUProduct: c.Query("gpuProduct"),
		Limit:      queryInt(c, "limit", 50),
		Offset:     queryInt(c, "offset", 0),
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	out := make([]dto.SnapshotView, 0, len(items))
	for i := range items {
		out = append(out, dto.FromSnapshot(&items[i]))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (a *API) GetSnapshot(c *gin.Context) {
	s, err := a.Svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromSnapshot(s))
}

func (a *API) DeleteSnapshot(c *gin.Context) {
	a.DeleteSnapshotGuarded(c)
}

func (a *API) Compatibility(c *gin.Context) {
	var body dto.CompatibilityBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	res, err := a.Svc.CheckCompatibility(c.Request.Context(), c.Param("id"), domain.TargetInfo{
		ClusterID: body.TargetClusterID, NodeName: body.TargetNodeName,
		GPUProduct: body.GPUProduct, DriverVersion: body.DriverVersion,
		Architecture: body.Architecture, ImageDigest: body.ImageDigest,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (a *API) Restore(c *gin.Context) {
	var body dto.RestoreBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	res, err := a.Svc.Restore(c.Request.Context(), domain.RestoreRequest{
		SnapshotID: c.Param("id"), ClusterID: body.TargetClusterID, Namespace: body.Namespace,
		PodName: body.PodName, TargetNodeName: body.TargetNodeName,
		ProbeEndpoint:  body.ProbeEndpoint,
		InferenceProbe: body.InferenceProbe,
		RestorePodSpec: body.RestorePodSpec,
		IdempotencyKey: c.GetHeader(middleware.HeaderIdempotencyKey),
		RequestedBy:    c.GetString(middleware.CtxActor),
		RequestID:      c.GetString(middleware.CtxRequestID),
	}, domain.TargetInfo{
		ClusterID: body.TargetClusterID, NodeName: body.TargetNodeName,
		GPUProduct: body.GPUProduct, DriverVersion: body.DriverVersion,
		Architecture: body.Architecture, ImageDigest: body.ImageDigest,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, dto.AcceptedBody{
		SnapshotID: c.Param("id"), OperationID: res.Operation.ID, Status: string(res.Operation.Status),
		Note: "NVIDIA restore — waits for Snapshot completion, Pod readiness, and any configured application probe",
	})
}

func (a *API) GetOperation(c *gin.Context) {
	op, err := a.Svc.GetOperation(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromOperation(op))
}

func (a *API) ListProviders(c *gin.Context) {
	items := []dto.ProviderView{{
		Name: a.Provider.Name(), Healthy: true,
		Note: providerNote(a.Provider.Name()),
	}}
	c.JSON(http.StatusOK, gin.H{"items": items, "product": "Gvault"})
}

func providerNote(name string) string {
	if name == "nvidia-snapshot" {
		return "NVIDIA Snapshot v0.3.0 — Phase 1 product provider; live kube required for READY"
	}
	return "unsupported provider (Phase 1 is NVIDIA only)"
}

func writeErr(c *gin.Context, err error) {
	var ae *domain.AppError
	if errors.As(err, &ae) {
		status := http.StatusBadRequest
		switch ae.Code {
		case domain.CodeNotFound:
			status = http.StatusNotFound
		case domain.CodeUnauthorized:
			status = http.StatusUnauthorized
		case domain.CodeForbidden:
			status = http.StatusForbidden
		case domain.CodeConflict:
			status = http.StatusConflict
		case domain.CodeIncompatible, domain.CodeSnapshotNotReady,
			domain.CodeGPUModelMismatch, domain.CodeDriverMismatch,
			domain.CodeArchMismatch, domain.CodeImageDigestMismatch,
			domain.CodeMissingMetadata:
			status = http.StatusUnprocessableEntity
		case domain.CodeInternal, domain.CodeProvider:
			status = http.StatusInternalServerError
		}
		c.JSON(status, dto.ErrorBody{
			Code: string(ae.Code), Message: ae.Message, Details: ae.Details,
			RequestID: c.GetString(middleware.CtxRequestID),
		})
		return
	}
	c.JSON(http.StatusInternalServerError, dto.ErrorBody{
		Code: string(domain.CodeInternal), Message: "internal error",
		RequestID: c.GetString(middleware.CtxRequestID),
	})
}

func queryInt(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	var n int
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
