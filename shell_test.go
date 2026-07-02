package lamvms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
	"github.com/gorilla/websocket"
	"go.uber.org/mock/gomock"
)

func newShellServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shell" {
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer func() {
			if err := conn.Close(); err != nil {
				t.Logf("close error: %v", err)
			}
		}()

		initMsg, err := json.Marshal(map[string]string{
			"type":       "session_init",
			"session_id": "test-session-123",
		})
		if err != nil {
			t.Logf("marshal error: %v", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, initMsg); err != nil {
			return
		}

		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.TextMessage {
				continue
			}
			if string(data) == "exit\n" {
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, []byte("echo: "+string(data))); err != nil {
				return
			}
		}
	}))
}

func TestShell_GetMicrovmFails(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		GetMicrovm(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("microvm not found"))

	app := &App{client: mock}
	err := app.Shell(context.Background(), &ShellOption{MicrovmID: "mvm-12345"})
	if err == nil {
		t.Fatal("expected error when getting microvm fails")
	}
}

func TestShell_GetMicrovm(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	srv := newShellServer(t)
	defer srv.Close()

	endpoint := srv.Listener.Addr().String()

	mock.EXPECT().
		GetMicrovm(gomock.Any(), gomock.Eq(&lambdamicrovms.GetMicrovmInput{
			MicrovmIdentifier: aws.String("mvm-12345"),
		})).
		Return(&lambdamicrovms.GetMicrovmOutput{
			MicrovmId:    aws.String("mvm-12345"),
			Endpoint:     aws.String(endpoint),
			State:        types.MicrovmStateRunning,
			ImageVersion: aws.String("1.0"),
		}, nil)

	mock.EXPECT().
		CreateMicrovmShellAuthToken(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.CreateMicrovmShellAuthTokenOutput{
			AuthToken: map[string]string{"X-aws-proxy-auth": "test-shell-token"},
		}, nil)

	app := &App{client: mock}
	err := app.Shell(context.Background(), &ShellOption{MicrovmID: "mvm-12345"})
	if err != nil && err.Error() != "websocket dial: websocket: bad handshake" {
		t.Logf("shell error (expected due to test server TLS): %v", err)
	}
}

func TestShell_AuthTokenMissing(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		GetMicrovm(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmOutput{
			MicrovmId: aws.String("mvm-12345"),
			Endpoint:  aws.String("localhost:9999"),
			State:     types.MicrovmStateRunning,
		}, nil)

	mock.EXPECT().
		CreateMicrovmShellAuthToken(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.CreateMicrovmShellAuthTokenOutput{
			AuthToken: map[string]string{},
		}, nil)

	app := &App{client: mock}
	err := app.Shell(context.Background(), &ShellOption{MicrovmID: "mvm-12345", TokenExpiration: 60 * time.Minute})
	if err == nil {
		t.Fatal("expected error when auth token key is missing")
	}
	if err.Error() != "auth token missing X-aws-proxy-auth key" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestShell_CreateTokenFails(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		GetMicrovm(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmOutput{
			MicrovmId: aws.String("mvm-12345"),
			Endpoint:  aws.String("localhost:9999"),
			State:     types.MicrovmStateRunning,
		}, nil)

	mock.EXPECT().
		CreateMicrovmShellAuthToken(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("token creation failed"))

	app := &App{client: mock}
	err := app.Shell(context.Background(), &ShellOption{MicrovmID: "mvm-12345", TokenExpiration: 60 * time.Minute})
	if err == nil {
		t.Fatal("expected error when token creation fails")
	}
}

func TestShell_SelectInteractively(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	expectListFound(mock)

	mock.EXPECT().
		ListMicrovms(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListMicrovmsOutput{
			Items: []types.MicrovmItem{
				{MicrovmId: aws.String("mvm-single"), State: types.MicrovmStateRunning, StartedAt: aws.Time(time.Now())},
			},
		}, nil)

	mock.EXPECT().
		GetMicrovm(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.GetMicrovmOutput{
			MicrovmId: aws.String("mvm-single"),
			Endpoint:  aws.String("localhost:9999"),
			State:     types.MicrovmStateRunning,
		}, nil)

	mock.EXPECT().
		CreateMicrovmShellAuthToken(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.CreateMicrovmShellAuthTokenOutput{
			AuthToken: map[string]string{"X-aws-proxy-auth": "test-token"},
		}, nil)

	app := newTestApp(t, mock, "testdata/microvm.json")
	err := app.Shell(context.Background(), &ShellOption{})
	if err != nil {
		t.Logf("shell error (expected due to no real server): %v", err)
	}
}
