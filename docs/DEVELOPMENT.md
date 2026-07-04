# Development Guide

## Prerequisites

- Go 1.24+
- [buf](https://buf.build/) (for proto linting)
- [golangci-lint](https://golangci-lint.run/) (for Go linting)

## Getting Started

```bash
git clone https://github.com/balcieren/connect-errors-go.git
cd connect-errors-go
go mod download
make test
```

## Project Structure

- `error.go` - Main API: `New()`, `NewWithMessage()`, `FromCode()`, `Wrap()`
- `registry.go` - Error definitions and constants
- `template.go` - Template parsing and substitution
- `proto/connecterrors/v1/error.proto` - Proto extension definition
- `cmd/protoc-gen-connect-errors-go/` - Protoc plugin
- `examples/` - Usage examples

## Running Tests

```bash
make test          # Run all tests with race detection
go test -cover ./... # With coverage
```

## Building the Plugin

```bash
make build         # Build to bin/
make install       # Install to GOPATH/bin
```

## Code Style

- Follow standard Go conventions
- Run `gofmt -s -w .` before committing
- Run `golangci-lint run` to check for issues
