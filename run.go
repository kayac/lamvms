package lamvms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
)

// Run starts a new MicroVM from the configured image.
func (app *App) Run(ctx context.Context, opt *RunOption) error {
	img := app.microvmImage
	name := aws.ToString(img.Name)

	existing, err := app.findMicrovmImageByName(ctx, name)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("microvm image %q not found", name)
	}

	input := &lambdamicrovms.RunMicrovmInput{
		ImageIdentifier: existing.ImageArn,
	}

	runDefPath := app.resolveRunDefPath(opt.RunDef)
	if runDefPath != "" {
		rc, err := app.loader.LoadRunConfig(runDefPath)
		if err != nil {
			return err
		}
		rcInput := lambdamicrovms.RunMicrovmInput(*rc)
		rcInput.ImageIdentifier = existing.ImageArn
		input = &rcInput
	}

	if opt.ImageVersion != "" {
		input.ImageVersion = aws.String(opt.ImageVersion)
	}
	if opt.ExecutionRoleArn != "" {
		input.ExecutionRoleArn = aws.String(opt.ExecutionRoleArn)
	}
	if opt.MaximumDurationInSeconds > 0 {
		input.MaximumDurationInSeconds = aws.Int32(opt.MaximumDurationInSeconds)
	}
	if opt.RunHookPayload != "" {
		input.RunHookPayload = aws.String(opt.RunHookPayload)
	}

	slog.Info("running microvm", "name", name)
	out, err := app.client.RunMicrovm(ctx, input)
	if err != nil {
		return fmt.Errorf("run microvm: %w", err)
	}

	microvmID := aws.ToString(out.MicrovmId)
	endpoint := aws.ToString(out.Endpoint)
	state := string(out.State)
	imageVersion := aws.ToString(out.ImageVersion)

	if opt.Wait {
		got, err := app.waitForMicrovmRunning(ctx, microvmID)
		if err != nil {
			return err
		}
		state = string(got.State)
	}

	result := map[string]any{
		"microvmId":    microvmID,
		"endpoint":     endpoint,
		"state":        state,
		"imageVersion": imageVersion,
	}

	if opt.CreateAuthToken {
		token, err := app.createAuthToken(ctx, microvmID, opt.TokenExpiration)
		if err != nil {
			return err
		}
		result["authToken"] = token
	}

	return printRunOutput(result, opt.Output)
}

// Suspend suspends a running MicroVM.
func (app *App) Suspend(ctx context.Context, opt *SuspendOption) error {
	id := opt.MicrovmID
	if id == "" {
		var err error
		id, err = app.selectMicrovmID(ctx, types.MicrovmStateRunning)
		if err != nil {
			return err
		}
	}
	slog.Info("suspending microvm", "id", id)
	_, err := app.client.SuspendMicrovm(ctx, &lambdamicrovms.SuspendMicrovmInput{
		MicrovmIdentifier: aws.String(id),
	})
	if err != nil {
		return fmt.Errorf("suspend microvm: %w", err)
	}
	slog.Info("microvm suspended", "id", id)
	return nil
}

// Resume resumes a suspended MicroVM.
func (app *App) Resume(ctx context.Context, opt *ResumeOption) error {
	id := opt.MicrovmID
	if id == "" {
		var err error
		id, err = app.selectMicrovmID(ctx, types.MicrovmStateSuspended)
		if err != nil {
			return err
		}
	}
	slog.Info("resuming microvm", "id", id)
	_, err := app.client.ResumeMicrovm(ctx, &lambdamicrovms.ResumeMicrovmInput{
		MicrovmIdentifier: aws.String(id),
	})
	if err != nil {
		return fmt.Errorf("resume microvm: %w", err)
	}
	slog.Info("microvm resumed", "id", id)

	if opt.CreateAuthToken {
		token, err := app.createAuthToken(ctx, id, opt.TokenExpiration)
		if err != nil {
			return err
		}
		result := map[string]any{
			"microvmId": id,
			"authToken": token,
		}
		return printRunOutput(result, opt.Output)
	}
	return nil
}

