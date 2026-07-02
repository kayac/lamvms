package lamvms

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestCreateZipArchive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log('hello')"), 0644)
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM node:24"), 0644)

	f, err := createZipArchive(dir, defaultExcludes)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	r, err := zip.OpenReader(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	names := make(map[string]bool)
	for _, entry := range r.File {
		names[entry.Name] = true
	}
	if !names["app.js"] {
		t.Error("app.js not found in archive")
	}
	if !names["Dockerfile"] {
		t.Error("Dockerfile not found in archive")
	}
}

func TestCreateZipArchive_Excludes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.js"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "microvm.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, ".microvmignore"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(""), 0644)

	f, err := createZipArchive(dir, defaultExcludes)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	r, err := zip.OpenReader(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	for _, entry := range r.File {
		switch entry.Name {
		case "microvm.json", ".microvmignore":
			t.Errorf("excluded file %q found in archive", entry.Name)
		}
		if filepath.Dir(entry.Name) == ".git" {
			t.Errorf("excluded .git file %q found in archive", entry.Name)
		}
	}
}

func TestCreateZipArchive_CustomIgnore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.js"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "secret.key"), []byte("secret"), 0644)
	os.WriteFile(filepath.Join(dir, ".microvmignore"), []byte("secret.key\n"), 0644)

	excludes := loadExcludes(dir)
	f, err := createZipArchive(dir, excludes)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	r, err := zip.OpenReader(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	for _, entry := range r.File {
		if entry.Name == "secret.key" {
			t.Error("secret.key should be excluded by .microvmignore")
		}
	}
}

func TestMatchExcludes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path    string
		pattern string
		want    bool
	}{
		{"microvm.json", "microvm.json", true},
		{"microvm.jsonnet", "microvm.jsonnet", true},
		{".git/config", ".git/*", true},
		{".git/objects/pack/idx", ".git/*", true},
		{"app.js", "microvm.json", false},
		{"app.js", ".git/*", false},
	}
	for _, tt := range tests {
		got := matchExcludes(tt.path, []string{tt.pattern})
		if got != tt.want {
			t.Errorf("matchExcludes(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
		}
	}
}

func TestParseS3URI(t *testing.T) {
	t.Parallel()
	bucket, key, err := parseS3URI("s3://my-bucket/path/to/artifact.zip")
	if err != nil {
		t.Fatal(err)
	}
	if bucket != "my-bucket" {
		t.Errorf("bucket = %q, want %q", bucket, "my-bucket")
	}
	if key != "path/to/artifact.zip" {
		t.Errorf("key = %q, want %q", key, "path/to/artifact.zip")
	}
}

func TestParseS3URI_Invalid(t *testing.T) {
	t.Parallel()
	_, _, err := parseS3URI("https://example.com/file.zip")
	if err == nil {
		t.Fatal("expected error for non-s3 URI")
	}
}

func TestLoadExcludes_NoFile(t *testing.T) {
	t.Parallel()
	excludes := loadExcludes(t.TempDir())
	if len(excludes) != len(defaultExcludes) {
		t.Errorf("len(excludes) = %d, want %d (defaultExcludes)", len(excludes), len(defaultExcludes))
	}
}

func TestLoadExcludes_WithFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".microvmignore"), []byte("*.log\n# comment\n\ntemp/*\n"), 0644)

	excludes := loadExcludes(dir)
	want := len(defaultExcludes) + 2
	if len(excludes) != want {
		t.Errorf("len(excludes) = %d, want %d", len(excludes), want)
	}
}

func TestCodeArtifactURI(t *testing.T) {
	t.Parallel()
	loader := NewLoader(context.Background(), aws.Config{}, nil, nil)
	img, _, err := loader.Load("testdata/gen/codeArtifact_uri.json")
	if err != nil {
		t.Fatal(err)
	}
	uri := codeArtifactURI(img)
	if uri != "s3://gen-test-bucket/artifact.zip" {
		t.Errorf("codeArtifactURI = %q, want %q", uri, "s3://gen-test-bucket/artifact.zip")
	}
}
