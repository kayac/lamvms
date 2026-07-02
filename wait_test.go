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

func TestWait_AlreadyReady(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Wait(context.Background(), &WaitOption{}); err != nil {
		t.Fatal(err)
	}
}

func TestWait_SpecificVersion(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)
	expectVersionSuccessful(mock, "5.0")

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Wait(context.Background(), &WaitOption{Version: "5.0"}); err != nil {
		t.Fatal(err)
	}
}

func TestWait_LatestVersion(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListMicrovmImages(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImagesOutput{
			Items: []types.MicrovmImageSummary{
				{Name: aws.String("test-microvm"), ImageArn: aws.String(testImageARN), State: types.MicrovmImageStateUpdating},
			},
		}, nil)
	mock.EXPECT().
		GetMicrovmImage(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmImageOutput{
			Name:     aws.String("test-microvm"),
			ImageArn: aws.String(testImageARN),
			State:    types.MicrovmImageStateUpdating,
		}, nil)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("2.0"), CreatedAt: aws.Time(now)},
			},
		}, nil)

	expectVersionSuccessful(mock, "2.0")

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Wait(context.Background(), &WaitOption{}); err != nil {
		t.Fatal(err)
	}
}

func TestWait_NotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListNotFound(mock)

	app := newTestApp(t, mock, "testdata/microvm.json")
	err := app.Wait(context.Background(), &WaitOption{})
	if err == nil {
		t.Fatal("expected error when image not found")
	}
}
