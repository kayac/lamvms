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

// Rollback deactivates the latest active version.
func (app *App) Rollback(ctx context.Context, opt *RollbackOption) error {
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
	activeVersions, err := app.listActiveVersions(ctx, imageARN)
	if err != nil {
		return err
	}

	if len(activeVersions) < 2 {
		return fmt.Errorf("rollback requires at least 2 active versions, found %d", len(activeVersions))
	}

	slices.SortFunc(activeVersions, versionNewerFirst)

	latest := activeVersions[0]
	previous := activeVersions[1]
	latestVersion := aws.ToString(latest.ImageVersion)
	previousVersion := aws.ToString(previous.ImageVersion)

	slog.Info("rolling back",
		"name", name,
		"deactivating", latestVersion,
		"falling_back_to", previousVersion,
	)

	if opt.DryRun {
		slog.Info("dry run: would deactivate version", "version", latestVersion)
		return nil
	}

	_, err = app.client.UpdateMicrovmImageVersion(ctx, &lambdamicrovms.UpdateMicrovmImageVersionInput{
		ImageIdentifier: aws.String(imageARN),
		ImageVersion:    aws.String(latestVersion),
		Status:          types.MicrovmImageVersionStatusInactive,
	})
	if err != nil {
		return fmt.Errorf("failed to deactivate version %s: %w", latestVersion, err)
	}

	slog.Info("rollback completed",
		"deactivated", latestVersion,
		"active", previousVersion,
	)
	return nil
}
