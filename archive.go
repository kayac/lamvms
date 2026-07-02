package lamvms

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// IgnoreFilename is the name of the file listing exclusion patterns for zip archive creation.
const IgnoreFilename = ".microvmignore"

var defaultExcludes = []string{
	IgnoreFilename,
	"microvm.json",
	"microvm.jsonnet",
	".git/*",
}

func createZipArchive(src string, excludes []string, keepSymlink bool) (*os.File, error) {
	slog.Info("creating zip archive", "src", src)
	tmpfile, err := os.CreateTemp("", "lamvms-archive-*.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	w := zip.NewWriter(tmpfile)
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relpath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		relpath = filepath.ToSlash(relpath)
		if matchExcludes(relpath, excludes) {
			slog.Debug("skipping", "path", relpath)
			return nil
		}
		slog.Debug("archiving", "path", relpath)
		return addToZip(w, path, relpath, d, keepSymlink)
	})
	cleanup := func() {
		if err := tmpfile.Close(); err != nil {
			slog.Warn("failed to close temp file", "error", err)
		}
		if err := os.Remove(tmpfile.Name()); err != nil {
			slog.Warn("failed to remove temp file", "error", err)
		}
	}
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}
	if err := w.Close(); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to finalize zip: %w", err)
	}
	if _, err := tmpfile.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}
	stat, err := tmpfile.Stat()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to stat temp file: %w", err)
	}
	slog.Info("zip archive created", "bytes", stat.Size())
	return tmpfile, nil
}

func addToZip(z *zip.Writer, path, relpath string, d fs.DirEntry, keepSymlink bool) error {
	info, err := d.Info()
	if err != nil {
		return err
	}

	var r io.Reader
	if info.Mode()&fs.ModeSymlink != 0 {
		if keepSymlink {
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", path, err)
			}
			r = strings.NewReader(link)
		} else {
			resolvedPath, resolvedInfo, err := followSymlink(path)
			if err != nil {
				slog.Warn("failed to follow symlink, skipping", "path", path, "error", err)
				return nil
			}
			path, info = resolvedPath, resolvedInfo
		}
	}
	if r == nil {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			if err := f.Close(); err != nil {
				slog.Warn("failed to close file", "path", path, "error", err)
			}
		}()
		r = f
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = relpath
	header.Method = zip.Deflate
	w, err := z.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}

func followSymlink(path string) (string, fs.FileInfo, error) {
	link, err := os.Readlink(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read symlink %s: %w", path, err)
	}
	target := link
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", nil, fmt.Errorf("failed to stat symlink target %s: %w", target, err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("symlink target is a directory: %s", target)
	}
	return target, info, nil
}

func matchExcludes(path string, excludes []string) bool {
	for _, pattern := range excludes {
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(path, prefix+"/") {
				return true
			}
		}
	}
	return false
}

func loadExcludes(src string) []string {
	excludes := append([]string{}, defaultExcludes...)
	path := filepath.Join(src, IgnoreFilename)
	f, err := os.Open(path)
	if err != nil {
		return excludes
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("failed to close ignore file", "path", path, "error", err)
		}
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		excludes = append(excludes, line)
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("failed to read ignore file", "path", path, "error", err)
	}
	return excludes
}

func (app *App) uploadToS3(ctx context.Context, f *os.File, s3URI string) error {
	bucket, key, err := parseS3URI(s3URI)
	if err != nil {
		return err
	}
	slog.Info("uploading to S3", "bucket", bucket, "key", key)
	svc := s3.NewFromConfig(app.awsConfig)
	_, err = svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to s3://%s/%s: %w", bucket, key, err)
	}
	slog.Info("uploaded to S3", "uri", s3URI)
	return nil
}

func parseS3URI(uri string) (bucket, key string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("invalid S3 URI %q: %w", uri, err)
	}
	if u.Scheme != "s3" {
		return "", "", fmt.Errorf("invalid S3 URI scheme %q (expected s3://)", uri)
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}
