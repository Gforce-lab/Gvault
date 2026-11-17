package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	v1alpha1 "github.com/gvault/gvault/api/v1alpha1"
	"github.com/gvault/gvault/internal/controller"
	"github.com/gvault/gvault/internal/domain"
)

// Ensure CRD demo fields on API.
func (a *API) ensureCRD() {
	if a.CRDStore == nil {
		a.CRDStore = controller.NewStore()
	}
	if a.Reconciler == nil && a.Provider != nil {
		a.Reconciler = controller.NewReconciler(a.CRDStore, a.Provider)
	}
}

func (a *API) CreateVaultSnapshotCRD(c *gin.Context) {
	a.ensureCRD()
	var body struct {
		Name            string `json:"name" binding:"required"`
		Namespace       string `json:"namespace" binding:"required"`
		ClusterID       string `json:"clusterId"`
		Provider        string `json:"provider"`
		PodName         string `json:"podName" binding:"required"`
		ContainerName   string `json:"containerName" binding:"required"`
		RetentionPolicy string `json:"retentionPolicy"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	obj := &v1alpha1.VaultSnapshot{
		APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		Kind:       v1alpha1.KindSnapshot,
		Metadata: v1alpha1.ObjectMeta{
			Name: body.Name, Namespace: body.Namespace, Generation: 1,
			UID: domain.NewID("crd"),
		},
		Spec: v1alpha1.VaultSnapshotSpec{
			Provider: defaultStr(body.Provider, a.Provider.Name()), ClusterID: defaultStr(body.ClusterID, "dev-gpu-cluster"),
			RetentionPolicy: defaultStr(body.RetentionPolicy, "Retain"),
			Source:          v1alpha1.VaultSnapshotSource{PodName: body.PodName, ContainerName: body.ContainerName},
		},
	}
	a.CRDStore.PutSnapshot(obj)
	out, err := a.Reconciler.ReconcileSnapshot(c.Request.Context(), body.Namespace, body.Name)
	if err != nil {
		writeErr(c, err)
		return
	}
	// Keep reconciling briefly for fake async readiness in one request (beta convenience).
	deadline := time.Now().Add(3 * time.Second)
	for out.Status.Phase != string(domain.SnapshotReady) && out.Status.Phase != string(domain.SnapshotFailed) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
		out, err = a.Reconciler.ReconcileSnapshot(c.Request.Context(), body.Namespace, body.Name)
		if err != nil {
			writeErr(c, err)
			return
		}
	}
	if a.StatusSync != nil && out.Status.Phase == string(domain.SnapshotReady) {
		catalogID := domain.NewID("snap")
		out.Status.CatalogSnapshotID = catalogID
		a.CRDStore.PutSnapshot(out)
		_ = a.StatusSync.UpsertFromVaultSnapshot(c.Request.Context(), out, catalogID)
	}
	c.JSON(http.StatusAccepted, out)
}

func (a *API) ListVaultSnapshotCRDs(c *gin.Context) {
	a.ensureCRD()
	c.JSON(http.StatusOK, gin.H{"items": a.CRDStore.ListSnapshots()})
}

func (a *API) GetVaultSnapshotCRD(c *gin.Context) {
	a.ensureCRD()
	ns := c.Param("namespace")
	name := c.Param("name")
	obj, ok := a.CRDStore.GetSnapshot(ns, name)
	if !ok {
		writeErr(c, domain.NewAppError(domain.CodeNotFound, "VaultSnapshot not found"))
		return
	}
	c.JSON(http.StatusOK, obj)
}

func (a *API) ReconcileVaultSnapshotCRD(c *gin.Context) {
	a.ensureCRD()
	ns := c.Param("namespace")
	name := c.Param("name")
	out, err := a.Reconciler.ReconcileSnapshot(c.Request.Context(), ns, name)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *API) CreateVaultRestoreCRD(c *gin.Context) {
	a.ensureCRD()
	var body struct {
		Name        string `json:"name" binding:"required"`
		Namespace   string `json:"namespace" binding:"required"`
		SnapshotRef string `json:"snapshotRef" binding:"required"`
		PodName     string `json:"podName" binding:"required"`
		NodeName    string `json:"nodeName"`
		ClusterID   string `json:"clusterId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeErr(c, domain.NewAppError(domain.CodeValidation, err.Error()))
		return
	}
	nsMap := map[string]string{}
	if body.NodeName != "" {
		nsMap["kubernetes.io/hostname"] = body.NodeName
	}
	obj := &v1alpha1.VaultRestore{
		APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		Kind:       v1alpha1.KindRestore,
		Metadata:   v1alpha1.ObjectMeta{Name: body.Name, Namespace: body.Namespace, Generation: 1, UID: domain.NewID("crd")},
		Spec: v1alpha1.VaultRestoreSpec{
			SnapshotRef: body.SnapshotRef, ClusterID: body.ClusterID,
			Target: v1alpha1.VaultRestoreTarget{PodName: body.PodName, NodeSelector: nsMap},
		},
	}
	a.CRDStore.PutRestore(obj)
	out, err := a.Reconciler.ReconcileRestore(c.Request.Context(), body.Namespace, body.Name)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, out)
}

func defaultStr(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
