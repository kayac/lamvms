package lamvms

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
	"go.uber.org/mock/gomock"
)

func init() {
	waitInterval = 10 * time.Millisecond
}

const testImageARN = "arn:aws:lambda:ap-northeast-1:123456789012:microvm-image:test-microvm"

func newTestApp(t *testing.T, client LambdaMicroVMsClient, fixturePath string) *App {
	t.Helper()
	loader := NewLoader(aws.Config{}, nil, nil)
	img, _, err := loader.Load(context.Background(), fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	return &App{
		client:       client,
		loader:       loader,
		microvmImage: img,
	}
}

func expectListNotFound(mock *MockLambdaMicroVMsClient) {
	mock.EXPECT().
		ListMicrovmImages(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImagesOutput{
			Items: []types.MicrovmImageSummary{},
		}, nil)
}

func expectListFound(mock *MockLambdaMicroVMsClient) {
	mock.EXPECT().
		ListMicrovmImages(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImagesOutput{
			Items: []types.MicrovmImageSummary{
				{
					Name:     aws.String("test-microvm"),
					ImageArn: aws.String(testImageARN),
					State:    types.MicrovmImageStateCreated,
				},
			},
		}, nil)
	mock.EXPECT().
		GetMicrovmImage(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageOutput{
			Name:                     aws.String("test-microvm"),
			ImageArn:                 aws.String(testImageARN),
			State:                    types.MicrovmImageStateCreated,
			LatestActiveImageVersion: aws.String("1.0"),
		}, nil)
}

func expectTagsEmpty(mock *MockLambdaMicroVMsClient) {
	mock.EXPECT().
		ListTags(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListTagsOutput{
			Tags: map[string]string{},
		}, nil)
}

func expectVersionSuccessful(mock *MockLambdaMicroVMsClient, version string) {
	mock.EXPECT().
		GetMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageVersionOutput{
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String(version),
			State:        types.MicrovmImageVersionStateSuccessful,
			Status:       types.MicrovmImageVersionStatusActive,
		}, nil)
}

func expectVersionFailed(mock *MockLambdaMicroVMsClient, version string) {
	mock.EXPECT().
		GetMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageVersionOutput{
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String(version),
			State:        types.MicrovmImageVersionStateFailed,
			Status:       types.MicrovmImageVersionStatusActive,
		}, nil)
}

func TestDeploy_Create(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListNotFound(mock)

	mock.EXPECT().
		CreateMicrovmImage(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.CreateMicrovmImageOutput{
			Name:         aws.String("test-microvm"),
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String("1.0"),
			State:        types.MicrovmImageStateCreating,
		}, nil)

	expectVersionSuccessful(mock, "1.0")

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Deploy(context.Background(), &DeployOption{SkipArchive: true, Wait: true}); err != nil {
		t.Fatal(err)
	}
}

func TestDeploy_Update(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		UpdateMicrovmImage(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.UpdateMicrovmImageOutput{
			Name:         aws.String("test-microvm"),
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String("2.0"),
			State:        types.MicrovmImageStateUpdating,
		}, nil)

	expectTagsEmpty(mock)
	expectVersionSuccessful(mock, "2.0")

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Deploy(context.Background(), &DeployOption{SkipArchive: true, Wait: true}); err != nil {
		t.Fatal(err)
	}
}

func TestDeploy_Create_WithTags(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListNotFound(mock)

	mock.EXPECT().
		CreateMicrovmImage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input *lambdamicrovms.CreateMicrovmImageInput, _ ...func(*lambdamicrovms.Options)) (*lambdamicrovms.CreateMicrovmImageOutput, error) {
			if input.Tags == nil {
				t.Error("Tags should be set")
			}
			if input.Tags["env"] != "test" {
				t.Errorf("Tags[env] = %q, want %q", input.Tags["env"], "test")
			}
			if input.Tags["project"] != "lamvms" {
				t.Errorf("Tags[project] = %q, want %q", input.Tags["project"], "lamvms")
			}
			return &lambdamicrovms.CreateMicrovmImageOutput{
				Name:         aws.String("test-microvm-full"),
				ImageArn:     aws.String(testImageARN),
				ImageVersion: aws.String("1.0"),
				State:        types.MicrovmImageStateCreating,
			}, nil
		})

	expectVersionSuccessful(mock, "1.0")

	app := newTestApp(t, mock, "testdata/microvm_full.json")
	if err := app.Deploy(context.Background(), &DeployOption{SkipArchive: true, Wait: true}); err != nil {
		t.Fatal(err)
	}
}

