package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gvault/gvault/internal/api"
	"github.com/gvault/gvault/internal/api/handlers"
	"github.com/gvault/gvault/internal/config"
	"github.com/gvault/gvault/internal/controller"
	"github.com/gvault/gvault/internal/kube"
	"github.com/gvault/gvault/internal/probe"
	"github.com/gvault/gvault/internal/provider"
	"github.com/gvault/gvault/internal/provider/fake"
	"github.com/gvault/gvault/internal/provider/nvidia"
	"github.com/gvault/gvault/internal/repository/postgres"
	"github.com/gvault/gvault/internal/service"
	"github.com/gvault/gvault/internal/telemetry"
	"github.com/gvault/gvault/internal/version"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))

	db, err := postgres.Open(cfg.DatabaseURL)
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := postgres.MigrateUp(ctx, db, cfg.MigrationsDir); err != nil {
		log.Error("migrate", "error", err)
		os.Exit(1)
	}

	var p provider.CheckpointProvider
	var prober probe.Prober = probe.HTTPProber{}
	switch cfg.Provider {
	case "fake":
		// CI / ALLOW_TEST_PROVIDER only — not a product GPU path.
		log.Warn("test harness provider enabled", "note", "not a real GPU provider; see docs/scope.md")
		p = fake.New(cfg.FakeProviderStep)
		prober = probe.FakeProber{}
	default:
		np := nvidia.New(cfg.EnableNVIDIAProvider)
		kc, err := kube.MaybeFromEnv()
		if err != nil {
			log.Error("kube client", "error", err)
			os.Exit(1)
		}
		if kc != nil {
			np.WithKubeClient(kc)
			log.Info("nvidia provider wired to kubernetes client", "pinned", nvidia.PinnedRelease)
		} else {
			log.Warn("nvidia provider active but no kube client; capture/restore stay fail-closed until cluster is connected",
				"pinned", nvidia.PinnedRelease,
				"docs", "docs/gpu-setup.md",
			)
		}
		p = np
	}

	metrics := telemetry.New()
	snaps := postgres.NewSnapshotRepo(db)
	ops := postgres.NewOperationRepo(db)
	audits := postgres.NewAuditRepo(db)
	svc := service.NewSnapshotService(db, snaps, ops, audits, p, log, cfg.PollInterval).
		WithMetrics(metrics).
		WithNamespacePolicy(cfg.NamespacePolicy()).
		WithRestoreTimeout(cfg.RestoreTimeout).
		WithProber(prober)
	crdStore := controller.NewStore()
	h := &handlers.API{
		DB: db, Svc: svc, Provider: p, Metrics: metrics,
		CRDStore: crdStore, Reconciler: controller.NewReconciler(crdStore, p),
		StatusSync:     &controller.StatusSync{Snaps: snaps},
		EnableS3Export: cfg.EnableS3Export,
	}
	router := api.NewRouter(cfg, log, h)

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: router}
	go func() {
		log.Info("gvault-api listening",
			"product", cfg.ProductName,
			"version", version.Version,
			"channel", "beta",
			"addr", cfg.HTTPAddr,
			"provider", p.Name(),
			"phase", "1-nvidia-only",
			"amd", "out-of-scope",
			"ui", cfg.UIDir,
			"oidc", cfg.OIDC().Enabled(),
			"namespacePolicy", cfg.NamespacePolicy().Enabled(),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
