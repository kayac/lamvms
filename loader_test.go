package lamvms

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestLoader_LoadJSON(t *testing.T) {
	t.Parallel()
	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	img, _, err := loader.Load("testdata/microvm.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(img.Name); got != "test-microvm" {
		t.Errorf("Name = %q, want %q", got, "test-microvm")
	}
	if got := aws.ToString(img.BaseImageArn); got != "arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1" {
		t.Errorf("BaseImageArn = %q, want arn:...:al2023-1", got)
	}
}

func TestLoader_LoadJsonnet(t *testing.T) {
	t.Parallel()
	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	img, _, err := loader.Load("testdata/microvm.jsonnet")
	if err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(img.Name); got != "test-microvm-jsonnet" {
		t.Errorf("Name = %q, want %q", got, "test-microvm-jsonnet")
	}
}

func TestLoader_JsonnetEnvFunctions(t *testing.T) {
	t.Setenv("TEST_MICROVM_NAME", "env-test-vm")
	t.Setenv("TEST_ACCOUNT_ID", "999999999999")

	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	img, _, err := loader.Load("testdata/microvm_with_env.jsonnet")
	if err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(img.Name); got != "env-test-vm" {
		t.Errorf("Name = %q, want %q", got, "env-test-vm")
	}
	want := "arn:aws:iam::999999999999:role/TestBuildRole"
	if got := aws.ToString(img.BuildRoleArn); got != want {
		t.Errorf("BuildRoleArn = %q, want %q", got, want)
	}
}

func TestLoader_JsonnetEnvDefault(t *testing.T) {
	t.Setenv("TEST_MICROVM_NAME", "default-test")

	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	img, _, err := loader.Load("testdata/microvm_with_env.jsonnet")
	if err != nil {
		t.Fatal(err)
	}
	want := "arn:aws:iam::000000000000:role/TestBuildRole"
	if got := aws.ToString(img.BuildRoleArn); got != want {
		t.Errorf("BuildRoleArn = %q, want %q (default should be used)", got, want)
	}
}

func TestLoader_JsonnetMustEnvMissing(t *testing.T) {
	t.Parallel()
	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	_, _, err := loader.Load("testdata/microvm_with_env.jsonnet")
	if err == nil {
		t.Fatal("expected error for missing must_env variable, got nil")
	}
}

func TestLoader_TemplateExpansion(t *testing.T) {
	t.Setenv("TEST_ACCOUNT_ID", "888888888888")

	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	img, _, err := loader.Load("testdata/microvm_template.json")
	if err != nil {
		t.Fatal(err)
	}
	wantRole := "arn:aws:iam::888888888888:role/TestBuildRole"
	if got := aws.ToString(img.BuildRoleArn); got != wantRole {
		t.Errorf("BuildRoleArn = %q, want %q", got, wantRole)
	}
}

func TestLoader_TemplateEnvOverride(t *testing.T) {
	t.Setenv("TEST_ACCOUNT_ID", "888888888888")
	t.Setenv("TEST_BUCKET", "custom-bucket")

	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	_, _, err := loader.Load("testdata/microvm_template.json")
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoader_ExtStr(t *testing.T) {
	t.Parallel()
	loader := NewLoader(context.Background(), aws.Config{}, map[string]string{"name": "ext-str-vm"}, nil)
	img, _, err := loader.Load("testdata/microvm_extstr.jsonnet")
	if err != nil {
		t.Fatal(err)
	}
	if got := aws.ToString(img.Name); got != "ext-str-vm" {
		t.Errorf("Name = %q, want %q", got, "ext-str-vm")
	}
}

func TestLoader_FileNotFound(t *testing.T) {
	t.Parallel()
	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	_, _, err := loader.Load("testdata/nonexistent.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestGeneratedFixtures(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("testdata/gen/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no generated fixtures found in testdata/gen/; run go generate")
	}
	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			img, _, err := loader.Load(f)
			if err != nil {
				t.Fatalf("load: %v", err)
			}

			data, err := json.Marshal(img)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var img2 MicrovmImage
			if err := json.Unmarshal(data, &img2); err != nil {
				t.Fatalf("roundtrip unmarshal: %v", err)
			}
		})
	}
}

func TestFindMicrovmFile_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := findMicrovmFile()
	if err == nil {
		t.Fatal("expected error when no default files exist, got nil")
	}
}
