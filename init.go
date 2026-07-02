package lamvms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
)

func initCmd(ctx context.Context, globalOpt *Option, opt *InitOption) (int, error) {
	awsCfg, err := newAWSConfig(ctx, globalOpt)
	if err != nil {
		return 1, err
	}
	client := lambdamicrovms.NewFromConfig(awsCfg)
	return runInit(ctx, client, opt)
}

func runInit(ctx context.Context, client LambdaMicroVMsClient, opt *InitOption) (int, error) {
	slog.Info("looking up image", "name", opt.ImageName)

	imageARN, err := findMicrovmImageARNByName(ctx, client, opt.ImageName)
	if err != nil {
		return 1, err
	}
	if imageARN == "" {
		return 1, fmt.Errorf("microvm image %q not found", opt.ImageName)
	}

	img, err := client.GetMicrovmImage(ctx, &lambdamicrovms.GetMicrovmImageInput{
		ImageIdentifier: aws.String(imageARN),
	})
	if err != nil {
		return 1, fmt.Errorf("get microvm image: %w", err)
	}

	version := aws.ToString(img.LatestActiveImageVersion)
	if version == "" {
		return 1, fmt.Errorf("no active version found for image %q", opt.ImageName)
	}

	versionOut, err := client.GetMicrovmImageVersion(ctx, &lambdamicrovms.GetMicrovmImageVersionInput{
		ImageIdentifier: aws.String(imageARN),
		ImageVersion:    aws.String(version),
	})
	if err != nil {
		return 1, fmt.Errorf("get microvm image version: %w", err)
	}

	def := buildRemoteMap(opt.ImageName, versionOut)
	cleaned := omitEmptyValues(def)

	data, err := json.MarshalIndent(cleaned, "", "  ")
	if err != nil {
		return 1, fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')

	outputPath := opt.Output
	if opt.Jsonnet && outputPath == "microvm.json" {
		outputPath = "microvm.jsonnet"
	}

	if _, err := os.Stat(outputPath); err == nil && !opt.ForceOverwrite {
		return 1, fmt.Errorf("%s already exists, use --force-overwrite to overwrite", outputPath)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return 1, fmt.Errorf("write %s: %w", outputPath, err)
	}

	slog.Info("wrote microvm definition", "path", outputPath, "version", version)
	return 0, nil
}
