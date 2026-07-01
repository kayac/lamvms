package lamvms

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"go.uber.org/mock/gomock"
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

func newFakeCurl(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := "curl"
	if runtime.GOOS == "windows" {
		name = "curl.bat"
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FAKE_CURL_ARGS_FILE\"\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return filepath.Join(t.TempDir(), "args.txt")
}

func TestCurl_BuildsURLFromPath(t *testing.T) {
	argsFile := newFakeCurl(t)
	t.Setenv("FAKE_CURL_ARGS_FILE", argsFile)

	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)
	mock.EXPECT().
		GetMicrovm(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmOutput{
			Endpoint: aws.String("abc.example.com"),
		}, nil)
	mock.EXPECT().
		CreateMicrovmAuthToken(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.CreateMicrovmAuthTokenOutput{
			AuthToken: map[string]string{"X-aws-proxy-auth": "test-token"},
		}, nil)

	app := &App{client: mock}
	err := app.Curl(context.Background(), &CurlOption{
		MicrovmID:       "test-id",
		TokenExpiration: 5 * time.Minute,
		Path:            "/health",
		Args:            []string{"-X", "POST"},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	gotArgs := strings.Split(strings.TrimRight(string(got), "\n"), "\n")
	want := []string{"--config", "-", "https://abc.example.com/health", "-X", "POST"}
	if strings.Join(gotArgs, " ") != strings.Join(want, " ") {
		t.Errorf("curl args = %v, want %v", gotArgs, want)
	}
}

func TestCurl_PreservesQueryString(t *testing.T) {
	argsFile := newFakeCurl(t)
	t.Setenv("FAKE_CURL_ARGS_FILE", argsFile)

	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)
	mock.EXPECT().
		GetMicrovm(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmOutput{
			Endpoint: aws.String("abc.example.com"),
		}, nil)
	mock.EXPECT().
		CreateMicrovmAuthToken(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.CreateMicrovmAuthTokenOutput{
			AuthToken: map[string]string{"X-aws-proxy-auth": "test-token"},
		}, nil)

	app := &App{client: mock}
	err := app.Curl(context.Background(), &CurlOption{
		MicrovmID:       "test-id",
		TokenExpiration: 5 * time.Minute,
		Path:            "/search?q=x",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	gotArgs := strings.Split(strings.TrimRight(string(got), "\n"), "\n")
	want := []string{"--config", "-", "https://abc.example.com/search?q=x"}
	if strings.Join(gotArgs, " ") != strings.Join(want, " ") {
		t.Errorf("curl args = %v, want %v", gotArgs, want)
	}
}

func TestCurl_RejectsFullURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	app := &App{client: mock}
	err := app.Curl(context.Background(), &CurlOption{
		MicrovmID: "test-id",
		Path:      "http://evil.example.com/health",
	})
	if err == nil {
		t.Fatal("expected error for path with scheme/host")
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
