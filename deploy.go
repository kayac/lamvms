package lamvms

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
)

var waitInterval = 5 * time.Second

// Deploy deploys a MicroVM image.
func (app *App) Deploy(ctx context.Context, opt *DeployOption) error {
	img := app.microvmImage
	name := aws.ToString(img.Name)
	slog.Info("deploying", "name", name)

	if err := app.prepareCodeArtifact(ctx, opt, img); err != nil {
		return err
	}

	existing, err := app.findMicrovmImageByName(ctx, name)
	if err != nil {
		return err
	}

	if opt.DryRun {
		if existing == nil {
			slog.Info("dry run: would create new image", "name", name)
		} else {
			slog.Info("dry run: would update existing image", "name", name, "state", existing.State)
		}
		return nil
	}

	var imageARN, imageVersion string
	if existing == nil {
		imageARN, imageVersion, err = app.createMicrovmImage(ctx, img)
	} else {
		imageARN, imageVersion, err = app.updateMicrovmImage(ctx, existing, img)
	}
	if err != nil {
		return err
	}

	if existing != nil {
		if err := app.syncTags(ctx, imageARN, img.Tags); err != nil {
			return err
		}
	}

	if !opt.Wait {
		if opt.KeepVersions > 0 {
			slog.Warn("--keep-versions is ignored with --no-wait, use 'wait --keep-versions' after the build completes")
		}
		slog.Info("skipping wait", "image", imageARN, "version", imageVersion)
		return nil
	}

	if err := app.waitForVersion(ctx, imageARN, imageVersion); err != nil {
		return err
	}

	if opt.KeepVersions > 0 {
		if err := app.deleteOldVersions(ctx, imageARN, opt.KeepVersions); err != nil {
			return err
		}
	}

	return nil
}

func (app *App) prepareCodeArtifact(ctx context.Context, opt *DeployOption, img *MicrovmImage) error {
	if opt.SkipArchive {
		if img.CodeArtifact == nil {
			return fmt.Errorf("--skip-archive requires CodeArtifact.uri in microvm definition")
		}
		slog.Info("skipping archive creation")
		return nil
	}

	s3URI := codeArtifactURI(img)
	if s3URI == "" {
		return fmt.Errorf("CodeArtifact.uri is required in microvm definition")
	}

	src := filepath.Join(filepath.Dir(app.microvmFilePath), opt.Src)
	excludes := loadExcludes(src)
	zipfile, err := createZipArchive(src, excludes)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer func() {
		if err := zipfile.Close(); err != nil {
			slog.Warn("failed to close zip file", "error", err)
		}
		if err := os.Remove(zipfile.Name()); err != nil {
			slog.Warn("failed to remove temp file", "error", err)
		}
	}()

	if opt.DryRun {
		slog.Info("dry run: would upload to S3", "uri", s3URI)
		return nil
	}

	return app.uploadToS3(ctx, zipfile, s3URI)
}

func codeArtifactURI(img *MicrovmImage) string {
	if img.CodeArtifact == nil {
		return ""
	}
	if ca, ok := img.CodeArtifact.(*types.CodeArtifactMemberUri); ok {
		return ca.Value
	}
	return ""
}

func (app *App) findMicrovmImageByName(ctx context.Context, name string) (*lambdamicrovms.GetMicrovmImageOutput, error) {
	arn, err := findMicrovmImageARNByName(ctx, app.client, name)
	if err != nil {
		return nil, err
	}
	if arn == "" {
		return nil, nil
	}
	return app.getMicrovmImageByARN(ctx, arn)
}

