package lamvms

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// LogsOption represents options for the logs subcommand.
type LogsOption struct {
	Since         string `help:"From what time to begin displaying logs." default:"10m" json:"since,omitempty"`
	Follow        bool   `help:"Follow new logs." default:"false" json:"follow,omitempty"`
	Format        string `help:"The format to display the logs." default:"detailed" enum:"detailed,short,json" json:"format,omitempty"`
	FilterPattern string `help:"The filter pattern to use." json:"filter_pattern,omitempty"`
}

func microvmLogGroupName(name string) string {
	return fmt.Sprintf("/aws/lambda-microvms/%s", name)
}

// Logs tails CloudWatch logs for the MicroVM image.
func (app *App) Logs(ctx context.Context, opt *LogsOption) error {
	img := app.microvmImage
	name := aws.ToString(img.Name)
	logGroup := microvmLogGroupName(name)

	command := []string{"aws"}
	if app.profile != "" {
		command = append(command, "--profile", app.profile)
	}
	if app.awsConfig.Region != "" {
		command = append(command, "--region", app.awsConfig.Region)
	}
	command = append(command, "logs", "tail", logGroup)

	if opt.Since != "" {
		command = append(command, "--since", opt.Since)
	}
	if opt.Follow {
		command = append(command, "--follow")
	}
	if opt.Format != "" {
		command = append(command, "--format", opt.Format)
	}
	if opt.FilterPattern != "" {
		command = append(command, "--filter-pattern", opt.FilterPattern)
	}

	bin, err := exec.LookPath(command[0])
	if err != nil {
		return fmt.Errorf("aws CLI not found: %w", err)
	}
	slog.Debug("invoking command", "command", strings.Join(command, " "))
	if err := syscall.Exec(bin, command, os.Environ()); err != nil {
		return fmt.Errorf("failed to invoke aws logs tail: %w", err)
	}
	return nil
}
