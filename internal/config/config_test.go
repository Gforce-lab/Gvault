package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gvault/gvault/internal/config"
)

func TestLoadNVIDIADefaults(t *testing.T) {
	t.Setenv("PROVIDER", "")
	t.Setenv("ENABLE_NVIDIA_PROVIDER", "")
	t.Setenv("DEV_AUTH_BYPASS", "true")
	t.Setenv("ALLOW_TEST_PROVIDER", "")
	t.Setenv("API_KEY", "")
	t.Setenv("RESTORE_TIMEOUT_SECONDS", "300")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "nvidia-snapshot" || !cfg.EnableNVIDIAProvider {
		t.Fatalf("expected nvidia defaults, got provider=%s enable=%v", cfg.Provider, cfg.EnableNVIDIAProvider)
	}
}

func TestLoadRejectsFakeWithoutAllow(t *testing.T) {
	t.Setenv("PROVIDER", "fake")
	t.Setenv("ALLOW_TEST_PROVIDER", "false")
	t.Setenv("DEV_AUTH_BYPASS", "true")
	t.Setenv("RESTORE_TIMEOUT_SECONDS", "300")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "CI test harness") {
		t.Fatalf("expected fake gate error, got %v", err)
	}
}

func TestLoadAllowsFakeWithAllow(t *testing.T) {
	t.Setenv("PROVIDER", "fake")
	t.Setenv("ALLOW_TEST_PROVIDER", "true")
	t.Setenv("DEV_AUTH_BYPASS", "true")
	t.Setenv("RESTORE_TIMEOUT_SECONDS", "300")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "fake" || !cfg.AllowTestProvider {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoadRejectsAMDProvider(t *testing.T) {
	t.Setenv("PROVIDER", "amd")
	t.Setenv("DEV_AUTH_BYPASS", "true")
	t.Setenv("RESTORE_TIMEOUT_SECONDS", "300")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "AMD") {
		t.Fatalf("expected AMD out-of-scope error, got %v", err)
	}
}

func TestLoadRequiresAuthWhenNotBypassed(t *testing.T) {
	t.Setenv("PROVIDER", "nvidia-snapshot")
	t.Setenv("ENABLE_NVIDIA_PROVIDER", "true")
	t.Setenv("DEV_AUTH_BYPASS", "false")
	t.Setenv("API_KEY", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("RESTORE_TIMEOUT_SECONDS", "300")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected auth required error")
	}
}

func TestRestoreTimeoutConfiguration(t *testing.T) {
	t.Setenv("PROVIDER", "nvidia-snapshot")
	t.Setenv("ENABLE_NVIDIA_PROVIDER", "true")
	t.Setenv("DEV_AUTH_BYPASS", "true")
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"", 300 * time.Second}, {"1", time.Second}, {"3600", time.Hour},
		{"0", 0}, {"-1", 0}, {"3601", 0}, {"invalid", 0}, {"18446744073709551615", 0},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("RESTORE_TIMEOUT_SECONDS", tc.value)
			cfg, err := config.Load()
			if tc.want == 0 {
				if err == nil || !strings.Contains(err.Error(), "RESTORE_TIMEOUT_SECONDS") {
					t.Fatalf("expected timeout validation error, got %v", err)
				}
			} else if err != nil || cfg.RestoreTimeout != tc.want {
				t.Fatalf("timeout=%s err=%v", cfg.RestoreTimeout, err)
			}
		})
	}
}
