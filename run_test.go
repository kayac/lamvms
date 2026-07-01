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

func TestRun(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		RunMicrovm(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.RunMicrovmOutput{
			MicrovmId:    aws.String("mvm-12345"),
			Endpoint:     aws.String("mvm-12345.lambda-microvm.ap-northeast-1.on.aws"),
			State:        types.MicrovmStatePending,
			ImageVersion: aws.String("1.0"),
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Run(context.Background(), &RunOption{Wait: false, Output: "json"}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_WithRunDef(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		RunMicrovm(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input *lambdamicrovms.RunMicrovmInput, _ ...func(*lambdamicrovms.Options)) (*lambdamicrovms.RunMicrovmOutput, error) {
			if len(input.IngressNetworkConnectors) == 0 {
				t.Error("IngressNetworkConnectors should be set from run-def")
			}
			if input.IdlePolicy == nil {
				t.Error("IdlePolicy should be set from run-def")
			}
			return &lambdamicrovms.RunMicrovmOutput{
				MicrovmId:    aws.String("mvm-67890"),
				Endpoint:     aws.String("mvm-67890.lambda-microvm.ap-northeast-1.on.aws"),
				State:        types.MicrovmStatePending,
				ImageVersion: aws.String("1.0"),
			}, nil
		})

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Run(context.Background(), &RunOption{Wait: false, RunDef: "testdata/run.json", Output: "json"}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_WithAuthToken(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		RunMicrovm(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.RunMicrovmOutput{
			MicrovmId:    aws.String("mvm-12345"),
			Endpoint:     aws.String("mvm-12345.lambda-microvm.ap-northeast-1.on.aws"),
			State:        types.MicrovmStatePending,
			ImageVersion: aws.String("1.0"),
		}, nil)

	mock.EXPECT().
		CreateMicrovmAuthToken(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.CreateMicrovmAuthTokenOutput{
			AuthToken: map[string]string{"X-aws-proxy-auth": "test-token"},
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Run(context.Background(), &RunOption{Wait: false, CreateAuthToken: true, TokenExpiration: 30 * time.Minute, Output: "json"}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_WithAuthToken_MissingTokenInResponse(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		RunMicrovm(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.RunMicrovmOutput{
			MicrovmId:    aws.String("mvm-12345"),
			Endpoint:     aws.String("mvm-12345.lambda-microvm.ap-northeast-1.on.aws"),
			State:        types.MicrovmStatePending,
			ImageVersion: aws.String("1.0"),
		}, nil)

	mock.EXPECT().
		CreateMicrovmAuthToken(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.CreateMicrovmAuthTokenOutput{
			AuthToken: map[string]string{},
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	err := app.Run(context.Background(), &RunOption{Wait: false, CreateAuthToken: true, TokenExpiration: 30 * time.Minute, Output: "json"})
	if err == nil {
		t.Fatal("expected error when response has no X-aws-proxy-auth token")
	}
}

func TestRun_ImageNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListNotFound(mock)

	app := newTestApp(t, mock, "testdata/microvm.json")
	err := app.Run(context.Background(), &RunOption{Wait: false})
	if err == nil {
		t.Fatal("expected error when image not found")
	}
}

func TestSuspend(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		SuspendMicrovm(gomock.Any(), gomock.Eq(&lambdamicrovms.SuspendMicrovmInput{
			MicrovmIdentifier: aws.String("mvm-12345"),
		})).
		Return(&lambdamicrovms.SuspendMicrovmOutput{}, nil)

	app := &App{client: mock}
	if err := app.Suspend(context.Background(), &SuspendOption{MicrovmID: "mvm-12345"}); err != nil {
		t.Fatal(err)
	}
}

func TestResume(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ResumeMicrovm(gomock.Any(), gomock.Eq(&lambdamicrovms.ResumeMicrovmInput{
			MicrovmIdentifier: aws.String("mvm-12345"),
		})).
		Return(&lambdamicrovms.ResumeMicrovmOutput{}, nil)

	app := &App{client: mock}
	if err := app.Resume(context.Background(), &ResumeOption{MicrovmID: "mvm-12345"}); err != nil {
		t.Fatal(err)
	}
}

func TestTerminate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		TerminateMicrovm(gomock.Any(), gomock.Eq(&lambdamicrovms.TerminateMicrovmInput{
			MicrovmIdentifier: aws.String("mvm-12345"),
		})).
		Return(&lambdamicrovms.TerminateMicrovmOutput{}, nil)

	app := &App{client: mock}
	if err := app.Terminate(context.Background(), &TerminateOption{MicrovmID: "mvm-12345"}); err != nil {
		t.Fatal(err)
	}
}
