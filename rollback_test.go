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

func TestRollback(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-2 * time.Hour))},
				{ImageVersion: aws.String("2.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("3.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now)},
			},
		}, nil)

	mock.EXPECT().
		UpdateMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.UpdateMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("3.0"),
			Status:          types.MicrovmImageVersionStatusInactive,
		})).
		Return(&lambdamicrovms.UpdateMicrovmImageVersionOutput{}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Rollback(context.Background(), &RollbackOption{}); err != nil {
		t.Fatal(err)
	}
}

func TestRollback_NotEnoughVersions(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(time.Now())},
			},
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	err := app.Rollback(context.Background(), &RollbackOption{})
	if err == nil {
		t.Fatal("expected error when only 1 active version")
	}
}

func TestRollback_DryRun(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("2.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now)},
			},
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Rollback(context.Background(), &RollbackOption{DryRun: true}); err != nil {
		t.Fatal(err)
	}
}

func TestRollback_SkipsFailedVersions(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-2 * time.Hour))},
				{ImageVersion: aws.String("2.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("3.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateFailed, CreatedAt: aws.Time(now)},
			},
		}, nil)

	mock.EXPECT().
		UpdateMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.UpdateMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("2.0"),
			Status:          types.MicrovmImageVersionStatusInactive,
		})).
		Return(&lambdamicrovms.UpdateMicrovmImageVersionOutput{}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Rollback(context.Background(), &RollbackOption{}); err != nil {
		t.Fatal(err)
	}
}
