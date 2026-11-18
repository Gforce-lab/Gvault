package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gvault/gvault/internal/config"
)

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
