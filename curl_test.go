package lamvms

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestExecCurl(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not found")
	}

	err := execCurl(context.Background(), []string{"--config", "-", "--version"}, "test-token")
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecCurl_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := execCurl(context.Background(), []string{"--version"}, "test-token")
	if err == nil {
		t.Fatal("expected error when curl not found")
	}
}

func TestExpirationMinutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		d    string
		want int32
	}{
		{"30m", "30m", 30},
		{"90s rounds up to 2", "90s", 2},
		{"30s clamps to 1", "30s", 1},
		{"0s clamps to 1", "0s", 1},
		{"59s rounds up to 1", "59s", 1},
		{"61s rounds up to 2", "61s", 2},
		{"1h", "1h", 60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := time.ParseDuration(tt.d)
			if err != nil {
				t.Fatal(err)
			}
			got := expirationMinutes(d)
			if got != tt.want {
				t.Errorf("expirationMinutes(%s) = %d, want %d", tt.d, got, tt.want)
			}
		})
	}
}
