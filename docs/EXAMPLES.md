# Examples

## Basic Usage

```go
package main

import (
    connecterrors "github.com/balcieren/connect-errors-go"
)

func handleGetUser(id string) error {
    if id == "" {
        return connecterrors.New(connecterrors.ErrInvalidArgument, connecterrors.M{
            "reason": "id is required",
        })
    }

    // Simulate not found
    return connecterrors.New(connecterrors.ErrNotFound, connecterrors.M{
        "id": id,
    })
}
```

## Custom Error Registration

```go
import (
    "connectrpc.com/connect"
    connecterrors "github.com/balcieren/connect-errors-go"
)

func init() {
    connecterrors.Register(connecterrors.Error{
        Code:        "ERROR_EMAIL_EXISTS",
        MessageTpl:  "Email '{{email}}' already registered",
        ConnectCode: connect.CodeAlreadyExists,
        Retryable:   false,
    })
}

func handleCreateUser(email string) error {
    return connecterrors.New(connecterrors.ErrorCode("ERROR_EMAIL_EXISTS"), connecterrors.M{
        "email": email,
    })
}
```

## Wrapping Errors

```go
user, err := db.GetUser(ctx, id)
if err != nil {
    return connecterrors.Wrap(connecterrors.ErrNotFound, err, connecterrors.M{
        "id": id,
    })
}
```

## Custom Message Override

```go
return connecterrors.NewWithMessage(
    connecterrors.ErrNotFound,
    "User '{{id}}' was deleted on {{date}}",
    connecterrors.M{"id": "123", "date": "2026-01-01"},
)
```

## Direct Connect Code

```go
return connecterrors.FromCode(connect.CodeInternal, "unexpected database failure")
```

## Template Validation

```go
tpl := "User '{{id}}' in org '{{org}}'"
err := connecterrors.ValidateTemplate(tpl, connecterrors.M{"id": "123"})
// err: template "User '{{id}}' in org '{{org}}'" missing fields: org
```

## Check Retryability

```go
if connecterrors.IsRetryable(connecterrors.ErrUnavailable) {
    // implement retry logic
}
```
