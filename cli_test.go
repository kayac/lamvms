package lamvms

import (
	"testing"
)

func TestParseCLI_Deploy(t *testing.T) {
	t.Parallel()
	sub, opts, _, err := ParseCLI([]string{"deploy", "--microvm", "testdata/microvm.json"})
	if err != nil {
		t.Fatal(err)
	}
	if sub != "deploy" {
		t.Errorf("sub = %q, want %q", sub, "deploy")
	}
	if opts.Microvm != "testdata/microvm.json" {
		t.Errorf("Microvm = %q, want %q", opts.Microvm, "testdata/microvm.json")
	}
	if opts.Deploy == nil {
		t.Fatal("Deploy option is nil")
	}
	if opts.Deploy.DryRun {
		t.Error("DryRun should be false by default")
	}
}

func TestParseCLI_DeployDryRun(t *testing.T) {
	t.Parallel()
	sub, opts, _, err := ParseCLI([]string{"deploy", "--dry-run"})
	if err != nil {
		t.Fatal(err)
	}
	if sub != "deploy" {
		t.Errorf("sub = %q, want %q", sub, "deploy")
	}
	if opts.Deploy == nil || !opts.Deploy.DryRun {
		t.Error("DryRun should be true")
	}
}

func TestParseCLI_ExtStr(t *testing.T) {
	t.Parallel()
	sub, opts, _, err := ParseCLI([]string{"deploy", "-V", "key=value", "-V", "foo=bar"})
	if err != nil {
		t.Fatal(err)
	}
	if sub != "deploy" {
		t.Errorf("sub = %q, want %q", sub, "deploy")
	}
	if opts.ExtStr["key"] != "value" {
		t.Errorf("ExtStr[key] = %q, want %q", opts.ExtStr["key"], "value")
	}
	if opts.ExtStr["foo"] != "bar" {
		t.Errorf("ExtStr[foo] = %q, want %q", opts.ExtStr["foo"], "bar")
	}
}

func TestParseCLI_LogLevel(t *testing.T) {
	t.Parallel()
	_, opts, _, err := ParseCLI([]string{"deploy", "--log-level", "debug"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", opts.LogLevel, "debug")
	}
}

func TestParseCLI_LogFormat(t *testing.T) {
	t.Parallel()
	_, opts, _, err := ParseCLI([]string{"deploy", "--log-format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", opts.LogFormat, "json")
	}
}

func TestParseCLI_NoColor(t *testing.T) {
	t.Parallel()
	_, opts, _, err := ParseCLI([]string{"deploy", "--no-color"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Color {
		t.Error("Color should be false with --no-color")
	}
}

func TestParseCLI_InvalidCommand(t *testing.T) {
	t.Parallel()
	_, _, _, err := ParseCLI([]string{"invalid"})
	if err == nil {
		t.Fatal("expected error for invalid command, got nil")
	}
}
