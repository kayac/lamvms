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
	// v1.0 ACTIVE, v2.0 FAILED, v3.0 FAILED, v4.0-v8.0 ACTIVE
	// keep=5 → cutoff after v4.0 → v1.0 is after cutoff → deleted
	// v2.0 FAILED, v3.0 FAILED are between v4.0 and v1.0 → also deleted
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

	// v4.0 is the 5th ACTIVE SUCCESSFUL (cutoff), so v1.0, v2.0(FAILED), v3.0(FAILED) are deleted
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
	// v1.0-v5.0 ACTIVE SUCCESSFUL, v6.0 FAILED, v7.0 FAILED
	// keep=5 → cutoff after v1.0 (5th ACTIVE SUCCESSFUL)
	// v6.0 and v7.0 are FAILED but newer than cutoff → not deleted
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

	// No deletions — 5 ACTIVE SUCCESSFUL exist, FAILED v6.0/v7.0 are newer than cutoff
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
	// 11.0 FAILED, 10.0 FAILED, 9.0 SUCCESSFUL INACTIVE, 8.0-5.0 SUCCESSFUL ACTIVE, 4.0 FAILED
	// keep=3 → count ACTIVE+SUCCESSFUL: 8.0(1), 7.0(2), 6.0(3=cutoff)
	// delete: 5.0(ACTIVE+SUCCESSFUL), 9.0(INACTIVE), 4.0(FAILED)
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
