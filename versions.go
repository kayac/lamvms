package lamvms

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
)

func (app *App) deleteOldVersions(ctx context.Context, imageARN string, keep int) error {
	allVersions, err := app.listAllVersions(ctx, imageARN)
	if err != nil {
		return err
	}

	slices.SortFunc(allVersions, versionNewerFirst)

	activeCount := 0
	var cutoff int
	for i, v := range allVersions {
		if v.Status == types.MicrovmImageVersionStatusActive && v.State == types.MicrovmImageVersionStateSuccessful {
			activeCount++
			if activeCount == keep {
				cutoff = i + 1
				break
			}
		}
	}
	if cutoff == 0 {
		slog.Debug("not enough active versions to prune", "active", activeCount, "keep", keep)
		return nil
	}

	toDelete := allVersions[cutoff:]
	if len(toDelete) == 0 {
		return nil
	}

	deleted := 0
	for _, v := range toDelete {
		version := aws.ToString(v.ImageVersion)
		slog.Info("deleting old version", "version", version, "status", v.Status, "state", v.State)
		_, err := app.client.DeleteMicrovmImageVersion(ctx, &lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(imageARN),
			ImageVersion:    aws.String(version),
		})
		if err != nil {
			if _, ok := errors.AsType[*types.ConflictException](err); ok {
				slog.Warn("image is busy, skipping remaining version deletions", "version", version, "remaining", len(toDelete)-deleted, "error", err)
				break
			}
			return fmt.Errorf("failed to delete version %s: %w", version, err)
		}
		deleted++
	}

	slog.Info("deleted old versions", "count", deleted, "kept", keep)
	return nil
}
