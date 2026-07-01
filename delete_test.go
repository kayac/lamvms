package lamvms

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"go.uber.org/mock/gomock"
)

func TestDelete(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		DeleteMicrovmImage(gomock.Any(), gomock.Eq(&lambdamicrovms.DeleteMicrovmImageInput{
			ImageIdentifier: aws.String(testImageARN),
		})).
		Return(&lambdamicrovms.DeleteMicrovmImageOutput{}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Delete(context.Background(), &DeleteOption{}); err != nil {
		t.Fatal(err)
	}
}

func TestDelete_DryRun(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	app := newTestApp(t, mock, "testdata/microvm.json")
	if err := app.Delete(context.Background(), &DeleteOption{DryRun: true}); err != nil {
		t.Fatal(err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListNotFound(mock)

	app := newTestApp(t, mock, "testdata/microvm.json")
	err := app.Delete(context.Background(), &DeleteOption{})
	if err == nil {
		t.Fatal("expected error when image not found")
	}
}
