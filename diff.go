package lamvms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
	"github.com/fatih/color"
	"github.com/kylelemons/godebug/diff"
)

// defaultMinimumMemoryInMiB is the baseline memory Lambda MicroVMs uses when
// Resources is omitted. See https://docs.aws.amazon.com/lambda/latest/dg/microvms-images.html#microvms-images-sizing
const defaultMinimumMemoryInMiB int32 = 2048

// ErrDiff indicates that differences were found between local and remote configurations.
var ErrDiff = errors.New("diff found")

// Diff shows the diff between local and deployed MicroVM image configuration.
//
// Resources and EgressNetworkConnectors resolve to a fixed AWS-side default
// when omitted locally, so the local value is filled with that default before
// comparing. BaseImageVersion instead resolves to "the latest base image at
// build time" when omitted, which always differs from whatever version is
// currently deployed; diffing it is not actionable, so it is excluded from
// the comparison whenever the local config doesn't pin a version.
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
		fmt.Println(coloredDiff("", string(localJSON)))
		if opt.ExitCode {
			return ErrDiff
		}
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

	remote := buildRemoteMap(name, versionOut)
	if img.BaseImageVersion == nil {
		delete(remote, "BaseImageVersion")
	}

	remoteJSON, err := marshalForDiff(omitEmptyValues(remote))
	if err != nil {
		return err
	}
	localJSON, err := marshalForDiff(fillDefaultValues(img))
	if err != nil {
		return err
	}

	if string(remoteJSON) == string(localJSON) {
		slog.Info("no changes", "name", name, "version", version)
		return nil
	}

	fmt.Println(coloredDiff(string(remoteJSON), string(localJSON)))

	if opt.ExitCode {
		return ErrDiff
	}
	return nil
}

// fillDefaultValues returns a copy of img with Resources and
// EgressNetworkConnectors pre-filled to the value AWS resolves them to on the
// server side when omitted, so that diffing against an omitted local field
// does not report a spurious difference.
func fillDefaultValues(img *MicrovmImage) *MicrovmImage {
	filled := *img
	if len(filled.Resources) == 0 {
		filled.Resources = []types.Resources{
			{MinimumMemoryInMiB: aws.Int32(defaultMinimumMemoryInMiB)},
		}
	}
	if len(filled.EgressNetworkConnectors) == 0 {
		baseImageARN, err := arn.Parse(aws.ToString(filled.BaseImageArn))
		if err != nil {
			slog.Warn("failed to parse BaseImageArn", "error", err)
		} else {
			filled.EgressNetworkConnectors = []string{
				fmt.Sprintf("arn:%s:lambda:%s:aws:network-connector:aws-network-connector:INTERNET_EGRESS",
					baseImageARN.Partition, baseImageARN.Region),
			}
		}
	}
	return &filled
}

func marshalForDiff(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal for diff: %w", err)
	}
	return data, nil
}

func buildRemoteMap(name string, v *lambdamicrovms.GetMicrovmImageVersionOutput) map[string]any {
	m := map[string]any{
		"Name":         name,
		"BaseImageArn": aws.ToString(v.BaseImageArn),
		"BuildRoleArn": aws.ToString(v.BuildRoleArn),
	}
	if v.CodeArtifact != nil {
		ca, err := convertFromCodeArtifact(v.CodeArtifact)
		if err != nil {
			slog.Warn("failed to convert CodeArtifact", "error", err)
		} else if ca != nil {
			m["CodeArtifact"] = ca
		}
	}
	if v.Description != nil {
		m["Description"] = aws.ToString(v.Description)
	}
	if v.Hooks != nil {
		m["Hooks"] = v.Hooks
	}
	if len(v.EnvironmentVariables) > 0 {
		m["EnvironmentVariables"] = v.EnvironmentVariables
	}
	if v.Logging != nil {
		lg, err := convertFromLogging(v.Logging)
		if err != nil {
			slog.Warn("failed to convert Logging", "error", err)
		} else if lg != nil {
			m["Logging"] = lg
		}
	}
	if len(v.AdditionalOsCapabilities) > 0 {
		m["AdditionalOsCapabilities"] = v.AdditionalOsCapabilities
	}
	if len(v.Resources) > 0 {
		m["Resources"] = v.Resources
	}
	if v.BaseImageVersion != nil {
		m["BaseImageVersion"] = aws.ToString(v.BaseImageVersion)
	}
	if len(v.CpuConfigurations) > 0 {
		m["CpuConfigurations"] = v.CpuConfigurations
	}
	if len(v.EgressNetworkConnectors) > 0 {
		m["EgressNetworkConnectors"] = v.EgressNetworkConnectors
	}
	if len(v.Tags) > 0 {
		m["Tags"] = v.Tags
	}
	return m
}

func coloredDiff(remote, local string) string {
	d := diff.Diff(remote, local)
	if d == "" {
		return ""
	}

	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	var buf strings.Builder
	buf.WriteString("--- remote\n+++ local\n")
	for _, line := range strings.Split(d, "\n") {
		switch {
		case strings.HasPrefix(line, "-"):
			buf.WriteString(red.Sprint(line))
		case strings.HasPrefix(line, "+"):
			buf.WriteString(green.Sprint(line))
		default:
			buf.WriteString(line)
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}
