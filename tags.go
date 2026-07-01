package lamvms

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
)

func (app *App) syncTags(ctx context.Context, resourceARN string, desired map[string]string) error {
	current, err := app.client.ListTags(ctx, &lambdamicrovms.ListTagsInput{
		Resource: aws.String(resourceARN),
	})
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}

	toAdd := map[string]string{}
	for k, v := range desired {
		if cv, ok := current.Tags[k]; !ok || cv != v {
			toAdd[k] = v
		}
	}

	var toRemove []string
	for k := range current.Tags {
		if _, ok := desired[k]; !ok {
			toRemove = append(toRemove, k)
		}
	}

	if len(toAdd) == 0 && len(toRemove) == 0 {
		slog.Debug("tags are up to date")
		return nil
	}

	if len(toRemove) > 0 {
		slog.Info("removing tags", "keys", toRemove)
		_, err := app.client.UntagResource(ctx, &lambdamicrovms.UntagResourceInput{
			Resource: aws.String(resourceARN),
			TagKeys:  toRemove,
		})
		if err != nil {
			return fmt.Errorf("untag resource: %w", err)
		}
	}

	if len(toAdd) > 0 {
		slog.Info("adding tags", "tags", toAdd)
		_, err := app.client.TagResource(ctx, &lambdamicrovms.TagResourceInput{
			Resource: aws.String(resourceARN),
			Tags:     toAdd,
		})
		if err != nil {
			return fmt.Errorf("tag resource: %w", err)
		}
	}

	return nil
}