func findMicrovmImageARNByName(ctx context.Context, client LambdaMicroVMsClient, name string) (string, error) {
	var nextToken *string
	for {
		out, err := client.ListMicrovmImages(ctx, &lambdamicrovms.ListMicrovmImagesInput{
			NameFilter: aws.String(name),
			NextToken:  nextToken,
		})
		if err != nil {
			return "", fmt.Errorf("list microvm images: %w", err)
		}
		for _, item := range out.Items {
			if aws.ToString(item.Name) == name {
				return aws.ToString(item.ImageArn), nil
			}
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return "", nil
}

func (app *App) getMicrovmImageByARN(ctx context.Context, arn string) (*lambdamicrovms.GetMicrovmImageOutput, error) {
	out, err := app.client.GetMicrovmImage(ctx, &lambdamicrovms.GetMicrovmImageInput{
		ImageIdentifier: aws.String(arn),
	})
	if err != nil {
		return nil, fmt.Errorf("get microvm image: %w", err)
	}
	return out, nil
}

func (app *App) createMicrovmImage(ctx context.Context, img *MicrovmImage) (string, string, error) {
	slog.Info("creating new microvm image", "name", aws.ToString(img.Name))
	input := lambdamicrovms.CreateMicrovmImageInput(*img)
	out, err := app.client.CreateMicrovmImage(ctx, &input)
	if err != nil {
		return "", "", fmt.Errorf("create microvm image: %w", err)
	}
	slog.Info("image creation started",
		"name", aws.ToString(out.Name),
		"image", aws.ToString(out.ImageArn),
		"version", aws.ToString(out.ImageVersion),
	)
	return aws.ToString(out.ImageArn), aws.ToString(out.ImageVersion), nil
}

func (app *App) updateMicrovmImage(ctx context.Context, existing *lambdamicrovms.GetMicrovmImageOutput, img *MicrovmImage) (string, string, error) {
	slog.Info("updating existing microvm image", "name", aws.ToString(img.Name))
	input := newUpdateMicrovmImageInput(img, aws.ToString(existing.ImageArn))
	out, err := app.client.UpdateMicrovmImage(ctx, input)
	if err != nil {
		return "", "", fmt.Errorf("update microvm image: %w", err)
	}
	slog.Info("image update started",
		"name", aws.ToString(out.Name),
		"image", aws.ToString(out.ImageArn),
		"version", aws.ToString(out.ImageVersion),
	)
	return aws.ToString(out.ImageArn), aws.ToString(out.ImageVersion), nil
}

func (app *App) waitForVersion(ctx context.Context, imageARN, imageVersion string) error {
	slog.Info("waiting for version to be ready", "image", imageARN, "version", imageVersion)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitInterval):
		}

		out, err := app.client.GetMicrovmImageVersion(ctx, &lambdamicrovms.GetMicrovmImageVersionInput{
			ImageIdentifier: aws.String(imageARN),
			ImageVersion:    aws.String(imageVersion),
		})
		if err != nil {
			return fmt.Errorf("get microvm image version: %w", err)
		}

		slog.Debug("version state", "image", imageARN, "version", imageVersion, "state", out.State)

		switch out.State {
		case types.MicrovmImageVersionStateSuccessful:
			slog.Info("version is ready", "version", imageVersion, "status", out.Status)
			return nil
		case types.MicrovmImageVersionStateFailed:
			reason := app.getFailureReason(ctx, imageARN, imageVersion)
			return fmt.Errorf("version build failed: version=%s reason=%s", imageVersion, reason)
		}
	}
}

func (app *App) getFailureReason(ctx context.Context, imageARN, imageVersion string) string {
	if imageVersion == "" {
		return "(unknown: no version)"
	}

	versionOut, err := app.client.GetMicrovmImageVersion(ctx, &lambdamicrovms.GetMicrovmImageVersionInput{
		ImageIdentifier: aws.String(imageARN),
		ImageVersion:    aws.String(imageVersion),
	})
	if err != nil {
		slog.Debug("failed to get version info", "error", err)
		return "(failed to get version info: " + err.Error() + ")"
	}
	if r := aws.ToString(versionOut.StateReason); r != "" {
		return r
	}

	buildsOut, err := app.client.ListMicrovmImageBuilds(ctx, &lambdamicrovms.ListMicrovmImageBuildsInput{
		ImageIdentifier: aws.String(imageARN),
		ImageVersion:    aws.String(imageVersion),
	})
	if err != nil {
		slog.Debug("failed to list builds", "error", err)
		return "(failed to list builds: " + err.Error() + ")"
	}
	var reasons []string
	for _, b := range buildsOut.Items {
		if b.BuildState != types.BuildStateFailed {
			continue
		}
		if r := aws.ToString(b.StateReason); r != "" {
			reasons = append(reasons, fmt.Sprintf("%s/%s: %s", b.Chipset, aws.ToString(b.ChipsetGeneration), r))
		}
	}
	if len(reasons) > 0 {
		return strings.Join(reasons, "; ")
	}
	return "(no reason found, check CloudWatch logs: /aws/lambda/microvms/)"
}