// Terminate terminates a MicroVM.
func (app *App) Terminate(ctx context.Context, opt *TerminateOption) error {
	id := opt.MicrovmID
	if id == "" {
		var err error
		id, err = app.selectMicrovmID(ctx, types.MicrovmStateRunning, types.MicrovmStateSuspended)
		if err != nil {
			return err
		}
	}
	slog.Info("terminating microvm", "id", id)
	_, err := app.client.TerminateMicrovm(ctx, &lambdamicrovms.TerminateMicrovmInput{
		MicrovmIdentifier: aws.String(id),
	})
	if err != nil {
		return fmt.Errorf("terminate microvm: %w", err)
	}
	slog.Info("microvm terminated", "id", id)
	return nil
}

func expirationMinutes(d time.Duration) int32 {
	return max(int32(math.Ceil(d.Minutes())), 1)
}

func (app *App) createAuthToken(ctx context.Context, microvmID string, expiration time.Duration) (string, error) {
	minutes := expirationMinutes(expiration)
	out, err := app.client.CreateMicrovmAuthToken(ctx, &lambdamicrovms.CreateMicrovmAuthTokenInput{
		MicrovmIdentifier:   aws.String(microvmID),
		ExpirationInMinutes: aws.Int32(minutes),
		AllowedPorts: []types.PortSpecification{
			&types.PortSpecificationMemberAllPorts{Value: types.Unit{}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("create auth token: %w", err)
	}
	token, ok := out.AuthToken["X-aws-proxy-auth"]
	if !ok {
		return "", fmt.Errorf("create auth token: response did not include an X-aws-proxy-auth token")
	}
	return token, nil
}

var defaultRunDefFiles = []string{
	"run.jsonnet",
	"run.json",
}

func (app *App) resolveRunDefPath(explicit string) string {
	if explicit != "" {
		if filepath.IsAbs(explicit) {
			return explicit
		}
		microvmDir := filepath.Dir(app.microvmFilePath)
		candidate := filepath.Join(microvmDir, explicit)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
		return ""
	}

	microvmDir := filepath.Dir(app.microvmFilePath)
	for _, name := range defaultRunDefFiles {
		candidate := filepath.Join(microvmDir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	for _, name := range defaultRunDefFiles {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return ""
}

func (app *App) waitForMicrovmRunning(ctx context.Context, microvmID string) (*lambdamicrovms.GetMicrovmOutput, error) {
	slog.Info("waiting for microvm to be running", "id", microvmID)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitInterval):
		}

		out, err := app.client.GetMicrovm(ctx, &lambdamicrovms.GetMicrovmInput{
			MicrovmIdentifier: aws.String(microvmID),
		})
		if err != nil {
			return nil, fmt.Errorf("get microvm: %w", err)
		}

		slog.Debug("microvm state", "id", microvmID, "state", out.State)

		switch out.State {
		case types.MicrovmStateRunning:
			slog.Info("microvm is running", "id", microvmID)
			return out, nil
		case types.MicrovmStateTerminating, types.MicrovmStateTerminated:
			return nil, fmt.Errorf("microvm terminated unexpectedly: state=%s reason=%s", out.State, aws.ToString(out.StateReason))
		}
	}
}

func printRunOutput(data map[string]any, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	default:
		order := []string{"microvmId", "endpoint", "state", "imageVersion", "authToken"}
		for _, k := range order {
			v, ok := data[k]
			if !ok {
				continue
			}
			if _, err := fmt.Fprintf(os.Stdout, "%s\t%v\n", k, v); err != nil {
				return err
			}
		}
		if token, ok := data["authToken"]; ok {
			endpoint := data["endpoint"]
			if _, err := fmt.Fprintf(os.Stdout, "\ncurl https://%s/ -H 'X-aws-proxy-auth: %s'\n", endpoint, token); err != nil {
				return err
			}
		}
		return nil
	}
}
