package lamvms

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
	"go.uber.org/mock/gomock"
)

func TestInit(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListMicrovmImages(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImagesOutput{
			Items: []types.MicrovmImageSummary{
				{Name: aws.String("test-image"), ImageArn: aws.String(testImageARN), State: types.MicrovmImageStateCreated},
			},
		}, nil)

	mock.EXPECT().
		GetMicrovmImage(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageOutput{
			Name:                     aws.String("test-image"),
			ImageArn:                 aws.String(testImageARN),
			State:                    types.MicrovmImageStateCreated,
			LatestActiveImageVersion: aws.String("1.0"),
		}, nil)

	mock.EXPECT().
		GetMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageVersionOutput{
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String("1.0"),
			BaseImageArn: aws.String("arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1"),
			BuildRoleArn: aws.String("arn:aws:iam::123456789012:role/BuildRole"),
			CodeArtifact: &types.CodeArtifactMemberUri{Value: "s3://bucket/app.zip"},
			State:        types.MicrovmImageVersionStateSuccessful,
			Status:       types.MicrovmImageVersionStatusActive,
		}, nil)

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "microvm.json")

	code, err := runInit(context.Background(), mock, &InitOption{ImageName: "test-image", Output: outputPath})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("output file is empty")
	}
}

func TestInit_ImageNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListMicrovmImages(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImagesOutput{
			Items: []types.MicrovmImageSummary{},
		}, nil)

	_, err := runInit(context.Background(), mock, &InitOption{ImageName: "nonexistent", Output: "out.json"})
	if err == nil {
		t.Fatal("expected error when image not found")
	}
}

func TestInit_FileExists(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListMicrovmImages(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImagesOutput{
			Items: []types.MicrovmImageSummary{
				{Name: aws.String("test-image"), ImageArn: aws.String(testImageARN), State: types.MicrovmImageStateCreated},
			},
		}, nil)

	mock.EXPECT().
		GetMicrovmImage(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageOutput{
			Name:                     aws.String("test-image"),
			ImageArn:                 aws.String(testImageARN),
			State:                    types.MicrovmImageStateCreated,
			LatestActiveImageVersion: aws.String("1.0"),
		}, nil)

	mock.EXPECT().
		GetMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageVersionOutput{
			ImageArn:     aws.String(testImageARN),
			ImageVersion: aws.String("1.0"),
			BaseImageArn: aws.String("arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1"),
			BuildRoleArn: aws.String("arn:aws:iam::123456789012:role/BuildRole"),
			CodeArtifact: &types.CodeArtifactMemberUri{Value: "s3://bucket/app.zip"},
			State:        types.MicrovmImageVersionStateSuccessful,
			Status:       types.MicrovmImageVersionStatusActive,
		}, nil)

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "microvm.json")
	if err := os.WriteFile(outputPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := runInit(context.Background(), mock, &InitOption{ImageName: "test-image", Output: outputPath})
	if err == nil {
		t.Fatal("expected error when file exists")
	}
}
