package lamvms

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/fujiwara/sloghandler"
	"github.com/kayac/lamvms/skillscmd"
)

// Option holds global flags shared across all subcommands.
type Option struct {
	Microvm   string `help:"Path to microvm definition file." name:"microvm" env:"LAMVMS_MICROVM" json:"microvm,omitempty"`
	LogLevel  string `help:"Log level (debug, info, warn, error)." default:"info" enum:",debug,info,warn,error" env:"LAMVMS_LOGLEVEL" json:"log_level"`
	LogFormat string `help:"Log format (text, json)." default:"text" enum:",text,json" env:"LAMVMS_LOGFORMAT" json:"log_format"`
	Color     bool   `help:"Enable colored output." default:"true" env:"LAMVMS_COLOR" negatable:"" json:"color,omitempty"`

	FilterCommand string            `help:"Filter command for interactive selection (e.g. peco, fzf)." env:"LAMVMS_FILTER_COMMAND" json:"filter_command,omitempty"`
	Region        *string           `help:"AWS region." env:"AWS_REGION" json:"region,omitempty"`
	Profile       *string           `help:"AWS credential profile name." env:"AWS_PROFILE" json:"profile,omitempty"`
	Endpoint      *string           `help:"AWS API endpoint." env:"AWS_ENDPOINT_URL" json:"endpoint,omitempty"`
	Envfile       []string          `help:"Environment files." env:"LAMVMS_ENVFILE" json:"envfile,omitempty"`
	ExtStr        map[string]string `help:"Jsonnet external variables." short:"V" json:"ext_str,omitempty"`
	ExtCode       map[string]string `help:"Jsonnet external code." name:"ext-code" json:"ext_code,omitempty"`
}

// CLIOptions combines global options with subcommand definitions.
type CLIOptions struct {
	Option

	Init      *InitOption         `cmd:"init" help:"Initialize a microvm definition from an existing image." json:"init,omitempty"`
	Deploy    *DeployOption       `cmd:"deploy" help:"Deploy a MicroVM image." json:"deploy,omitempty"`
	Diff      *DiffOption         `cmd:"diff" help:"Show diff between local and deployed configuration." json:"diff,omitempty"`
	Wait      *WaitOption         `cmd:"wait" help:"Wait for a MicroVM image to be ready." json:"wait,omitempty"`
	Rollback  *RollbackOption     `cmd:"rollback" help:"Rollback to the previous active version." json:"rollback,omitempty"`
	Run       *RunOption          `cmd:"run" help:"Run a new MicroVM." json:"run,omitempty"`
	Suspend   *SuspendOption      `cmd:"suspend" help:"Suspend a running MicroVM." json:"suspend,omitempty"`
	Resume    *ResumeOption       `cmd:"resume" help:"Resume a suspended MicroVM." json:"resume,omitempty"`
	Terminate *TerminateOption    `cmd:"terminate" help:"Terminate a MicroVM." json:"terminate,omitempty"`
	Shell     *ShellOption        `cmd:"shell" help:"Open a shell session to a running MicroVM." json:"shell,omitempty"`
	Curl      *CurlOption         `cmd:"curl" help:"Send a request to a running MicroVM via curl." json:"curl,omitempty"`
	Delete    *DeleteOption       `cmd:"delete" help:"Delete a MicroVM image." json:"delete,omitempty"`
	Logs      *LogsOption         `cmd:"logs" help:"Show CloudWatch logs of a MicroVM image." json:"logs,omitempty"`
	Skills    *skillscmd.Commands `cmd:"skills" help:"Manage agent skills." json:"-"`

	Version struct{} `cmd:"version" help:"Show version." json:"-"`
}

// InitOption represents options for the init subcommand.
type InitOption struct {
	ImageName      string `help:"Name of the existing MicroVM image." required:"true" name:"image-name" json:"image_name,omitempty"`
	Output         string `help:"Output file path." default:"microvm.json" json:"output,omitempty"`
	Jsonnet        bool   `help:"Output as .jsonnet format." default:"false" json:"jsonnet,omitempty"`
	ForceOverwrite bool   `help:"Overwrite existing files without prompting." default:"false" json:"force_overwrite,omitempty"`
}

// DiffOption represents options for the diff subcommand.
type DiffOption struct {
	ExitCode bool `help:"Exit with code 2 if there are differences." default:"false" json:"exit_code,omitempty"`
}

