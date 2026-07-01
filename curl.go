package lamvms

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
)

// Curl sends a request to a running MicroVM via curl.
func (app *App) Curl(ctx context.Context, opt *CurlOption) error {
	parsedPath, err := url.Parse(opt.Path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if parsedPath.Scheme != "" || parsedPath.Host != "" {
		return fmt.Errorf("path must be a relative path, not a full URL: %q", opt.Path)
	}

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

	token, err := app.createAuthToken(ctx, id, opt.TokenExpiration)
	if err != nil {
		return err
	}

	u := url.URL{Scheme: "https", Host: endpoint}
	u = *u.JoinPath(opt.Path)

	command := []string{"--config", "-", u.String()}
	if opt.Port > 0 {
		command = append(command, "-H", fmt.Sprintf("X-aws-proxy-port: %d", opt.Port))
	}
	command = append(command, opt.Args...)

	slog.Debug("invoking curl", "url", u.String(), "args", strings.Join(opt.Args, " "))
	return execCurl(ctx, command, token)
}

func execCurl(ctx context.Context, args []string, token string) error {
	bin, err := exec.LookPath("curl")
	if err != nil {
		return fmt.Errorf("curl not found: %w", err)
	}

	config := fmt.Sprintf("header = \"X-aws-proxy-auth: %s\"\n", token)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(config)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
