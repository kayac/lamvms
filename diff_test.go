package lamvms

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
	"go.uber.org/mock/gomock"
)

func TestDiff_NewImage(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListNotFound(mock)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Diff(context.Background(), &DiffOption{}); err != nil {
		t.Fatal(err)
	}
}

func TestDiff_NoChanges(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		GetMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageVersionOutput{
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String("1.0"),
			BaseImageArn: aws.String("arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1"),
			BuildRoleArn: aws.String("arn:aws:iam::123456789012:role/TestBuildRole"),
			CodeArtifact: &types.CodeArtifactMemberUri{Value: "s3://test-bucket/artifact.zip"},
			State:        types.MicrovmImageVersionStateSuccessful,
			Status:       types.MicrovmImageVersionStatusActive,
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Diff(context.Background(), &DiffOption{}); err != nil {
		t.Fatal(err)
	}
}

func TestDiff_HasChanges(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		GetMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageVersionOutput{
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String("1.0"),
			BaseImageArn: aws.String("arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1"),
			BuildRoleArn: aws.String("arn:aws:iam::123456789012:role/DifferentRole"),
			CodeArtifact: &types.CodeArtifactMemberUri{Value: "s3://test-bucket/artifact.zip"},
			State:        types.MicrovmImageVersionStateSuccessful,
			Status:       types.MicrovmImageVersionStatusActive,
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	err := app.Diff(context.Background(), &DiffOption{})
	if err != nil {
		t.Fatal("expected no error without --exit-code")
	}
}

func TestDiff_HasChanges_ExitCode(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		GetMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageVersionOutput{
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String("1.0"),
			BaseImageArn: aws.String("arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1"),
			BuildRoleArn: aws.String("arn:aws:iam::123456789012:role/DifferentRole"),
			CodeArtifact: &types.CodeArtifactMemberUri{Value: "s3://test-bucket/artifact.zip"},
			State:        types.MicrovmImageVersionStateSuccessful,
			Status:       types.MicrovmImageVersionStatusActive,
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	err := app.Diff(context.Background(), &DiffOption{ExitCode: true})
	if err != ErrDiff {
		t.Fatalf("expected ErrDiff, got %v", err)
	}
}

func TestDiff_NoChanges_AllFields(t *testing.T) {
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
		GetMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageVersionOutput{
			ImageArn:             aws.String(testImageARN),
			ImageVersion:         aws.String("1.0"),
			BaseImageArn:         aws.String("arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1"),
			BuildRoleArn:         aws.String("arn:aws:iam::123456789012:role/TestBuildRole"),
			CodeArtifact:         &types.CodeArtifactMemberUri{Value: "s3://test-bucket/artifact.zip"},
			Description:          aws.String("test description"),
			EnvironmentVariables: map[string]string{"ENV_KEY": "env_value"},
			Logging: &types.LoggingMemberCloudWatch{
				Value: types.CloudWatchLogging{LogGroup: aws.String("/aws/lambda/microvms/test")},
			},
			Tags:                    map[string]string{"env": "test", "project": "lamvms"},
			BaseImageVersion:        aws.String("1"),
			CpuConfigurations:       []types.CpuConfiguration{{Architecture: types.ArchitectureArm64}},
			EgressNetworkConnectors: []string{"arn:aws:lambda:ap-northeast-1:aws:network-connector:aws-network-connector:INTERNET_EGRESS"},
			State:                   types.MicrovmImageVersionStateSuccessful,
			Status:                  types.MicrovmImageVersionStatusActive,
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm_full.json")
	err := app.Diff(context.Background(), &DiffOption{ExitCode: true})
	if err != nil {
		t.Fatalf("expected no diff when all fields match, got: %v", err)
	}
}

func TestDiff_ImageNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListNotFound(mock)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Diff(context.Background(), &DiffOption{}); err != nil {
		t.Fatal(err)
	}
}
