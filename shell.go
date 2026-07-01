package lamvms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

const (
	shellPingInterval = 1 * time.Second
	shellPongTimeout  = 3 * time.Second
)

// Shell opens an interactive shell session to a running MicroVM.
func (app *App) Shell(ctx context.Context, opt *ShellOption) error {
	id := opt.MicrovmID
	if id == "" {
		var err error
		id, err = app.selectMicrovmID(ctx, types.MicrovmStateRunning)
		if err != nil {
			return err
		}
	}

	vm, err := app.client.GetMicrovm(ctx, &lambdamicrovms.GetMicrovmInput{
		MicrovmIdentifier: aws.String(id),
	})
	if err != nil {
		return fmt.Errorf("get microvm: %w", err)
	}
	endpoint := aws.ToString(vm.Endpoint)

	slog.Info("creating shell auth token", "id", id)
	tokenOut, err := app.client.CreateMicrovmShellAuthToken(ctx, &lambdamicrovms.CreateMicrovmShellAuthTokenInput{
		MicrovmIdentifier:   aws.String(id),
		ExpirationInMinutes: aws.Int32(expirationMinutes(opt.TokenExpiration)),
	})
	if err != nil {
		return fmt.Errorf("create shell auth token: %w", err)
	}
	token, ok := tokenOut.AuthToken["X-aws-proxy-auth"]
	if !ok {
		return fmt.Errorf("auth token missing X-aws-proxy-auth key")
	}

	wsURL := fmt.Sprintf("wss://%s/shell", endpoint)
	slog.Info("connecting to shell", "url", wsURL)

	header := http.Header{}
	header.Set("X-aws-proxy-auth", token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	var writeMu sync.Mutex
	writeMessage := func(msgType int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(msgType, data)
	}
	closeConn := func() {
		writeMu.Lock()
		defer writeMu.Unlock()
		if err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			slog.Debug("failed to send close message", "error", err)
		}
		if err := conn.Close(); err != nil {
			slog.Debug("failed to close websocket", "error", err)
		}
	}

	_, initMsg, err := conn.ReadMessage()
	if err != nil {
		closeConn()
		return fmt.Errorf("read session_init: %w", err)
	}
	var init struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(initMsg, &init); err != nil {
		closeConn()
		return fmt.Errorf("parse session_init: %w", err)
	}
	slog.Info("shell session started", "session_id", init.SessionID)
	fmt.Fprintln(os.Stderr, "Press Ctrl+D to disconnect.")

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		closeConn()
		return fmt.Errorf("failed to set raw terminal: %w", err)
	}
	restoreTerminal := sync.OnceFunc(func() {
		if err := term.Restore(int(os.Stdin.Fd()), oldState); err != nil {
			slog.Warn("failed to restore terminal", "error", err)
		}
	})
	defer restoreTerminal()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(shellPongTimeout)); err != nil {
			slog.Debug("failed to set read deadline", "error", err)
		}
		return nil
	})
	if err := conn.SetReadDeadline(time.Now().Add(shellPongTimeout)); err != nil {
		slog.Debug("failed to set initial read deadline", "error", err)
	}

	go func() {
		ticker := time.NewTicker(shellPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := writeMessage(websocket.PingMessage, nil); err != nil {
					slog.Debug("ping failed", "error", err)
					cancel()
					return
				}
			}
		}
	}()

	sendResizeFn := func() {
		w, h, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			slog.Debug("failed to get terminal size", "error", err)
			return
		}
		msg, err := json.Marshal(map[string]any{"type": "resize", "cols": w, "rows": h})
		if err != nil {
			slog.Debug("failed to marshal resize", "error", err)
			return
		}
		if err := writeMessage(websocket.TextMessage, msg); err != nil {
			slog.Debug("failed to send resize", "error", err)
		}
	}
	sendResizeFn()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigCh:
				sendResizeFn()
			}
		}
	}()

	go func() {
		defer cancel()
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				slog.Debug("stdin read ended", "error", err)
				return
			}
			for i := 0; i < n; i++ {
				if buf[i] == 0x04 {
					slog.Debug("Ctrl+D, exiting")
					return
				}
			}
			if err := writeMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				slog.Debug("websocket write ended", "error", err)
				return
			}
		}
	}()

	go func() {
		defer cancel()
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				slog.Debug("websocket read ended", "error", err)
				return
			}
			if err := conn.SetReadDeadline(time.Now().Add(shellPongTimeout)); err != nil {
				slog.Debug("failed to set read deadline", "error", err)
			}
			if msgType == websocket.BinaryMessage {
				if _, err := os.Stdout.Write(data); err != nil {
					slog.Debug("stdout write ended", "error", err)
					return
				}
			}
		}
	}()

	<-ctx.Done()
	restoreTerminal()
	closeConn()
	fmt.Fprintln(os.Stderr, "\r\nshell session ended")
	return nil
}
