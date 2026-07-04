# connect-errors-go

[![Test](https://github.com/balcieren/connect-errors-go/actions/workflows/test.yml/badge.svg)](https://github.com/balcieren/connect-errors-go/actions/workflows/test.yml)
[![Lint](https://github.com/balcieren/connect-errors-go/actions/workflows/lint.yml/badge.svg)](https://github.com/balcieren/connect-errors-go/actions/workflows/lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/balcieren/connect-errors-go.svg)](https://pkg.go.dev/github.com/balcieren/connect-errors-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Define errors in `.proto`, generate type-safe Go constructors, catch bugs at compile time.**

A proto-first error handling package for [Connect RPC](https://connectrpc.com). Define your errors alongside your service definitions, run `buf generate`, and get fully typed constructor functions with struct parameters — no magic strings, no typos, no runtime surprises.

```protobuf
// Define in your .proto file
option (errors.v1.rpc_error) = {
  error_code: "ERROR_USER_NOT_FOUND"
  message: "User '{{id}}' not found"
  status_code: NOT_FOUND
};
```

```go
// Use the generated typed constructor
return nil, userv1.NewErrUserNotFound(userv1.UserNotFoundParams{
    Id: req.Msg.Id,  // ← IDE autocomplete, compile-time checked
})
```

> Wrong field name? **Won't compile.** Missing field? **IDE warns you.** Wrong error code? **Doesn't exist.**

## Features

| Feature                       | Description                                                    |
| ----------------------------- | -------------------------------------------------------------- |
| 🔧 **Proto-first**            | Errors live in `.proto` files next to your service definitions |
| ⚡ **Generated Constructors** | `NewErrXxx(XxxParams{})` — fully typed, zero string literals   |
| 🎯 **Compile-time safe**      | `ErrorCode` type + struct params catch all typos at build      |
| 📝 **Template Messages**      | `{{placeholder}}` → struct fields, validated at runtime        |
| 🔄 **Retryable Errors**       | Mark errors as retryable with custom retry delays in proto     |
| 🪝 **Interceptors**           | Unary + streaming hooks for logging, metrics, and tracing      |
| ✅ **errors.Is / errors.As**  | Idiomatic Go error matching via `ErrorCode` & `CodedError`     |
| 🔀 **Error Matching**         | `MatchesError` + `MatchError` (switch-like, deterministic)     |
| 🔨 **Error Builder**          | Chainable API: `.WithFieldViolation().WithRetryDelay().Build()` |
| 🌐 **RFC 7807**               | `ToProblemDetails()` for REST/HTTP adapters                    |
| 🏷️ **Configurable Domain**    | `SetDomain("myapp")` for `google.rpc.ErrorInfo`                |
| 🧩 **Context-aware**          | `NewCtx()` + `SetContextExtractor()` for trace IDs, etc.       |
| 📊 **Customizable Logging**   | `SetErrorLogger()` + `SetValidationLogger()` for any log pkg   |

## Quick Start

```bash
go get github.com/balcieren/connect-errors-go
go install github.com/balcieren/connect-errors-go/cmd/protoc-gen-connect-errors-go@latest
```

---

## Step 1: Configure Buf

```yaml
# buf.yaml
version: v2
modules:
  - path: proto
deps:
  - buf.build/balcieren/errors
  - buf.build/protocolbuffers/wellknowntypes
```

```yaml
# buf.gen.yaml
version: v2

managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/yourorg/yourapp/gen/go

plugins:
  - local: protoc-gen-go
    out: gen/go
    opt: paths=source_relative

  - local: protoc-gen-connect-go
    out: gen/go
    opt: paths=source_relative

  # ⭐ Generates typed error constructors
  - local: protoc-gen-connect-errors-go
    out: gen/go
    opt: paths=source_relative
```

```bash
buf dep update
```
## Step 2: Define Errors in Proto

Errors are defined at the **method level** using the `errors.v1.rpc_error` option:

```protobuf
syntax = "proto3";
package user.v1;

import "errors/v1/error.proto";

service UserService {
  rpc GetUser(GetUserRequest) returns (User) {
    // Method-level: only for this RPC
    option (errors.v1.rpc_error) = {
      error_code: "ERROR_INVALID_USER_ID"
      message: "Invalid user ID: '{{id}}'"
      status_code: INVALID_ARGUMENT
    };
  };

  rpc DeleteUser(DeleteUserRequest) returns (Empty) {
    option (errors.v1.rpc_error) = {
      error_code: "ERROR_DELETE_FORBIDDEN"
      message: "Cannot delete user: {{reason}}"
      status_code: PERMISSION_DENIED
    };
  };

  rpc CreateUser(CreateUserRequest) returns (User) {
    option (errors.v1.rpc_error) = {
      error_code: "ERROR_EMAIL_EXISTS"
      message: "Email '{{email}}' is already registered"
      status_code: ALREADY_EXISTS
    };
  };
}
```

> Errors with the same code across different methods are deduplicated during generation.

Each `{{placeholder}}` in the message becomes a **struct field** in the generated constructor.

## Step 3: Generate Code

```bash
buf generate
```

This creates a `*_connect_errors.go` file:

```go
// Code generated by protoc-gen-connect-errors-go. DO NOT EDIT.
package userv1

import (
    "connectrpc.com/connect"
    cerr "github.com/balcieren/connect-errors-go"
)

// Typed error code constants.
const (
    ErrUserNotFound    cerr.ErrorCode = "ERROR_USER_NOT_FOUND"
    ErrInvalidUserId   cerr.ErrorCode = "ERROR_INVALID_USER_ID"
    ErrDeleteForbidden cerr.ErrorCode = "ERROR_DELETE_FORBIDDEN"
    ErrEmailExists     cerr.ErrorCode = "ERROR_EMAIL_EXISTS"
    ErrRateLimited     cerr.ErrorCode = "ERROR_RATE_LIMITED"
)

func init() { /* auto-registers all errors */ }

// ── Typed constructors ──────────────────────────────────────────

type UserNotFoundParams struct {
    Id string
}

func NewErrUserNotFound(p UserNotFoundParams) *connect.Error {
    return cerr.New(ErrUserNotFound, cerr.M{"id": p.Id})
}

type DeleteForbiddenParams struct {
    Reason string
}

func NewErrDeleteForbidden(p DeleteForbiddenParams) *connect.Error {
    return cerr.New(ErrDeleteForbidden, cerr.M{"reason": p.Reason})
}

type EmailExistsParams struct {
    Email string
}

func NewErrEmailExists(p EmailExistsParams) *connect.Error {
    return cerr.New(ErrEmailExists, cerr.M{"email": p.Email})
}

// No placeholders → no-arg constructor
func NewErrRateLimited() *connect.Error {
    return cerr.New(ErrRateLimited, nil)
}

// ── Client-side error matchers ──────────────────────────────────

func IsUserNotFound(err error) bool {
    var connectErr *connect.Error
    if !errors.As(err, &connectErr) {
        return false
    }
    code, ok := cerr.ExtractErrorCode(connectErr)
    return ok && code == string(ErrUserNotFound)
}

// IsInvalidUserId, IsDeleteForbidden, IsEmailExists, IsRateLimited ...
```

> Duplicate error codes across methods are automatically deduplicated.

## Step 4: Use in Your Handlers

```go
func (s *UserServer) GetUser(ctx context.Context, req *connect.Request[userv1.GetUserRequest]) (*connect.Response[userv1.User], error) {
    if req.Msg.Id == "" {
        return nil, userv1.NewErrInvalidUserId(userv1.InvalidUserIdParams{
            Id: req.Msg.Id,
        })
    }

    user, err := s.db.FindUser(ctx, req.Msg.Id)
    if err != nil {
        return nil, userv1.NewErrUserNotFound(userv1.UserNotFoundParams{
            Id: req.Msg.Id,
        })
    }

    return connect.NewResponse(user), nil
}

func (s *UserServer) CreateUser(ctx context.Context, req *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.User], error) {
    exists, _ := s.db.EmailExists(ctx, req.Msg.Email)
    if exists {
        return nil, userv1.NewErrEmailExists(userv1.EmailExistsParams{
            Email: req.Msg.Email,
        })
    }
    // ...
}
```

## Step 5: Handle Errors on the Client

The plugin generates `IsXxx(err error) bool` matchers for client-side error checking:

```go
// Client code — clean, type-safe error handling
_, err := client.GetUser(ctx, connect.NewRequest(&userv1.GetUserRequest{Id: "123"}))
if err != nil {
    switch {
    case userv1.IsUserNotFound(err):
        fmt.Println("User does not exist")
    case userv1.IsInvalidUserId(err):
        fmt.Println("Bad ID format")
    default:
        fmt.Println("Unexpected error:", err)
    }
}
```

### Server-Side Error Matching (errors.As)

```go
connectErr := userv1.NewErrUserNotFound(userv1.UserNotFoundParams{Id: "123"})

// Extract error details safely down to the core error
var coded *cerr.CodedError
if errors.As(connectErr.Unwrap(), &coded) {
    fmt.Println(coded.Code()) // "ERROR_USER_NOT_FOUND"
}
```

### Client-Side Error Info Extraction

The plugin also generates typed `ExtractXxxInfo` helpers for pulling structured `google.rpc.ErrorInfo` metadata from errors:

```go
// Use the generated typed extractor for specific errors:
if info := userv1.ExtractUserNotFoundInfo(err); info != nil {
    fmt.Println(info.Metadata["id"]) // "123"
}
```

## Interceptor — Centralized Error Handling

```go
interceptor := cerr.ErrorInterceptor(func(ctx context.Context, err *connect.Error, def cerr.Error) {
    slog.ErrorContext(ctx, "rpc error",
        "code", def.ErrorCode,
        "status_code", def.StatusCode,
        "retryable", def.Retryable,
    )
})

mux.Handle(userv1connect.NewUserServiceHandler(svc,
    connect.WithInterceptors(interceptor),
))
```

---

## Project Structure

```text
your-project/
├── buf.yaml
├── buf.gen.yaml
├── proto/
│   └── user/
│       └── v1/
│           └── service.proto        ← Define errors here
└── gen/
    └── go/
        └── user/
            └── v1/
                ├── service.pb.go
                ├── service_connect.go
                └── service_connect_errors.go  ← Generated constructors
```

---

## Alternative: Manual Usage (Without Proto)

If you don't use proto-based definitions, you can define errors manually:

```go
import cerr "github.com/balcieren/connect-errors-go"

// Define typed constants
const ErrEmailTaken cerr.ErrorCode = "ERROR_EMAIL_TAKEN"

func init() {
    cerr.Register(cerr.Error{
    ErrorCode:   ErrEmailTaken,
    MessageTpl:  "Email '{{email}}' is taken",
    StatusCode:  connect.CodeAlreadyExists,
    })
}

// Use with the generic API
return nil, cerr.New(ErrEmailTaken, cerr.M{"email": email})

// Or use built-in codes
return nil, cerr.New(cerr.ErrNotFound, nil)
return nil, cerr.Wrap(cerr.ErrInternal, dbErr, nil)
return nil, cerr.Newf(cerr.ErrNotFound, "User %q not found", id)
```

---

## API Reference

### Error Creation

| Function                          | Description                                   |
| --------------------------------- | --------------------------------------------- |
| `New(code, data)`                 | Create error from registry with template data |
| `NewWithMessage(code, msg, data)` | Override default template message             |
| `Newf(code, format, args...)`     | fmt.Sprintf-style formatting                  |
| `Wrap(code, err, data)`           | Wrap underlying error with context            |
| `FromCode(code, msg)`             | Create directly from connect.Code             |

All `code` parameters accept the `ErrorCoder` interface — both `ErrorCode` constants and `*CodedError` sentinels work.

### Error Inspection

| Function                       | Description                                |
| ------------------------------ | ------------------------------------------ |
| `FromError(connectErr)`        | Extract `Error` definition from metadata   |
| `ExtractErrorCode(connectErr)` | Get just the error code string             |
| `ExtractErrorInfo(err)`        | Extract `google.rpc.ErrorInfo` detail      |
| `ExtractRetryInfo(err)`        | Extract `google.rpc.RetryInfo` detail      |
| `IsRetryable(codeOrErr)`       | Check if an error code/error is retryable  |
| `StatusCode(code)`             | Get the `connect.Code` for an error code   |
| `MatchesError(err, code)`      | Check if an error matches a code           |
| `MatchError[T](err, matchers)` | Switch-like error matching (deterministic) |
| `ToProblemDetails(err)`        | Convert to RFC 7807 Problem Details        |

### Registry

| Function           | Description                                |
| ------------------ | ------------------------------------------ |
| `Register(err)`    | Register an error definition               |
| `RegisterAll(errs)`| Register multiple error definitions        |
| `Lookup(code)`     | Look up an error definition by code        |
| `MustLookup(code)` | Look up or panic if not found              |
| `Codes()`          | Return sorted list of all registered codes |

### Advanced Error Construction

| Function                             | Description                                    |
| ------------------------------------ | ---------------------------------------------- |
| `NewWithRetry(code, data, delay)`    | Create error with custom retry delay           |
| `NewCtx(ctx, code, data)`            | Create error with context-extracted metadata   |
| `NewBuilder(code, data)`             | Chainable builder for complex errors           |
| `WithFieldViolation(err, field, msg)`| Add `google.rpc.BadRequest` FieldViolation     |
| `WithDetails(err, details...)`       | Add protobuf details to an error               |
| `SetDomain(domain)`                  | Configure global error domain                  |
| `GetDomain()`                        | Get the current error domain                   |
| `GetHeaderKeys()`                    | Get the current header key configuration       |
| `SetContextExtractor(fn)`            | Configure context-to-metadata extraction       |
| `SetErrorLogger(fn)`                 | Configure logger for all error creations       |
| `SetValidationLogger(fn)`            | Configure logger for template validation fails |

### Template Utilities

```go
cerr.TemplateFields("User '{{id}}' in {{org}}")     // → ["id", "org"]
cerr.ValidateTemplate("User '{{id}}'", cerr.M{})    // → error: missing "id"
cerr.FormatTemplate("User '{{id}}'", cerr.M{"id": "123"}) // → "User '123'"
```

### Configuration

```go
// Customize metadata header keys
cerr.SetHeaderKeys("x-custom-error-code", "x-custom-retryable")

// Customize error domain (used in google.rpc.ErrorInfo)
cerr.SetDomain("myapp")

// Configure context-to-metadata extraction for trace IDs, etc.
cerr.SetContextExtractor(func(ctx context.Context) cerr.M {
    if traceID, ok := ctx.Value("trace_id").(string); ok {
        return cerr.M{"trace_id": traceID}
    }
    return nil
})
```

### Logging

Connect errors provides two customizable loggers:

- **ErrorLogger**: Called for every error creation (`New`, `NewWithMessage`, `Wrap`, etc.)
- **ValidationLogger**: Called when template validation fails (missing placeholder data)

Both default to no-op. Configure them to integrate with your logging/monitoring stack:

```go
// Log all errors with slog
cerr.SetErrorLogger(func(code string, statusCode connect.Code, retryable bool, data cerr.M) {
    slog.Info("error created", "code", code, "status_code", statusCode, "retryable", retryable)
})

// Log validation failures
cerr.SetValidationLogger(func(code string, data cerr.M, err error) {
    slog.Error("template validation failed", "code", code, "error", err)
})
```

#### Integration Examples

```go
// Zap
logger, _ := zap.NewProduction()
cerr.SetErrorLogger(func(code string, statusCode connect.Code, retryable bool, data cerr.M) {
    logger.Info("error created",
        zap.String("code", code),
        zap.String("status_code", statusCode.String()),
        zap.Bool("retryable", retryable),
    )
})

// Sentry
cerr.SetErrorLogger(func(code string, statusCode connect.Code, retryable bool, data cerr.M) {
    sentry.WithScope(func(scope *sentry.Scope) {
        scope.SetTag("error_code", code)
        scope.SetContext("data", data)
        sentry.CaptureMessage("Error created: " + code)
    })
})

// Prometheus metrics
cerr.SetErrorLogger(func(code string, statusCode connect.Code, retryable bool, data cerr.M) {
    errorsCreated.WithLabelValues(code, statusCode.String()).Inc()
})
```

### Error Builder

```go
// Chainable API for complex error construction
err := cerr.NewBuilder(cerr.ErrInvalidArgument, nil).
    WithFieldViolation("email", "already registered").
    WithRetryDelay(5 * time.Second).
    WithDomain("myapp").
    WithMessage("custom message").
    WithDetail(detail).
    Build()
```

### MatchError with Matcher[T]

```go
// Switch-like matching with typed return values
result, ok := cerr.MatchError(err, []cerr.Matcher[string]{
    {ErrorCode: cerr.ErrNotFound, Fn: func() string { return "not found" }},
    {ErrorCode: cerr.ErrInternal, Fn: func() string { return "internal error" }},
})
if ok {
    fmt.Println(result)
}
```

### RFC 7807 Problem Details

```go
// Convert Connect errors to RFC 7807 format for REST adapters
err := cerr.New(cerr.ErrNotFound, nil)
pd := cerr.ToProblemDetails(err)
// pd.Status = 404, pd.Title = "ERROR_NOT_FOUND", pd.Detail = "Resource not found"
```

ProblemDetails struct:

```go
type ProblemDetails struct {
    Type     string `json:"type,omitempty"`
    Title    string `json:"title,omitempty"`
    Detail   string `json:"detail,omitempty"`
    Status   int    `json:"status,omitempty"`
    Instance string `json:"instance,omitempty"`
}
```

### Streaming Interceptor

```go
// Streaming counterpart of ErrorInterceptor
interceptor := cerr.StreamingErrorInterceptor(func(ctx context.Context, err *connect.Error, def cerr.Error) {
    slog.ErrorContext(ctx, "streaming rpc error", "code", def.ErrorCode)
})
```

`StreamingHandlerInterceptorFunc` implements `connect.Interceptor` and provides hooks for all RPC types:

```go
type StreamingHandlerInterceptorFunc func(connect.StreamingHandlerFunc) connect.StreamingHandlerFunc

// Methods:
// - WrapUnary(next connect.UnaryFunc) connect.UnaryFunc
// - WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc
// - WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc
```

### errors.Is Support

```go
// ErrorCode implements the error interface, enabling idiomatic Go error matching
err := cerr.New(cerr.ErrNotFound, nil)
if errors.Is(err, cerr.ErrNotFound) {
    fmt.Println("not found")
}
```

---

## RPC Code Reference (Proto)

When defining errors in your `.proto` files, use the `google.rpc.Code` enum for the `status_code` field. Most modern IDEs will provide autocomplete for these.

| Proto Enum Constant  | Connect Status Code | Go Constant (`connect.Code`)     |
| -------------------- | ------------------- | -------------------------------- |
| `CANCELED`           | Canceled            | `connect.CodeCanceled`           |
| `UNKNOWN`            | Unknown             | `connect.CodeUnknown`            |
| `INVALID_ARGUMENT`   | Invalid Argument    | `connect.CodeInvalidArgument`    |
| `DEADLINE_EXCEEDED`  | Deadline Exceeded   | `connect.CodeDeadlineExceeded`   |
| `NOT_FOUND`          | Not Found           | `connect.CodeNotFound`           |
| `ALREADY_EXISTS`     | Already Exists      | `connect.CodeAlreadyExists`      |
| `PERMISSION_DENIED`  | Permission Denied   | `connect.CodePermissionDenied`   |
| `RESOURCE_EXHAUSTED` | Resource Exhausted  | `connect.CodeResourceExhausted`  |
| `FAILED_PRECONDITION`| Failed Precondition | `connect.CodeFailedPrecondition` |
| `ABORTED`            | Aborted             | `connect.CodeAborted`            |
| `OUT_OF_RANGE`       | Out Of Range        | `connect.CodeOutOfRange`         |
| `UNIMPLEMENTED`      | Unimplemented       | `connect.CodeUnimplemented`      |
| `INTERNAL`           | Internal            | `connect.CodeInternal`           |
| `UNAVAILABLE`        | Unavailable         | `connect.CodeUnavailable`        |
| `DATA_LOSS`          | Data Loss           | `connect.CodeDataLoss`           |
| `UNAUTHENTICATED`    | Unauthenticated     | `connect.CodeUnauthenticated`    |

---

## Built-in Go Error Codes

Pre-defined `ErrorCode` constants provided by the library:

| Constant                | Default Connect Code       | Default Retryable |
| ----------------------- | -------------------------- | ----------------- |
| `ErrNotFound`           | `CODE_NOT_FOUND`           | No                |
| `ErrInvalidArgument`    | `CODE_INVALID_ARGUMENT`    | No                |
| `ErrAlreadyExists`      | `CODE_ALREADY_EXISTS`      | No                |
| `ErrPermissionDenied`   | `CODE_PERMISSION_DENIED`   | No                |
| `ErrUnauthenticated`    | `CODE_UNAUTHENTICATED`     | No                |
| `ErrInternal`           | `CODE_INTERNAL`            | No                |
| `ErrUnavailable`        | `CODE_UNAVAILABLE`         | Yes               |
| `ErrDeadlineExceeded`   | `CODE_DEADLINE_EXCEEDED`   | Yes               |
| `ErrResourceExhausted`  | `CODE_RESOURCE_EXHAUSTED`  | Yes               |
| `ErrFailedPrecondition` | `CODE_FAILED_PRECONDITION` | No                |
| `ErrAborted`            | `CODE_ABORTED`             | Yes               |
| `ErrOutOfRange`         | `CODE_OUT_OF_RANGE`        | No                |
| `ErrUnimplemented`      | `CODE_UNIMPLEMENTED`       | No                |
| `ErrCanceled`           | `CODE_CANCELED`            | No                |
| `ErrDataLoss`           | `CODE_DATA_LOSS`           | No                |

---

## Concurrency Safety

All global state (registry, domain, header keys, loggers, context extractor) is protected for concurrent use:

- **Registry**: Copy-on-write with `atomic.Value` for lock-free reads, `sync.Mutex` for writes
- **Loggers**: `atomic.Value` for `SetErrorLogger()` / `SetValidationLogger()`
- **Domain & Header Keys**: `atomic.Value` with mutex-protected read-modify-write for `SetHeaderKeys()`
- **Context Extractor**: `atomic.Value`
- **Error Builder**: `WithDomain()` overrides domain per-error without mutating global state

All error creation functions (`New`, `Wrap`, `NewWithMessage`, etc.) are safe to call concurrently from multiple goroutines.

---

## Error Metadata & Details

Every error includes both HTTP/gRPC metadata headers and Protobuf `connect.ErrorDetail` messages:

### Headers

| Header         | Example           |
| -------------- | ----------------- |
| `x-error-code` | `ERROR_NOT_FOUND` |
| `x-retryable`  | `true` / `false`  |

### Protobuf Details

- `google.rpc.ErrorInfo`: Attached to all errors. `Reason` contains the error code, `Domain` defaults to `"connecterrors"` (configurable via `SetDomain()`), and `Metadata` contains the template variables.
- `google.rpc.RetryInfo`: Attached automatically when `Retryable` is true. Delay defaults to zero but can be set via `retry_delay_ms` in proto or `NewWithRetry()` at runtime.
- `google.rpc.BadRequest`: Attached via `WithFieldViolation()` for input validation failures.

Use the provided extractors to safely parse details:

```go
if info, ok := cerr.ExtractErrorInfo(err); ok {
    fmt.Println(info.Reason)     // "ERROR_NOT_FOUND"
    fmt.Println(info.Metadata)   // map[string]string{"id": "123"}
}

if retry, ok := cerr.ExtractRetryInfo(err); ok {
    fmt.Println("Is retryable!")
}
```

---

## Contributing

See [CONTRIBUTING.md](docs/CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)
