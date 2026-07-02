# Contributing

Thanks for your interest in contributing to lamvms!

## Development setup

```bash
go generate ./...   # codegen + mockgen
go build ./...
go test ./...
```

`go generate` runs two generators and must be re-run whenever their inputs change:

- `cmd/codegen/` regenerates `aws.gen.go` and `testdata/gen/*.json` from the AWS SDK types listed in `cmd/codegen/main.go`'s `targets` slice.
- `mockgen` regenerates `mock_test.go` from the `LambdaMicroVMsClient` interface in `lamvms.go`.

**Do not hand-edit `aws.gen.go`, `mock_test.go`, or files under `testdata/gen/`.** They are generated and will be overwritten.

If you need custom `json.Unmarshal`/`json.Marshal` handling for a new AWS SDK struct (typically one with a union-type interface field), add it to the `targets` slice in `cmd/codegen/main.go` instead of writing it by hand.

## Code style

- Run `golangci-lint run ./...` before submitting; it must report 0 issues.
- Do not add `//nolint` suppressions. Fix the underlying issue instead.
- Exported identifiers need GoDoc comments; avoid inline comments.
- See `CLAUDE.md` for the full set of project conventions.

## Submitting changes

- One logical change per pull request.
- Update `README.md` and `README.ja.md` together when documentation changes are needed.
- Releases are cut via the [tagpr](https://github.com/Songmu/tagpr) flow; you don't need to bump version numbers or create tags yourself.

## Reporting bugs and requesting features

Use the issue templates under `.github/ISSUE_TEMPLATE/`.

## Reporting security issues

See [SECURITY.md](SECURITY.md). Do not open a public issue for security vulnerabilities.
