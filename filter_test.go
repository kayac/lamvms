package lamvms

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
	"go.uber.org/mock/gomock"
)

func TestRunExternalFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		requires []string
		command  string
		input    string
		want     string
		wantErr  bool
	}{
		{
			name:     "直接実行パス: stdin がコマンドに渡り末尾改行が削られる",
			requires: []string{"cat"},
			command:  "cat",
			input:    "mvm-1\tRUNNING\nmvm-2\tPENDING\n",
			want:     "mvm-1\tRUNNING\nmvm-2\tPENDING",
		},
		{
			name:     "sh -c パス: スペースを含むコマンドはシェル経由で実行される",
			requires: []string{"sh", "head"},
			command:  "head -n1",
			input:    "mvm-1\tRUNNING\nmvm-2\tPENDING\n",
			want:     "mvm-1\tRUNNING",
		},
		{
			name:     "sh -c パス: パイプを含むコマンド",
			requires: []string{"sh", "grep"},
			command:  "grep PENDING | head -n1",
			input:    "mvm-1\tRUNNING\nmvm-2\tPENDING\n",
			want:     "mvm-2\tPENDING",
		},
		{
			name:    "直接実行パス: 存在しないコマンドはエラー",
			command: "lamvms-no-such-filter-command",
			input:   "mvm-1\n",
			wantErr: true,
		},
		{
			name:     "sh -c パス: 非ゼロ終了はエラー",
			requires: []string{"sh"},
			command:  "exit 3",
			input:    "mvm-1\n",
			wantErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, bin := range tc.requires {
				if _, err := exec.LookPath(bin); err != nil {
					t.Skipf("%s not found", bin)
				}
			}
			got, err := runExternalFilter(context.Background(), tc.command, strings.NewReader(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("runExternalFilter(%q) = %q, want error", tc.command, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("runExternalFilter(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}

func TestRunExternalFilter_ContextCanceled(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat not found")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if got, err := runExternalFilter(ctx, "cat", strings.NewReader("mvm-1\n")); err == nil {
		t.Fatalf("runExternalFilter with canceled context = %q, want error", got)
	}
}

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