// DeleteOption represents options for the delete subcommand.
type DeleteOption struct {
	DryRun bool `help:"Dry run." default:"false" json:"dry_run,omitempty"`
}

// DeployOption represents options for the deploy subcommand.
type DeployOption struct {
	Src          string `help:"Source directory to archive and upload. Defaults to the directory of the microvm definition file." json:"src,omitempty"`
	SkipArchive  bool   `help:"Skip creating and uploading zip archive." default:"false" json:"skip_archive,omitempty"`
	Wait         bool   `help:"Wait for the image build to complete." default:"true" negatable:"" json:"wait,omitempty"`
	KeepVersions int    `help:"Number of latest versions to keep. Older versions will be deleted." default:"0" json:"keep_versions,omitempty"`
	DryRun       bool   `help:"Dry run." default:"false" json:"dry_run,omitempty"`
}

// WaitOption represents options for the wait subcommand.
type WaitOption struct {
	Version      string `help:"Image version to wait for. Defaults to the latest version." json:"version,omitempty"`
	KeepVersions int    `help:"Number of latest versions to keep. Older versions will be deleted." default:"0" json:"keep_versions,omitempty"`
}

// RollbackOption represents options for the rollback subcommand.
type RollbackOption struct {
	DryRun bool `help:"Dry run." default:"false" json:"dry_run,omitempty"`
}

// RunOption represents options for the run subcommand.
type RunOption struct {
	RunDef                   string        `help:"Path to run definition file (run.jsonnet or run.json)." name:"run-def" json:"run_def,omitempty"`
	ImageVersion             string        `help:"Image version to run." name:"image-version" json:"image_version,omitempty"`
	ExecutionRoleArn         string        `help:"IAM role ARN for runtime permissions." name:"execution-role-arn" json:"execution_role_arn,omitempty"`
	MaximumDurationInSeconds int32         `help:"Maximum duration in seconds (1-28800)." name:"max-duration" json:"maximum_duration_in_seconds,omitempty"`
	RunHookPayload           string        `help:"Payload for /run lifecycle hook (max 16KB)." name:"run-hook-payload" json:"run_hook_payload,omitempty"`
	Wait                     bool          `help:"Wait for the MicroVM to be running." default:"true" negatable:"" json:"wait,omitempty"`
	CreateAuthToken          bool          `help:"Create an auth token after run." default:"false" json:"create_auth_token,omitempty"`
	TokenExpiration          time.Duration `help:"Auth token expiration duration." default:"30m" name:"token-expiration" json:"token_expiration,omitempty"`
	Output                   string        `help:"Output format." default:"text" enum:"text,json" json:"output,omitempty"`
}

// SuspendOption represents options for the suspend subcommand.
type SuspendOption struct {
	MicrovmID string `arg:"" optional:"" help:"MicroVM ID to suspend. If omitted, select interactively."`
}

// ResumeOption represents options for the resume subcommand.
type ResumeOption struct {
	MicrovmID       string        `arg:"" optional:"" help:"MicroVM ID to resume. If omitted, select interactively."`
	CreateAuthToken bool          `help:"Create an auth token after resume." default:"false" json:"create_auth_token,omitempty"`
	TokenExpiration time.Duration `help:"Auth token expiration duration." default:"30m" name:"token-expiration" json:"token_expiration,omitempty"`
	Output          string        `help:"Output format." default:"text" enum:"text,json" json:"output,omitempty"`
}

// TerminateOption represents options for the terminate subcommand.
type TerminateOption struct {
	MicrovmID string `arg:"" optional:"" help:"MicroVM ID to terminate. If omitted, select interactively."`
}

// ShellOption represents options for the shell subcommand.
type ShellOption struct {
	MicrovmID       string        `arg:"" optional:"" help:"MicroVM ID to connect. If omitted, select interactively."`
	TokenExpiration time.Duration `help:"Shell auth token expiration duration." default:"60m" name:"token-expiration" json:"token_expiration,omitempty"`
}

// CurlOption represents options for the curl subcommand.
type CurlOption struct {
	MicrovmID       string        `help:"MicroVM ID. If omitted, select interactively." json:"microvm_id,omitempty"`
	Port            int           `help:"Target port." default:"0" json:"port,omitempty"`
	TokenExpiration time.Duration `help:"Auth token expiration duration." default:"5m" name:"token-expiration" json:"token_expiration,omitempty"`
	Path            string        `arg:"" help:"Request path."`
	Args            []string      `arg:"" optional:"" passthrough:"" help:"Arguments passed to curl."`
}

