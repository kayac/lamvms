---
name: lamvms
description: AWS Lambda MicroVMs deployment tool - deploy, run, and manage MicroVM images and instances
license: MIT
compatibility:
  - claude
  - codex
  - agents
allowed_tools:
  - Bash
  - Read
---

# lamvms Skill for LLM Agents

## Overview

lamvms is a deployment and lifecycle management tool for [AWS Lambda MicroVMs](https://aws.amazon.com/lambda/lambda-microvms/), inspired by [fujiwara/lambroll](https://github.com/fujiwara/lambroll). It manages two lifecycles:

- **MicroVM image lifecycle**: `deploy`, `wait`, `rollback`, `delete`
- **MicroVM instance lifecycle**: `run`, `suspend`, `resume`, `terminate`, `shell`, `curl`

Configuration files are API payloads as-is: `microvm.jsonnet`/`microvm.json` maps directly to the [`CreateMicrovmImage`](https://docs.aws.amazon.com/lambda/latest/microvm-api/API_CreateMicrovmImage.html) request, and `run.jsonnet`/`run.json` maps directly to the [`RunMicrovm`](https://docs.aws.amazon.com/lambda/latest/microvm-api/API_RunMicrovm.html) request. To understand what a field in either file does, read the corresponding API doc rather than guessing.

## Official documentation

- Service overview: https://aws.amazon.com/lambda/lambda-microvms/
- API reference (all actions and data types, including `CreateMicrovmImage` and `RunMicrovm`): https://docs.aws.amazon.com/lambda/latest/microvm-api/

## Common workflows

### Check current state before making changes

```bash
# Show differences between local definition and the deployed image
lamvms diff --microvm microvm.jsonnet

# Show differences and exit with code 2 if there are any (useful for scripting)
lamvms diff --microvm microvm.jsonnet --exit-code
```

### Deploy an image

```bash
lamvms deploy --microvm microvm.jsonnet
```

This creates a zip archive from the source directory, uploads it to S3, creates or updates the MicroVM image, and waits for the build to finish. Use `--dry-run` to preview without making changes, and `--keep-versions N` to prune old ACTIVE+SUCCESSFUL versions after a successful deploy.

### Run and connect to a MicroVM instance

```bash
lamvms run --microvm microvm.jsonnet
lamvms shell --microvm microvm.jsonnet
lamvms curl /health --microvm microvm.jsonnet
```

`run` reads `run.jsonnet`/`run.json` next to the microvm definition (or the current directory) if `--run-def` is not given; CLI flags override values from that file. `curl` requires the request path as the first positional argument, followed by any curl flags, e.g. `lamvms curl /health -X POST -d '{"key":"value"}'`.

### Roll back or delete an image

```bash
# Deactivate the latest active version, falling back to the previous one
lamvms rollback --microvm microvm.jsonnet

# Delete the image entirely
lamvms delete --microvm microvm.jsonnet
```

### Inspect instances and logs

```bash
# Interactively select a running instance if no ID is given
lamvms suspend
lamvms resume
lamvms terminate

# Tail CloudWatch logs for the image (delegates to `aws logs tail`)
lamvms logs --microvm microvm.jsonnet --follow
```

## Configuration file reference

- `microvm.jsonnet` / `microvm.json`: MicroVM image definition (`CreateMicrovmImage` payload). Auto-discovered in the current directory if `--microvm` is omitted.
- `run.jsonnet` / `run.json`: MicroVM run configuration (`RunMicrovm` payload, minus `ImageIdentifier` which is resolved automatically). If not found, `run` starts the MicroVM with only `ImageIdentifier`.
- `.microvmignore`: zip archive exclusion patterns (like `.gitignore`).

Jsonnet native functions available in definition files: `std.native('env')('NAME', 'default')`, `std.native('must_env')('NAME')`, `std.native('caller_identity')()` (returns `{Account, Arn, UserID}`). The equivalent Go template functions (`{{ env "NAME" "default" }}`, `{{ must_env "NAME" }}`, `{{ (caller_identity).Account }}`) are available in `.json` files.

## Useful global flags

- `--microvm <path>`: path to the microvm definition file (also `LAMVMS_MICROVM`)
- `--log-level debug`: verbose logging for troubleshooting
- `--profile` / `--region` / `--endpoint`: AWS credential and endpoint overrides
- `--envfile <path>`: load environment variables from a file before evaluating definitions

Run `lamvms <command> --help` for the full flag reference of any subcommand.
