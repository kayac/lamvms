package lamvms

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestDeploy_Create_BuildLogsBestEffortWhenAWSCLIMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

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
	if err := app.Deploy(context.Background(), &DeployOption{SkipArchive: true, Wait: true, BuildLogs: true}); err != nil {
		t.Fatal(err)
	}
}

func newFakeAWS(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := "aws"
	if runtime.GOOS == "windows" {
		name = "aws.bat"
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FAKE_AWS_ARGS_FILE\"\nsleep 5\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return filepath.Join(t.TempDir(), "args.txt")
}

func TestStartBuildLogTail_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	app := newTestApp(t, nil, "testdata/microvm.json")
	stop := app.startBuildLogTail(context.Background(), "1.0")
	stop()
}

func TestStartBuildLogTail_BuildsCommand(t *testing.T) {
	argsFile := newFakeAWS(t)
	t.Setenv("FAKE_AWS_ARGS_FILE", argsFile)

	app := newTestApp(t, nil, "testdata/microvm.json")
	app.profile = "test-profile"
	app.awsConfig.Region = "ap-northeast-1"

	stop := app.startBuildLogTail(context.Background(), "1.0")
	defer stop()

	deadline := time.Now().Add(3 * time.Second)
	var got []byte
	for {
		var err error
		got, err = os.ReadFile(argsFile)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("args file was not written in time: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	gotArgs := strings.Split(strings.TrimRight(string(got), "\n"), "\n")
	want := []string{
		"--profile", "test-profile",
		"--region", "ap-northeast-1",
		"logs", "tail", "/aws/lambda-microvms/test-microvm",
		"--follow", "--log-stream-name-prefix", "1.0/",
	}
	if strings.Join(gotArgs, " ") != strings.Join(want, " ") {
		t.Errorf("aws args = %v, want %v", gotArgs, want)
	}
}

func TestStartBuildLogTail_StopDoesNotHang(t *testing.T) {
	newFakeAWS(t)

	app := newTestApp(t, nil, "testdata/microvm.json")
	stop := app.startBuildLogTail(context.Background(), "1.0")

	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stop() did not return in time")
	}
}
