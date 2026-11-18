package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gvault/gvault/internal/authz"
)

// Config holds runtime settings for gvault API.
type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	LogLevel             string
	Provider             string
	DevAuthBypass        bool
	APIKey               string
	FakeProviderStep     time.Duration
	MigrationsDir        string
	PollInterval         time.Duration
	EnableNVIDIAProvider bool
	UIDir                string
	ProductName          string
	AllowedNamespaces    string
	OIDCIssuer           string
	OIDCAudience         string
	OIDCClientSecret     string
	EnableS3Export       bool
	LeaderElection       bool
	ProbeDefaultURL      string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:             getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:          getenv("DATABASE_URL", "postgres://vault:vault@localhost:5433/gvault?sslmode=disable"),
		LogLevel:             getenv("LOG_LEVEL", "info"),
		Provider:             getenv("PROVIDER", "fake"),
		DevAuthBypass:        getenvBool("DEV_AUTH_BYPASS", false),
		APIKey:               os.Getenv("API_KEY"),
		FakeProviderStep:     time.Duration(getenvInt("FAKE_PROVIDER_STEP_MS", 200)) * time.Millisecond,
		MigrationsDir:        getenv("MIGRATIONS_DIR", "db/migrations"),
		PollInterval:         time.Duration(getenvInt("POLL_INTERVAL_MS", 100)) * time.Millisecond,
		EnableNVIDIAProvider: getenvBool("ENABLE_NVIDIA_PROVIDER", false),
		UIDir:                getenv("UI_DIR", "web"),
		ProductName:          getenv("PRODUCT_NAME", "Gvault"),
		AllowedNamespaces:    os.Getenv("ALLOWED_NAMESPACES"),
		OIDCIssuer:           os.Getenv("OIDC_ISSUER"),
		OIDCAudience:         getenv("OIDC_AUDIENCE", "gvault"),
		OIDCClientSecret:     os.Getenv("OIDC_CLIENT_SECRET"),
		EnableS3Export:       getenvBool("ENABLE_S3_EXPORT", false),
		LeaderElection:       getenvBool("LEADER_ELECTION", false),
		ProbeDefaultURL:      os.Getenv("PROBE_DEFAULT_URL"),
	}
	switch cfg.Provider {
	case "fake", "nvidia-snapshot":
	default:
		return cfg, fmt.Errorf("unsupported PROVIDER %q (use fake or nvidia-snapshot)", cfg.Provider)
	}
	if cfg.Provider == "nvidia-snapshot" && !cfg.EnableNVIDIAProvider {
		return cfg, fmt.Errorf("PROVIDER=nvidia-snapshot requires ENABLE_NVIDIA_PROVIDER=true")
	}
	oidc := cfg.OIDC()
	if !cfg.DevAuthBypass && cfg.APIKey == "" && !oidc.Enabled() {
		return cfg, fmt.Errorf("API_KEY or OIDC_* is required when DEV_AUTH_BYPASS is false")
	}
	return cfg, nil
}

func (c Config) OIDC() authz.OIDCConfig {
	return authz.OIDCConfig{
		Issuer:   c.OIDCIssuer,
		Audience: c.OIDCAudience,
		Secret:   c.OIDCClientSecret,
	}
}

func (c Config) NamespacePolicy() *authz.NamespacePolicy {
	return authz.NewNamespacePolicy(c.AllowedNamespaces)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getenvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
