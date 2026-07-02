# lamvms

[![CI](https://github.com/kayac/lamvms/actions/workflows/ci.yml/badge.svg)](https://github.com/kayac/lamvms/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/kayac/lamvms.svg)](https://pkg.go.dev/github.com/kayac/lamvms)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

lamvms is a deployment and lifecycle management tool for [AWS Lambda MicroVMs](https://aws.amazon.com/lambda/lambda-microvms/), inspired by [fujiwara/lambroll](https://github.com/fujiwara/lambroll).

## Install

### Binary (GitHub Releases)

Download the latest binary from [GitHub Releases](https://github.com/kayac/lamvms/releases).

### Go

```bash
go install github.com/kayac/lamvms/cmd/lamvms@latest
```

## Quick Start

### 1. Create a microvm definition

`microvm.jsonnet`:

```jsonnet
local must_env = std.native('must_env');
local caller_identity = std.native('caller_identity');

{
  Name: 'my-app',
  BaseImageArn: 'arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1',
  BuildRoleArn: 'arn:aws:iam::' + caller_identity().Account + ':role/MicrovmBuildRole',
  CodeArtifact: {
    uri: 's3://' + must_env('S3_BUCKET') + '/my-app/app.zip',
  },
}
```

### 2. Deploy

```bash
lamvms deploy --microvm microvm.jsonnet
```

This will:
1. Create a zip archive from the source directory (defaults to the directory of `microvm.jsonnet`)
2. Upload it to S3
3. Create or update the MicroVM image
4. Wait for the build to complete

### 3. Run a MicroVM

```bash
lamvms run --microvm microvm.jsonnet
```

### 4. Connect to a shell

```bash
lamvms shell --microvm microvm.jsonnet
```

Press `Ctrl+D` to disconnect from the shell session.

## Required IAM permissions

lamvms needs the following IAM permissions to operate:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "lambdamicrovms:CreateMicrovmImage",
        "lambdamicrovms:UpdateMicrovmImage",
        "lambdamicrovms:GetMicrovmImage",
        "lambdamicrovms:GetMicrovmImageVersion",
        "lambdamicrovms:UpdateMicrovmImageVersion",
        "lambdamicrovms:DeleteMicrovmImageVersion",
        "lambdamicrovms:ListMicrovmImages",
        "lambdamicrovms:ListMicrovmImageVersions",
        "lambdamicrovms:ListMicrovmImageBuilds",
        "lambdamicrovms:RunMicrovm",
        "lambdamicrovms:GetMicrovm",
        "lambdamicrovms:SuspendMicrovm",
        "lambdamicrovms:ResumeMicrovm",
        "lambdamicrovms:TerminateMicrovm",
        "lambdamicrovms:ListMicrovms",
        "lambdamicrovms:DeleteMicrovmImage",
        "lambdamicrovms:ListTags",
        "lambdamicrovms:TagResource",
        "lambdamicrovms:UntagResource",
        "lambdamicrovms:CreateMicrovmAuthToken",
        "lambdamicrovms:CreateMicrovmShellAuthToken"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": "s3:PutObject",
      "Resource": "arn:aws:s3:::YOUR_BUCKET/*"
    },
    {
      "Effect": "Allow",
      "Action": "sts:GetCallerIdentity",
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": "iam:PassRole",
      "Resource": "arn:aws:iam::*:role/YOUR_EXECUTION_ROLE"
    }
  ]
}
```

- `s3:PutObject` is required for `deploy` to upload the source archive.
- `sts:GetCallerIdentity` is used by the `caller_identity()` template function.
- `iam:PassRole` is only required when passing `--execution-role-arn` to `run`. Scope it down with an `iam:PassedToService` condition once the service principal for AWS Lambda MicroVMs is confirmed.

## Configuration Files

### microvm.jsonnet / microvm.json

The MicroVM image definition. Maps directly to the [CreateMicrovmImage API](https://docs.aws.amazon.com/lambda/latest/microvm-api/API_CreateMicrovmImage.html) payload.

If `--microvm` is not specified, lamvms searches for `microvm.jsonnet` then `microvm.json` in the current directory.

### run.jsonnet / run.json

The MicroVM run configuration. Maps to the [RunMicrovm API](https://docs.aws.amazon.com/lambda/latest/microvm-api/API_RunMicrovm.html) payload (except `ImageIdentifier`, which is resolved from the microvm definition).

```jsonnet
{
  IngressNetworkConnectors: [
    'arn:aws:lambda:ap-northeast-1:aws:network-connector:aws-network-connector:HTTP_INGRESS',
    'arn:aws:lambda:ap-northeast-1:aws:network-connector:aws-network-connector:SHELL_INGRESS',
  ],
  EgressNetworkConnectors: [
    'arn:aws:lambda:ap-northeast-1:aws:network-connector:aws-network-connector:INTERNET_EGRESS',
  ],
  IdlePolicy: {
    AutoResumeEnabled: true,
    MaxIdleDurationSeconds: 900,
    SuspendedDurationSeconds: 300,
  },
}
```

If `--run-def` is not specified, lamvms searches for `run.jsonnet` then `run.json` relative to the microvm definition file, then in the current directory. If no run definition file is found, the MicroVM is started with only the `ImageIdentifier` (minimal configuration).

### .microvmignore

Exclusion patterns for zip archive creation (like `.gitignore`). `microvm.json`, `microvm.jsonnet`, `.microvmignore`, and `.git/*` are excluded by default.

## Template Functions

### Jsonnet native functions

- `std.native('env')('NAME', 'default')` — environment variable with default
- `std.native('must_env')('NAME')` — environment variable (error if unset)
- `std.native('caller_identity')()` — returns `{Account, Arn, UserID}`

### Go template functions (in `.json` files)

- `{{ env "NAME" "default" }}`
- `{{ must_env "NAME" }}`
- `{{ (caller_identity).Account }}`

## Commands

### init

Initialize a microvm definition from an existing MicroVM image.

```bash
lamvms init --image-name my-app
```

| Flag | Description | Default |
|------|-------------|---------|
| `--image-name` | Name of the existing MicroVM image (required) | |
| `--output` | Output file path | `microvm.json` |
| `--jsonnet` | Output as .jsonnet format | `false` |
| `--force-overwrite` | Overwrite existing file | `false` |

### deploy

Deploy a MicroVM image (create or update).

```bash
lamvms deploy [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--src` | Source directory for zip archive | Directory of microvm definition |
| `--skip-archive` | Skip zip creation and S3 upload | `false` |
| `--wait` / `--no-wait` | Wait for build completion | `true` |
| `--keep-versions N` | Keep N latest active versions, delete older | `0` (disabled) |
| `--dry-run` | Show what would be done | `false` |

### wait

Wait for a MicroVM image version to be ready.

```bash
lamvms wait [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--version` | Specific version to wait for | Latest version |
| `--keep-versions N` | Delete old versions after wait | `0` (disabled) |

### rollback

Deactivate the latest active version, falling back to the previous one.

```bash
lamvms rollback [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--dry-run` | Show what would be done | `false` |

### diff

Show the diff between the local microvm definition and the deployed configuration.

```bash
lamvms diff [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--exit-code` | Exit with code 2 if there are differences | `false` |

### run

Run a new MicroVM instance. CLI flags override values from `run.jsonnet`.

```bash
lamvms run [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--run-def` | Path to run definition file | Auto-discovered |
| `--image-version` | Image version to run | Latest active |
| `--execution-role-arn` | IAM role ARN for runtime | |
| `--max-duration` | Maximum duration in seconds | |
| `--run-hook-payload` | Payload for /run lifecycle hook | |
| `--wait` / `--no-wait` | Wait for RUNNING state | `true` |
| `--create-auth-token` | Create auth token after run | `false` |
| `--token-expiration` | Auth token expiration | `30m` |
| `--output` | Output format (`text` or `json`) | `text` |

### shell

Open an interactive shell session to a running MicroVM via WebSocket. Requires `SHELL_INGRESS` network connector.

```bash
lamvms shell [microvm-id]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--token-expiration` | Shell token expiration | `60m` |

Press `Ctrl+D` to disconnect.

### curl

Send a request to a running MicroVM via curl. Automatically handles auth token.

```bash
lamvms curl <path> [curl-flags...]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--microvm-id` | MicroVM ID | Interactive selection |
| `--port` | Target port | Default (8080) |
| `--token-expiration` | Auth token expiration | `5m` |

Example:

```bash
lamvms --microvm microvm.jsonnet curl /health -s
```

### suspend / resume / terminate

Manage MicroVM lifecycle. MicroVM ID can be omitted for interactive selection.

```bash
lamvms suspend [microvm-id]
lamvms resume [microvm-id]
lamvms terminate [microvm-id]
```

`resume` supports `--create-auth-token` and `--token-expiration` to generate a fresh auth token, and `--output` (`text` or `json`, default `text`) to control the output format.

### delete

Delete a MicroVM image.

```bash
lamvms delete [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--dry-run` | Show what would be done | `false` |

### logs

Tail CloudWatch logs for a MicroVM image (delegates to `aws logs tail`).

```bash
lamvms logs [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--since` | Start time | `10m` |
| `--follow` | Follow new logs | `false` |
| `--format` | Log format (`detailed`, `short`, `json`) | `detailed` |
| `--filter-pattern` | CloudWatch filter pattern | |

### skills

Manage the bundled [Agent Skill](https://www.skillsmith.app/) (`SKILL.md`) that teaches LLM coding agents (Claude Code, Codex, etc.) how to use lamvms.

```bash
lamvms skills list
lamvms skills install [--scope user|repo] [--dry-run]
lamvms skills update
lamvms skills reinstall
lamvms skills uninstall
lamvms skills status
```

| Flag | Description | Default |
|------|-------------|---------|
| `--scope` | Install scope (`user` or `repo`) | `user` |
| `--prefix` | Override install directory | |
| `--dry-run` | Preview changes without applying | `false` |
| `--force` | Overwrite unmanaged skills or force downgrade | `false` |

`install --scope repo` installs to `.agents/skills/` in the repository root, which can be committed so teammates' agents pick it up without installing individually.

## Global Flags

| Flag | Env | Description |
|------|-----|-------------|
| `--microvm` | `LAMVMS_MICROVM` | Path to microvm definition |
| `--log-level` | `LAMVMS_LOGLEVEL` | Log level (`debug`, `info`, `warn`, `error`) |
| `--log-format` | `LAMVMS_LOGFORMAT` | Log format (`text`, `json`) |
| `--color` / `--no-color` | `LAMVMS_COLOR` | Colored output |
| `--region` | `AWS_REGION` | AWS region |
| `--profile` | `AWS_PROFILE` | AWS profile |
| `--endpoint` | `AWS_ENDPOINT_URL` | AWS API endpoint |
| `--envfile` | `LAMVMS_ENVFILE` | Environment files |
| `--filter-command` | `LAMVMS_FILTER_COMMAND` | Filter command for interactive selection (e.g. `peco`, `fzf`). If the value contains spaces, it is evaluated via `sh -c`. |
| `-V key=value` | | Jsonnet external variables |
| `--ext-code key=code` | | Jsonnet external code |

## License

MIT
