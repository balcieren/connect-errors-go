# connect-errors-go

[![Test](https://github.com/balcieren/connect-errors-go/actions/workflows/test.yml/badge.svg)](https://github.com/balcieren/connect-errors-go/actions/workflows/test.yml)
[![Lint](https://github.com/balcieren/connect-errors-go/actions/workflows/lint.yml/badge.svg)](https://github.com/balcieren/connect-errors-go/actions/workflows/lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/balcieren/connect-errors-go.svg)](https://pkg.go.dev/github.com/balcieren/connect-errors-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Define errors in `.proto`, generate type-safe Go constructors, catch bugs at compile time.**

A proto-first error handling package for [Connect RPC](https://connectrpc.com). Define your errors alongside your service definitions, run `buf generate`, and get fully typed constructor functions with struct parameters — no magic strings, no typos, no runtime surprises.

```protobuf
// Define in your .proto file
option (connecterrors.v1.error) = {
  code: "ERROR_USER_NOT_FOUND"
  message: "User '{{id}}' not found"
  connect_code: CODE_NOT_FOUND
};
```

```go
// Use the generated typed constructor
return nil, userv1.NewUserNotFound(userv1.UserNotFoundParams{
    Id: req.Msg.Id,  // ← IDE autocomplete, compile-time checked
})
```

> Wrong field name? **Won't compile.** Missing field? **IDE warns you.** Wrong error code? **Doesn't exist.**

## Features

| Feature                       | Description                                                    |
| ----------------------------- | -------------------------------------------------------------- |
| 🔧 **Proto-first**            | Errors live in `.proto` files next to your service definitions |
| ⚡ **Generated Constructors** | `NewXxx(XxxParams{})` — fully typed, zero string literals      |
| 🎯 **Compile-time safe**      | `ErrorCode` type + struct params catch all typos at build      |
| 📝 **Template Messages**      | `{{placeholder}}` → struct fields, validated by the compiler   |
| 🔄 **Retryable Errors**       | Mark errors as retryable directly in proto                     |
| 🪝 **Interceptor**            | Server-side hook for logging, metrics, and tracing             |
| ✅ **errors.Is/As**           | Generated sentinels for standard Go error matching             |

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
  - buf.build/balcieren/connect-errors
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

Errors can be defined at **two levels**:

- **File-level** (`connecterrors.v1.error`) — shared across all services, defined once
- **Method-level** (`connecterrors.v1.connect_error`) — specific to a single RPC

```protobuf
syntax = "proto3";
package user.v1;

import "connecterrors/v1/error.proto";

// ── File-level: shared errors, defined once ──────────────────
option (connecterrors.v1.error) = {
  code: "ERROR_USER_NOT_FOUND"
  message: "User '{{id}}' not found"
  connect_code: CODE_NOT_FOUND
};
option (connecterrors.v1.error) = {
  code: "ERROR_UNAUTHORIZED"
  message: "Authentication required"
  connect_code: CODE_UNAUTHENTICATED
};
option (connecterrors.v1.error) = {
  code: "ERROR_RATE_LIMITED"
  message: "Too many requests, try again later"
  connect_code: CODE_RESOURCE_EXHAUSTED
  retryable: true
};

service UserService {
  rpc GetUser(GetUserRequest) returns (User) {
    // Method-level: only for this RPC
    option (connecterrors.v1.connect_error) = {
      code: "ERROR_INVALID_USER_ID"
      message: "Invalid user ID: '{{id}}'"
      connect_code: CODE_INVALID_ARGUMENT
    };
  };

  rpc DeleteUser(DeleteUserRequest) returns (Empty) {
    option (connecterrors.v1.connect_error) = {
      code: "ERROR_DELETE_FORBIDDEN"
      message: "Cannot delete user: {{reason}}"
      connect_code: CODE_PERMISSION_DENIED
    };
  };

  rpc CreateUser(CreateUserRequest) returns (User) {
    option (connecterrors.v1.connect_error) = {
      code: "ERROR_EMAIL_EXISTS"
      message: "Email '{{email}}' is already registered"
      connect_code: CODE_ALREADY_EXISTS
    };
  };
}
```

> File-level and method-level errors are merged and deduplicated during generation.

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

// Sentinels for errors.Is matching.
var (
    ErrUserNotFoundSentinel    = cerr.NewCodedError(ErrUserNotFound)
    ErrInvalidUserIdSentinel   = cerr.NewCodedError(ErrInvalidUserId)
    ErrDeleteForbiddenSentinel = cerr.NewCodedError(ErrDeleteForbidden)
    ErrEmailExistsSentinel     = cerr.NewCodedError(ErrEmailExists)
    ErrRateLimitedSentinel     = cerr.NewCodedError(ErrRateLimited)
)

func init() { /* auto-registers all errors */ }

// ── Typed constructors ──────────────────────────────────────────

type UserNotFoundParams struct {
    Id string
}

func NewUserNotFound(p UserNotFoundParams) *connect.Error {
    return cerr.New(ErrUserNotFound, cerr.M{"id": p.Id})
}

type DeleteForbiddenParams struct {
    Reason string
}

func NewDeleteForbidden(p DeleteForbiddenParams) *connect.Error {
    return cerr.New(ErrDeleteForbidden, cerr.M{"reason": p.Reason})
}

type EmailExistsParams struct {
    Email string
}

func NewEmailExists(p EmailExistsParams) *connect.Error {
    return cerr.New(ErrEmailExists, cerr.M{"email": p.Email})
}

// No placeholders → no-arg constructor
func NewRateLimited() *connect.Error {
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
        return nil, userv1.NewInvalidUserId(userv1.InvalidUserIdParams{
            Id: req.Msg.Id,
        })
    }

    user, err := s.db.FindUser(ctx, req.Msg.Id)
    if err != nil {
        return nil, userv1.NewUserNotFound(userv1.UserNotFoundParams{
            Id: req.Msg.Id,
        })
    }

    return connect.NewResponse(user), nil
}

func (s *UserServer) CreateUser(ctx context.Context, req *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.User], error) {
    exists, _ := s.db.EmailExists(ctx, req.Msg.Email)
    if exists {
        return nil, userv1.NewEmailExists(userv1.EmailExistsParams{
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

### Server-Side Error Matching (errors.Is / errors.As)

```go
connectErr := userv1.NewUserNotFound(userv1.UserNotFoundParams{Id: "123"})

// Match using generated sentinel
errors.Is(connectErr.Unwrap(), userv1.ErrUserNotFoundSentinel)  // true

// Extract error details
var coded *cerr.CodedError
if errors.As(connectErr.Unwrap(), &coded) {
    fmt.Println(coded.Code()) // "ERROR_USER_NOT_FOUND"
}
```

## Interceptor — Centralized Error Handling

```go
interceptor := cerr.ErrorInterceptor(func(ctx context.Context, err *connect.Error, def cerr.Error) {
    slog.ErrorContext(ctx, "rpc error",
        "code", def.Code,
        "connect_code", def.ConnectCode,
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
        Code:        ErrEmailTaken,
        MessageTpl:  "Email '{{email}}' is taken",
        ConnectCode: connect.CodeAlreadyExists,
    })
}

// Use with the generic API
return nil, cerr.New(ErrEmailTaken, cerr.M{"email": email})

// Or use built-in codes
return nil, cerr.New(cerr.ErrNotFound, cerr.M{"id": id})
return nil, cerr.Wrap(cerr.ErrInternal, dbErr, cerr.M{})
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

| Function                       | Description                              |
| ------------------------------ | ---------------------------------------- |
| `FromError(connectErr)`        | Extract `Error` definition from metadata |
| `ExtractErrorCode(connectErr)` | Get just the error code string           |
| `IsRetryable(code)`            | Check if an error code is retryable      |
| `ConnectCode(code)`            | Get the `connect.Code` for an error code |

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
```

---

## Connect Code Reference (Proto)

When defining errors in your `.proto` files, use the `Code` enum for the `connect_code` field. Most modern IDEs will provide autocomplete for these.

| Proto Enum Constant        | Connect Status Code | Go Constant (`connect.Code`)     |
| -------------------------- | ------------------- | -------------------------------- |
| `CODE_CANCELED`            | Canceled            | `connect.CodeCanceled`           |
| `CODE_UNKNOWN`             | Unknown             | `connect.CodeUnknown`            |
| `CODE_INVALID_ARGUMENT`    | Invalid Argument    | `connect.CodeInvalidArgument`    |
| `CODE_DEADLINE_EXCEEDED`   | Deadline Exceeded   | `connect.CodeDeadlineExceeded`   |
| `CODE_NOT_FOUND`           | Not Found           | `connect.CodeNotFound`           |
| `CODE_ALREADY_EXISTS`      | Already Exists      | `connect.CodeAlreadyExists`      |
| `CODE_PERMISSION_DENIED`   | Permission Denied   | `connect.CodePermissionDenied`   |
| `CODE_RESOURCE_EXHAUSTED`  | Resource Exhausted  | `connect.CodeResourceExhausted`  |
| `CODE_FAILED_PRECONDITION` | Failed Precondition | `connect.CodeFailedPrecondition` |
| `CODE_ABORTED`             | Aborted             | `connect.CodeAborted`            |
| `CODE_OUT_OF_RANGE`        | Out Of Range        | `connect.CodeOutOfRange`         |
| `CODE_UNIMPLEMENTED`       | Unimplemented       | `connect.CodeUnimplemented`      |
| `CODE_INTERNAL`            | Internal            | `connect.CodeInternal`           |
| `CODE_UNAVAILABLE`         | Unavailable         | `connect.CodeUnavailable`        |
| `CODE_DATA_LOSS`           | Data Loss           | `connect.CodeDataLoss`           |
| `CODE_UNAUTHENTICATED`     | Unauthenticated     | `connect.CodeUnauthenticated`    |

---

## Built-in Go Error Codes

Preprocessing `ErrorCode` constants provided by the library:

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

## Error Metadata & Details

Every error includes both HTTP/gRPC metadata headers and Protobuf `connect.ErrorDetail` messages:

### Headers

| Header         | Example           |
| -------------- | ----------------- |
| `x-error-code` | `ERROR_NOT_FOUND` |
| `x-retryable`  | `true` / `false`  |

### Protobuf Details

- `google.rpc.ErrorInfo`: Attached to all errors. `Reason` contains the error code, `Domain` is `"connecterrors"`, and `Metadata` contains the template variables.
- `google.rpc.RetryInfo`: Attached automatically when `Retryable` is true (zero delay).

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
