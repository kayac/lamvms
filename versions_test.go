package lamvms

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
	"github.com/aws/smithy-go"
	"go.uber.org/mock/gomock"
)

func TestDeleteOldVersions_KeepsN(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-3 * time.Hour))},
				{ImageVersion: aws.String("2.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-2 * time.Hour))},
				{ImageVersion: aws.String("3.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("4.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now)},
			},
		}, nil)

	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("2.0"),
		})).
		Return(&lambdamicrovms.DeleteMicrovmImageVersionOutput{}, nil)

	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("1.0"),
		})).
		Return(&lambdamicrovms.DeleteMicrovmImageVersionOutput{}, nil)

	app := &App{client: mock}
	if err := app.deleteOldVersions(context.Background(), testImageARN, 2); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOldVersions_NothingToDelete(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now)},
			},
		}, nil)

	app := &App{client: mock}
	if err := app.deleteOldVersions(context.Background(), testImageARN, 3); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOldVersions_FailedBeforeActiveNotDeleted(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-7 * time.Hour))},
				{ImageVersion: aws.String("2.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateFailed, CreatedAt: aws.Time(now.Add(-6 * time.Hour))},
				{ImageVersion: aws.String("3.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateFailed, CreatedAt: aws.Time(now.Add(-5 * time.Hour))},
				{ImageVersion: aws.String("4.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-4 * time.Hour))},
				{ImageVersion: aws.String("5.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-3 * time.Hour))},
				{ImageVersion: aws.String("6.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-2 * time.Hour))},
				{ImageVersion: aws.String("7.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("8.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now)},
			},
		}, nil)

	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("3.0"),
		})).
		Return(&lambdamicrovms.DeleteMicrovmImageVersionOutput{}, nil)

	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("2.0"),
		})).
		Return(&lambdamicrovms.DeleteMicrovmImageVersionOutput{}, nil)

	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("1.0"),
		})).
		Return(&lambdamicrovms.DeleteMicrovmImageVersionOutput{}, nil)

	app := &App{client: mock}
	if err := app.deleteOldVersions(context.Background(), testImageARN, 5); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOldVersions_RecentFailedNotDeleted(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-6 * time.Hour))},
				{ImageVersion: aws.String("2.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-5 * time.Hour))},
				{ImageVersion: aws.String("3.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-4 * time.Hour))},
				{ImageVersion: aws.String("4.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-3 * time.Hour))},
				{ImageVersion: aws.String("5.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-2 * time.Hour))},
				{ImageVersion: aws.String("6.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateFailed, CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("7.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateFailed, CreatedAt: aws.Time(now)},
			},
		}, nil)

	app := &App{client: mock}
	if err := app.deleteOldVersions(context.Background(), testImageARN, 5); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOldVersions_MixedStates(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("4.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateFailed, CreatedAt: aws.Time(now.Add(-7 * time.Hour))},
				{ImageVersion: aws.String("5.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-6 * time.Hour))},
				{ImageVersion: aws.String("6.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-5 * time.Hour))},
				{ImageVersion: aws.String("7.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-4 * time.Hour))},
				{ImageVersion: aws.String("8.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-3 * time.Hour))},
				{ImageVersion: aws.String("9.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusInactive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-2 * time.Hour))},
				{ImageVersion: aws.String("10.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateFailed, CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("11.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateFailed, CreatedAt: aws.Time(now)},
			},
		}, nil)

	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("5.0"),
		})).
		Return(&lambdamicrovms.DeleteMicrovmImageVersionOutput{}, nil)

	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("4.0"),
		})).
		Return(&lambdamicrovms.DeleteMicrovmImageVersionOutput{}, nil)

	app := &App{client: mock}
	if err := app.deleteOldVersions(context.Background(), testImageARN, 3); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOldVersions_ImageBusySkipsRemaining(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-3 * time.Hour))},
				{ImageVersion: aws.String("2.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-2 * time.Hour))},
				{ImageVersion: aws.String("3.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("4.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now)},
			},
		}, nil)

	// 1件目 (3.0) は成功させ、2件目 (2.0) で busy になる部分成功パスを検証する。
	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("3.0"),
		})).
		Return(&lambdamicrovms.DeleteMicrovmImageVersionOutput{}, nil)

	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageVersionInput{
			ImageIdentifier: aws.String(testImageARN),
			ImageVersion:    aws.String("2.0"),
		})).
		Return(nil, &smithy.OperationError{
			ServiceID:     "Lambda Microvms",
			OperationName: "DeleteMicrovmImageVersion",
			Err:           &types.ConflictException{Message: aws.String("MicroVM Image is already in state: UPDATING")},
		})

	app := &App{client: mock}
	if err := app.deleteOldVersions(context.Background(), testImageARN, 1); err != nil {
		t.Fatalf("expected image-busy conflict to be swallowed, got error: %v", err)
	}
}

func TestDeleteOldVersions_OtherDeleteErrorPropagates(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	now := time.Now()
	mock.EXPECT().
		ListMicrovmImageVersions(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmImageVersionsOutput{
			Items: []types.MicrovmImageVersionSummary{
				{ImageVersion: aws.String("1.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now.Add(-1 * time.Hour))},
				{ImageVersion: aws.String("2.0"), ImageArn: aws.String(testImageARN), Status: types.MicrovmImageVersionStatusActive, State: types.MicrovmImageVersionStateSuccessful, CreatedAt: aws.Time(now)},
			},
		}, nil)

	wantErr := errors.New("network error")
	mock.EXPECT().
		DeleteMicrovmImageVersion(gomock.Any(), gomock.Any()).
		Return(nil, wantErr)

	app := &App{client: mock}
	err := app.deleteOldVersions(context.Background(), testImageARN, 1)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error to wrap %v, got %v", wantErr, err)
	}
}
