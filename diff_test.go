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