// CLIParseFunc is the signature of the CLI parser function.
type CLIParseFunc func([]string) (string, *CLIOptions, func(), error)

// ParseCLI parses command-line arguments and returns the subcommand name,
// parsed options, a usage function, and any error.
func ParseCLI(args []string) (string, *CLIOptions, func(), error) {
	if len(args) == 0 || (len(args) > 0 && args[0] == "help") {
		args = []string{"--help"}
	}

	var opts CLIOptions
	parser, err := kong.New(&opts, kong.Vars{"version": Version})
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create CLI parser: %w", err)
	}
	c, err := parser.Parse(args)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse args: %w", err)
	}

	fields := strings.Fields(c.Command())
	sub := fields[0]
	clearInactiveSkillsSubcommand(&opts, fields)
	return sub, &opts, func() { _ = c.PrintUsage(true) }, nil
}

// clearInactiveSkillsSubcommand clears the non-selected skillscmd.Commands
// pointer fields. Kong allocates all cmd:"" pointer fields under a command
// group during parsing, so skillscmd.Commands.Run must not rely on their
// nilness without this.
func clearInactiveSkillsSubcommand(opts *CLIOptions, fields []string) {
	if len(fields) < 2 || fields[0] != "skills" || opts.Skills == nil {
		return
	}
	s := opts.Skills
	sub := fields[1]
	if sub != "list" {
		s.List = nil
	}
	if sub != "install" {
		s.Install = nil
	}
	if sub != "update" {
		s.Update = nil
	}
	if sub != "reinstall" {
		s.Reinstall = nil
	}
	if sub != "uninstall" {
		s.Uninstall = nil
	}
	if sub != "status" {
		s.Status = nil
	}
}

// CLI is the main entry point. It parses CLI options, configures logging,
// and dispatches to the appropriate subcommand.
func CLI(ctx context.Context, parse CLIParseFunc) (int, error) {
	sub, opts, usage, err := parse(os.Args[1:])
	if err != nil {
		return 1, err
	}

	color.NoColor = !opts.Color

	logLevel := new(slog.LevelVar)
	if err := logLevel.UnmarshalText([]byte(opts.LogLevel)); err != nil {
		logLevel.Set(slog.LevelInfo)
	}

	var handler slog.Handler
	switch opts.LogFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	default:
		handler = sloghandler.NewLogHandler(os.Stderr, &sloghandler.HandlerOptions{
			Color: opts.Color,
			HandlerOptions: slog.HandlerOptions{
				Level: logLevel,
			},
		})
	}
	slog.SetDefault(slog.New(handler))

	return dispatchCLI(ctx, sub, usage, opts)
}

func dispatchCLI(ctx context.Context, sub string, usage func(), opts *CLIOptions) (int, error) {
	switch sub {
	case "version", "":
		fmt.Println("lamvms", Version)
		return 0, nil
	}

	if sub == "init" {
		slog.Info("lamvms", "version", Version)
		return initCmd(ctx, &opts.Option, opts.Init)
	}

	if sub == "skills" {
		if err := dispatchSkills(ctx, opts.Skills); err != nil {
			return 1, err
		}
		return 0, nil
	}

	app, err := New(ctx, &opts.Option)
	if err != nil {
		return 1, err
	}
	slog.Info("lamvms", "version", Version)

	switch sub {
	case "deploy":
		err = app.Deploy(ctx, opts.Deploy)
	case "diff":
		err = app.Diff(ctx, opts.Diff)
	case "wait":
		err = app.Wait(ctx, opts.Wait)
	case "rollback":
		err = app.Rollback(ctx, opts.Rollback)
	case "run":
		err = app.Run(ctx, opts.Run)
	case "suspend":
		err = app.Suspend(ctx, opts.Suspend)
	case "resume":
		err = app.Resume(ctx, opts.Resume)
	case "terminate":
		err = app.Terminate(ctx, opts.Terminate)
	case "shell":
		err = app.Shell(ctx, opts.Shell)
	case "curl":
		err = app.Curl(ctx, opts.Curl)
	case "delete":
		err = app.Delete(ctx, opts.Delete)
	case "logs":
		err = app.Logs(ctx, opts.Logs)
	default:
		usage()
	}
	if errors.Is(err, ErrDiff) {
		return 2, nil
	}
	if err != nil {
		return 1, err
	}
	return 0, nil
}