func TestDeploy_Update_AllFieldsMapped(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListMicrovmImages(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImagesOutput{
			Items: []types.MicrovmImageSummary{
				{Name: aws.String("test-microvm-full"), ImageArn: aws.String(testImageARN), State: types.MicrovmImageStateCreated},
			},
		}, nil)
	mock.EXPECT().
		GetMicrovmImage(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageOutput{
			Name:                     aws.String("test-microvm-full"),
			ImageArn:                 aws.String(testImageARN),
			State:                    types.MicrovmImageStateCreated,
			LatestActiveImageVersion: aws.String("1.0"),
		}, nil)

	mock.EXPECT().
		UpdateMicrovmImage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input *lambdamicrovms.UpdateMicrovmImageInput, _ ...func(*lambdamicrovms.Options)) (*lambdamicrovms.UpdateMicrovmImageOutput, error) {
			if aws.ToString(input.ImageIdentifier) != testImageARN {
				t.Errorf("ImageIdentifier = %q, want %q", aws.ToString(input.ImageIdentifier), testImageARN)
			}
			if aws.ToString(input.Description) != "test description" {
				t.Errorf("Description = %q, want %q", aws.ToString(input.Description), "test description")
			}
			if input.EnvironmentVariables == nil || input.EnvironmentVariables["ENV_KEY"] != "env_value" {
				t.Errorf("EnvironmentVariables not mapped: %v", input.EnvironmentVariables)
			}
			if input.Logging == nil {
				t.Error("Logging not mapped")
			}
			if input.CodeArtifact == nil {
				t.Error("CodeArtifact not mapped")
			}
			return &lambdamicrovms.UpdateMicrovmImageOutput{
				Name:         aws.String("test-microvm-full"),
				ImageArn:     aws.String(testImageARN),
				ImageVersion: aws.String("2.0"),
				State:        types.MicrovmImageStateUpdating,
			}, nil
		})

	mock.EXPECT().
		ListTags(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListTagsOutput{
			Tags: map[string]string{},
		}, nil)
	mock.EXPECT().
		TagResource(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.TagResourceOutput{}, nil)

	expectVersionSuccessful(mock, "2.0")

	app := newTestApp(t, mock, "testdata/microvm_full.json")
	if err := app.Deploy(context.Background(), &DeployOption{SkipArchive: true, Wait: true}); err != nil {
		t.Fatal(err)
	}
}

func TestDeploy_CreateFailed(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListNotFound(mock)

	mock.EXPECT().
		CreateMicrovmImage(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.CreateMicrovmImageOutput{
			Name:         aws.String("test-microvm"),
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String("1.0"),
			State:        types.MicrovmImageStateCreating,
		}, nil)

	expectVersionFailed(mock, "1.0")

	mock.EXPECT().
		GetMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageVersionOutput{
			ImageVersion: aws.String("1.0"),
			State:        types.MicrovmImageVersionStateFailed,
		}, nil)

	mock.EXPECT().
		ListMicrovmImageBuilds(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageBuildsOutput{
			Items: []types.MicrovmImageBuildSummary{
				{BuildState: types.BuildStateFailed, Chipset: "GRAVITON", ChipsetGeneration: aws.String("4"), StateReason: aws.String("build error")},
			},
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	err := app.Deploy(context.Background(), &DeployOption{SkipArchive: true, Wait: true})
	if err == nil {
		t.Fatal("expected error on create failure")
	}
}

func TestDeploy_DryRun(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListNotFound(mock)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Deploy(context.Background(), &DeployOption{SkipArchive: true, Wait: true, DryRun: true}); err != nil {
		t.Fatal(err)
	}
}
