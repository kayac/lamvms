package lamvms

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
	"go.uber.org/mock/gomock"
)

func TestSelectMicrovmID_NilStartedAt(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("head"); err != nil {
		t.Skip("head not found")
	}

	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		ListMicrovms(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmsOutput{
			Items: []types.MicrovmItem{
				{MicrovmId: aws.String("mvm-1"), State: types.MicrovmStatePending, StartedAt: nil},
				{MicrovmId: aws.String("mvm-2"), State: types.MicrovmStateRunning, StartedAt: aws.Time(time.Now())},
			},
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	app.filterCommand = "head -n1"

	id, err := app.selectMicrovmID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if id != "mvm-1" {
		t.Errorf("selectMicrovmID = %q, want %q", id, "mvm-1")
	}
}
