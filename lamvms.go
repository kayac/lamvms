// Package lamvms provides a deployment tool for AWS Lambda MicroVMs.
package lamvms

//go:generate go run ./cmd/codegen/
//go:generate go run go.uber.org/mock/mockgen@latest -source=lamvms.go -destination=mock_test.go -package=lamvms

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/hashicorp/go-envparse"
)

// LambdaMicroVMsClient is the interface for AWS Lambda MicroVMs API operations.
type LambdaMicroVMsClient interface {
	CreateMicrovmImage(ctx context.Context, params *lambdamicrovms.CreateMicrovmImageInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.CreateMicrovmImageOutput, error)
	UpdateMicrovmImage(ctx context.Context, params *lambdamicrovms.UpdateMicrovmImageInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.UpdateMicrovmImageOutput, error)
	GetMicrovmImage(ctx context.Context, params *lambdamicrovms.GetMicrovmImageInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.GetMicrovmImageOutput, error)
	GetMicrovmImageVersion(ctx context.Context, params *lambdamicrovms.GetMicrovmImageVersionInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.GetMicrovmImageVersionOutput, error)
	UpdateMicrovmImageVersion(ctx context.Context, params *lambdamicrovms.UpdateMicrovmImageVersionInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.UpdateMicrovmImageVersionOutput, error)
	DeleteMicrovmImageVersion(ctx context.Context, params *lambdamicrovms.DeleteMicrovmImageVersionInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.DeleteMicrovmImageVersionOutput, error)
	ListMicrovmImages(ctx context.Context, params *lambdamicrovms.ListMicrovmImagesInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.ListMicrovmImagesOutput, error)
	ListMicrovmImageVersions(ctx context.Context, params *lambdamicrovms.ListMicrovmImageVersionsInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.ListMicrovmImageVersionsOutput, error)
	ListMicrovmImageBuilds(ctx context.Context, params *lambdamicrovms.ListMicrovmImageBuildsInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.ListMicrovmImageBuildsOutput, error)
	RunMicrovm(ctx context.Context, params *lambdamicrovms.RunMicrovmInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.RunMicrovmOutput, error)
	GetMicrovm(ctx context.Context, params *lambdamicrovms.GetMicrovmInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.GetMicrovmOutput, error)
	SuspendMicrovm(ctx context.Context, params *lambdamicrovms.SuspendMicrovmInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.SuspendMicrovmOutput, error)
	ResumeMicrovm(ctx context.Context, params *lambdamicrovms.ResumeMicrovmInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.ResumeMicrovmOutput, error)
	TerminateMicrovm(ctx context.Context, params *lambdamicrovms.TerminateMicrovmInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.TerminateMicrovmOutput, error)
	ListMicrovms(ctx context.Context, params *lambdamicrovms.ListMicrovmsInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.ListMicrovmsOutput, error)
	DeleteMicrovmImage(ctx context.Context, params *lambdamicrovms.DeleteMicrovmImageInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.DeleteMicrovmImageOutput, error)
	ListTags(ctx context.Context, params *lambdamicrovms.ListTagsInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.ListTagsOutput, error)
	TagResource(ctx context.Context, params *lambdamicrovms.TagResourceInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.TagResourceOutput, error)
	UntagResource(ctx context.Context, params *lambdamicrovms.UntagResourceInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.UntagResourceOutput, error)
	CreateMicrovmAuthToken(ctx context.Context, params *lambdamicrovms.CreateMicrovmAuthTokenInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.CreateMicrovmAuthTokenOutput, error)
	CreateMicrovmShellAuthToken(ctx context.Context, params *lambdamicrovms.CreateMicrovmShellAuthTokenInput, optFns ...func(*lambdamicrovms.Options)) (*lambdamicrovms.CreateMicrovmShellAuthTokenOutput, error)
}

// App represents lamvms application.
type App struct {
	awsConfig     aws.Config
	client        LambdaMicroVMsClient
	profile       string
	filterCommand string
	loader        *Loader

	microvmImage    *MicrovmImage
	microvmFilePath string
}

// New creates an App instance.
func New(ctx context.Context, opt *Option) (*App, error) {
	for _, envfile := range opt.Envfile {
		if err := exportEnvFile(envfile); err != nil {
			return nil, err
		}
	}

	awsCfg, err := newAWSConfig(ctx, opt)
	if err != nil {
		return nil, err
	}

	loader := NewLoader(ctx, awsCfg, opt.ExtStr, opt.ExtCode)

	var profile string
	if opt.Profile != nil {
		profile = *opt.Profile
	}

	app := &App{
		awsConfig:     awsCfg,
		client:        lambdamicrovms.NewFromConfig(awsCfg),
		profile:       profile,
		filterCommand: opt.FilterCommand,
		loader:        loader,
	}

	img, resolvedPath, err := app.loader.Load(opt.Microvm)
	if err != nil {
		return nil, err
	}
	app.microvmImage = img
	app.microvmFilePath = resolvedPath

	return app, nil
}

func newAWSConfig(ctx context.Context, opt *Option) (aws.Config, error) {
	var cfgOpts []func(*awsconfig.LoadOptions) error
	if opt.Region != nil && *opt.Region != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithRegion(*opt.Region))
	}
	if opt.Profile != nil && *opt.Profile != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithSharedConfigProfile(*opt.Profile))
	}
	if opt.Endpoint != nil && *opt.Endpoint != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithBaseEndpoint(*opt.Endpoint))
	}
	return awsconfig.LoadDefaultConfig(ctx, cfgOpts...)
}

func exportEnvFile(file string) error {
	if file == "" {
		return nil
	}
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("failed to open envfile %s: %w", file, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("failed to close envfile", "path", file, "error", err)
		}
	}()

	envs, err := envparse.Parse(f)
	if err != nil {
		return fmt.Errorf("failed to parse envfile %s: %w", file, err)
	}
	for key, value := range envs {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set env %s: %w", key, err)
		}
	}
	return nil
}
