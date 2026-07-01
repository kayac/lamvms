package lamvms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
)

// Diff shows the diff between local and deployed MicroVM image configuration.
func (app *App) Diff(ctx context.Context, opt *DiffOption) error {
	img := app.microvmImage
	name := aws.ToString(img.Name)

	existing, err := app.findMicrovmImageByName(ctx, name)
	if err != nil {
		return err
	}
	if existing == nil {
		slog.Info("image does not exist remotely, will be created", "name", name)
		localJSON, err := marshalForDiff(img)
		if err != nil {
			return err
		}
		fmt.Println(colorDiff("", string(localJSON)))
		return nil
	}

	imageARN := aws.ToString(existing.ImageArn)
	version := aws.ToString(existing.LatestActiveImageVersion)
	if version == "" {
		return fmt.Errorf("no active version found for image %q", name)
	}

	versionOut, err := app.client.GetMicrovmImageVersion(ctx, &lambdamicrovms.GetMicrovmImageVersionInput{
		ImageIdentifier: aws.String(imageARN),
		ImageVersion:    aws.String(version),
	})
	if err != nil {
		return fmt.Errorf("get microvm image version: %w", err)
	}

	remote := map[string]any{
		"Name":         name,
		"BaseImageArn": aws.ToString(versionOut.BaseImageArn),
		"BuildRoleArn": aws.ToString(versionOut.BuildRoleArn),
	}
	if versionOut.CodeArtifact != nil {
		ca, err := convertFromCodeArtifact(versionOut.CodeArtifact)
		if err != nil {
			slog.Warn("failed to convert CodeArtifact", "error", err)
		} else if ca != nil {
			remote["CodeArtifact"] = ca
		}
	}
	if versionOut.Description != nil {
		remote["Description"] = aws.ToString(versionOut.Description)
	}
	if versionOut.Hooks != nil {
		remote["Hooks"] = versionOut.Hooks
	}
	if len(versionOut.EnvironmentVariables) > 0 {
		remote["EnvironmentVariables"] = versionOut.EnvironmentVariables
	}
	if versionOut.Logging != nil {
		lg, err := convertFromLogging(versionOut.Logging)
		if err != nil {
			slog.Warn("failed to convert Logging", "error", err)
		} else if lg != nil {
			remote["Logging"] = lg
		}
	}
	if len(versionOut.AdditionalOsCapabilities) > 0 {
		remote["AdditionalOsCapabilities"] = versionOut.AdditionalOsCapabilities
	}
	if len(versionOut.Resources) > 0 {
		remote["Resources"] = versionOut.Resources
	}

	remoteJSON, err := marshalForDiff(omitEmptyValues(remote))
	if err != nil {
		return err
	}
	localJSON, err := marshalForDiff(img)
	if err != nil {
		return err
	}

	if string(remoteJSON) == string(localJSON) {
		slog.Info("no changes", "name", name, "version", version)
		if opt.ExitCode {
			return nil
		}
		return nil
	}

	diff := colorDiff(string(remoteJSON), string(localJSON))
	fmt.Println(diff)

	if opt.ExitCode {
		os.Exit(2)
	}
	return nil
}

func marshalForDiff(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal for diff: %w", err)
	}
	return data, nil
}

func colorDiff(remote, local string) string {
	remoteLines := strings.Split(remote, "\n")
	localLines := strings.Split(local, "\n")

	var buf strings.Builder
	buf.WriteString("--- remote\n+++ local\n")

	remoteSet := make(map[string]bool)
	for _, line := range remoteLines {
		remoteSet[line] = true
	}
	localSet := make(map[string]bool)
	for _, line := range localLines {
		localSet[line] = true
	}

	for _, line := range remoteLines {
		if !localSet[line] {
			fmt.Fprintf(&buf, "- %s\n", line)
		}
	}
	for _, line := range localLines {
		if !remoteSet[line] {
			fmt.Fprintf(&buf, "+ %s\n", line)
		}
	}

	return buf.String()
}
