package lamvms

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms/types"
)

func (app *App) selectMicrovmID(ctx context.Context, stateFilter ...types.MicrovmState) (string, error) {
	existing, err := app.findMicrovmImageByName(ctx, aws.ToString(app.microvmImage.Name))
	if err != nil {
		return "", err
	}
	if existing == nil {
		return "", fmt.Errorf("microvm image %q not found", aws.ToString(app.microvmImage.Name))
	}

	items, err := app.listMicrovms(ctx, aws.ToString(existing.ImageArn), stateFilter)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no microvms found")
	}
	if len(items) == 1 {
		return aws.ToString(items[0].MicrovmId), nil
	}

	var buf bytes.Buffer
	for _, item := range items {
		fmt.Fprintf(&buf, "%s\t%s\t%s\n",
			aws.ToString(item.MicrovmId),
			item.State,
			item.StartedAt.Format("2006-01-02T15:04:05"),
		)
	}

	selected, err := app.runFilter(ctx, &buf, "MicroVM ID")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(selected)
	if len(fields) == 0 {
		return "", fmt.Errorf("no MicroVM ID selected")
	}
	return fields[0], nil
}

func (app *App) listMicrovms(ctx context.Context, imageARN string, stateFilter []types.MicrovmState) ([]types.MicrovmItem, error) {
	var items []types.MicrovmItem
	var nextToken *string
	for {
		out, err := app.client.ListMicrovms(ctx, &lambdamicrovms.ListMicrovmsInput{
			ImageIdentifier: aws.String(imageARN),
			NextToken:       nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list microvms: %w", err)
		}
		for _, item := range out.Items {
			if len(stateFilter) == 0 || containsState(stateFilter, item.State) {
				items = append(items, item)
			}
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return items, nil
}

func containsState(states []types.MicrovmState, s types.MicrovmState) bool {
	for _, st := range states {
		if st == s {
			return true
		}
	}
	return false
}

func (app *App) runFilter(ctx context.Context, src io.Reader, title string) (string, error) {
	if app.filterCommand != "" {
		return runExternalFilter(ctx, app.filterCommand, src)
	}
	return runInternalFilter(src, title)
}

func runExternalFilter(ctx context.Context, command string, src io.Reader) (string, error) {
	var cmd *exec.Cmd
	if strings.Contains(command, " ") {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	} else {
		cmd = exec.CommandContext(ctx, command)
	}
	cmd.Stderr = os.Stderr
	p, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	go func() {
		if _, err := io.Copy(p, src); err != nil {
			return
		}
		if err := p.Close(); err != nil {
			slog.Debug("failed to close stdin pipe", "error", err)
		}
	}()
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute filter command: %w", err)
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

func runInternalFilter(src io.Reader, title string) (string, error) {
	var items []string
	s := bufio.NewScanner(src)
	for s.Scan() {
		line := s.Text()
		fmt.Fprintln(os.Stderr, line)
		items = append(items, line)
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no items to select")
	}

	fmt.Fprintf(os.Stderr, "Enter %s: ", title)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", fmt.Errorf("no input")
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return "", fmt.Errorf("no input")
	}

	for _, item := range items {
		if strings.HasPrefix(item, input) {
			return item, nil
		}
	}
	return "", fmt.Errorf("no match for %q", input)
}
