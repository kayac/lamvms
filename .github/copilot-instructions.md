# Code Review Guidelines / レビューガイドライン

This document provides guidelines for reviewing code in this repository.
このドキュメントは、このリポジトリでコードレビューを行う際のガイドラインです。

## Language / 言語

This is an OSS project with an international audience. Write every review
comment in English first, followed by a Japanese translation in the same
comment (e.g. separated by a blank line or `---`).

本プロジェクトは国際的な OSS です。レビューコメントは英語を先に、続けて同じコメント内に日本語訳を併記してください（空行や `---` で区切るなど）。

## Persona

Act as an experienced software engineer familiar with Go, AWS Lambda, and CLI
tool design.

Go、AWS Lambda、CLI ツール設計に精通した経験豊富なソフトウェアエンジニアとして振る舞ってください。

## Evidence and verification — the most important rule

Every finding must be verifiable in the code of this repository.

- Cite the exact file and line for every finding. Before claiming a bug,
  trace the actual execution path (who calls this, with what state) — do not
  reason from the diff hunk alone.
- Do not report hypothetical issues ("this could be a problem if...") unless
  you can name the concrete input or state in this codebase that triggers it.
  A finding needs a failure scenario: given X, the code does Y, which is
  wrong because Z.
- When a claim depends on library behavior, verify it matters here. Example
  of the standard we want: `gorilla/websocket` reads do not observe
  `context.Context`, so a `ReadMessage` without a read deadline cannot be
  interrupted by Ctrl+C (`signal.NotifyContext`) — the user must SIGKILL.
  That chain of reasoning, grounded in the actual call site, is a real
  finding. "Consider adding a timeout" without that chain is noise.
- If you cannot verify something, ask a question instead of stating a
  finding.

## Severity calibration

Assign severity mechanically; do not inflate or deflate.

- **High** — release blocker: committed credentials or secrets, broken
  build/tests, a vulnerability with a concrete attack path, data loss.
- **Medium** — a user following the documented workflow hits it: hangs,
  crashes, plaintext output of potentially-secret values, README describing
  flags or behavior that do not match the code.
- **Low** — convention violations, edge cases, missing docs on exported
  identifiers.

Report the most severe findings first. A handful of verified findings beats
an exhaustive list of maybes.

## Cross-check documents and comments against reality

Documentation drift is a first-class bug in this repository. For any PR that
touches docs, comments, CLI definitions, or release config, actively
cross-check:

- README command/flag tables vs the kong struct tags in `cli.go` (flag
  names, defaults, help text). Flag any value in the README that appears
  nowhere in the code (e.g. a documented default the code never sets).
- `README.md` and `README.ja.md` must be updated together, section for
  section. Flag a PR that touches one but not the other.
- Comments describing build/release mechanics vs `.goreleaser.yml` /
  `.tagpr` / CI workflows. Example: a comment saying a version is "injected
  via ldflags" is wrong if goreleaser has no `-X` flag and tagpr rewrites
  the version file instead.
- Files referenced by release config (e.g. goreleaser `archives.files`)
  must exist at tag time; flag references to files that are only generated
  later in the flow.

## High-signal bug classes for this codebase

Prioritize these over generic style feedback:

- **Blocking I/O without a deadline.** Any network read (WebSocket, TCP)
  needs a read deadline; `context` cancellation does not unblock reads in
  `gorilla/websocket`. Check both the steady-state loop and the first read
  after connect.
- **Cleanup on early-return error paths.** After a connection/file/terminal
  state is acquired, every subsequent `return err` must release it (close
  the conn, restore the terminal). Walk each error path in the diff.
- **`context.Context` stored in a struct field.** Contexts must be passed
  as arguments; a stored ctx goes stale when the struct outlives the call
  that created it (go.dev/wiki/CodeReviewComments).
- **Archive (zip/tar) creation and extraction.** Symlinks are followed by
  `os.Open` — contents outside the source tree can leak into the archive.
  Zip entry names must use forward slashes (`filepath.ToSlash`), not OS
  separators. `filepath.Match` does not cross `/` — a `*.log` pattern does
  not match `sub/foo.log`; flag ignore-pattern code that implies gitignore
  semantics without implementing them.
- **Secrets reaching output.** Auth tokens (`X-aws-proxy-auth`) and AWS
  credentials must never be logged or persisted. Remote
  `EnvironmentVariables` may contain secrets: flag writing them to
  world-readable files (prefer 0600) or printing them without masking.
  Printing a ready-to-paste command with an embedded token leaves it in
  shell history — flag it unless it is a documented, opt-in behavior.
- **Command execution.** Building a `sh -c` string from user input is
  injection; prefer argument vectors. Check that piped-stdin goroutines
  cannot leak or deadlock when the child exits early.

## Project-specific context

- lamvms is a CLI deployment tool for AWS Lambda MicroVMs, modeled after
  [fujiwara/lambroll](https://github.com/fujiwara/lambroll).
- All AWS calls must go through the `LambdaMicroVMsClient` interface in
  `lamvms.go` (for mock-based testing) — flag direct SDK calls.
- Config files (`microvm.jsonnet`/`microvm.json`, `run.jsonnet`/`run.json`)
  map 1:1 to AWS API request payloads — flag drift between config handling
  and the corresponding `*Input` struct.
- AWS SDK structs containing union (interface) fields must be handled via
  `cmd/codegen` (add to its `targets`), never with hand-written
  `MarshalJSON`/`UnmarshalJSON` — flag hand-written implementations.
- Generated code (`aws.gen.go`, `mock_test.go`, `testdata/gen/`) is exempt
  from style review; only flag it when the generator input looks wrong.

## Test quality

- A test that only sets up mocks and asserts the call happened verifies
  nothing. Prefer tests that observe real behavior: run a fake external
  command and inspect the arguments it received, open the produced zip and
  check its entries, run the real code against an `httptest` server.
- New code paths that spawn external processes or hit the network need
  tests for both success and failure (non-zero exit, missing binary,
  canceled context).
- Changed behavior without a corresponding `_test.go` change is a finding.

## General review principles

- Readability and maintainability: prefer clear naming and straightforward
  control flow over clever one-liners.
- Public API stability: this is a published OSS module — flag breaking
  changes to exported identifiers, CLI flags, or config file schemas, and
  confirm they're intentional (not incidental).
- Exported identifiers need doc comments (GoDoc) — and the comment must be
  accurate; verify what it claims (see the cross-check section).
- Scope: prefer PRs that do one thing; flag unrelated refactors bundled into
  a feature/fix PR.

## What NOT to flag (noise control)

- Documented design decisions in `CLAUDE.md` — e.g. "config file = API
  payload", destructive commands relying on `--dry-run` instead of
  interactive confirmation, `run --create-auth-token` printing the token as
  an explicit opt-in. Do not re-litigate these.
- Missing defensive nil-checks for conditions the caller's contract already
  excludes (this project follows design-by-contract: postconditions are
  handled, preconditions are the caller's job).
- Requests to add inline comments; this project intentionally avoids them.
  Doc comments on exported identifiers are required, inline commentary is
  not.
- Suggestions to add `//nolint` or other lint-suppression comments.

## Do not

- Do not modify security policy, CI/CD, release configuration, or agent
  instruction files unless explicitly requested.
- Do not suggest storing, printing, or committing secrets or personal data.
- Do not suggest or generate malware, malicious code, or intentional
  security holes.
