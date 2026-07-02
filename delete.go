package lamvms

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
)

// Delete deletes a MicroVM image.
func (app *App) Delete(ctx context.Context, opt *DeleteOption) error {
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
	slog.Info("deleting microvm image", "name", name, "image", imageARN)

	if opt.DryRun {
		slog.Info("dry run: would delete image", "name", name)
		return nil
	}

	_, err = app.client.DeleteMicrovmImage(ctx, &lambdamicrovms.DeleteMicrovmImageInput{
		ImageIdentifier: aws.String(imageARN),
	})
	if err != nil {
		return fmt.Errorf("delete microvm image: %w", err)
	}

	slog.Info("microvm image deleted", "name", name)
	return nil
}
