package lamvms

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
)

// Wait waits for a MicroVM image version to be ready.
func (app *App) Wait(ctx context.Context, opt *WaitOption) error {
	img := app.microvmImage
	name := aws.ToString(img.Name)

	existing, err := app.findMicrovmImageByName(ctx, name)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("microvm image %q not found", name)
	}

	imageARN := aws.ToString(existing.ImageArn)

	if opt.Version != "" {
		slog.Info("waiting for version", "name", name, "version", opt.Version)
		if err := app.waitForVersion(ctx, imageARN, opt.Version, opt.BuildLogs); err != nil {
			return err
		}
	} else {
		switch existing.State {
		case types.MicrovmImageStateCreated, types.MicrovmImageStateUpdated:
			slog.Info("image is already ready", "name", name, "state", existing.State)
		case types.MicrovmImageStateCreateFailed, types.MicrovmImageStateUpdateFailed:
			return fmt.Errorf("image is in failed state: %s", existing.State)
		default:
			version, err := app.findLatestVersion(ctx, imageARN)
			if err != nil {
				return err
			}
			slog.Info("waiting for version", "name", name, "version", version)
			if err := app.waitForVersion(ctx, imageARN, version, opt.BuildLogs); err != nil {
				return err
			}
		}
	}

	if opt.KeepVersions > 0 {
		if err := app.deleteOldVersions(ctx, imageARN, opt.KeepVersions); err != nil {
			return err
		}
	}
	return nil
}

func (app *App) findLatestVersion(ctx context.Context, imageARN string) (string, error) {
	allVersions, err := app.listAllVersions(ctx, imageARN)
	if err != nil {
		return "", err
	}
	if len(allVersions) == 0 {
		return "", fmt.Errorf("no versions found for image %s", imageARN)
	}

	slices.SortFunc(allVersions, versionNewerFirst)
	return aws.ToString(allVersions[0].ImageVersion), nil
}

// listActiveVersions returns all ACTIVE+SUCCESSFUL versions.
func (app *App) listActiveVersions(ctx context.Context, imageARN string) ([]types.MicrovmImageVersionSummary, error) {
	allVersions, err := app.listAllVersions(ctx, imageARN)
	if err != nil {
		return nil, err
	}
	var active []types.MicrovmImageVersionSummary
	for _, v := range allVersions {
		if v.Status == types.MicrovmImageVersionStatusActive && v.State == types.MicrovmImageVersionStateSuccessful {
			active = append(active, v)
		}
	}
	return active, nil
}

func (app *App) listAllVersions(ctx context.Context, imageARN string) ([]types.MicrovmImageVersionSummary, error) {
	var versions []types.MicrovmImageVersionSummary
	var nextToken *string
	for {
		out, err := app.client.ListMicrovmImageVersions(ctx, &lambdamicrovms.ListMicrovmImageVersionsInput{
			ImageIdentifier: aws.String(imageARN),
			NextToken:       nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list microvm image versions: %w", err)
		}
		versions = append(versions, out.Items...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return versions, nil
}
