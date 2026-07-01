package lamvms

import (
	"os"
	"testing"
)

func TestExportEnvFile(t *testing.T) {
	t.Setenv("TEST_ENVFILE_VAR", "")
	t.Setenv("TEST_ACCOUNT_ID", "")

	if err := exportEnvFile("testdata/envfile.env"); err != nil {
		t.Fatal(err)
	}

	if got := os.Getenv("TEST_ENVFILE_VAR"); got != "from-envfile" {
		t.Errorf("TEST_ENVFILE_VAR = %q, want %q", got, "from-envfile")
	}
	if got := os.Getenv("TEST_ACCOUNT_ID"); got != "111111111111" {
		t.Errorf("TEST_ACCOUNT_ID = %q, want %q", got, "111111111111")
	}
}

func TestExportEnvFile_Empty(t *testing.T) {
	if err := exportEnvFile(""); err != nil {
		t.Fatal(err)
	}
}

func TestExportEnvFile_NotFound(t *testing.T) {
	err := exportEnvFile("testdata/nonexistent.env")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
