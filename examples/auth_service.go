package examples

import (
	"context"

	"connectrpc.com/connect"

	cerr "github.com/balcieren/connect-errors-go"
)

// Error codes for the Auth Service.
const (
	ErrInvalidCredentials cerr.ErrorCode = "ERROR_INVALID_CREDENTIALS"
	ErrAccountLocked      cerr.ErrorCode = "ERROR_ACCOUNT_LOCKED"
	ErrTokenExpired       cerr.ErrorCode = "ERROR_TOKEN_EXPIRED"
)

func init() {
	cerr.Register(cerr.Error{
		ErrorCode:   ErrInvalidCredentials,
		MessageTpl:  "Invalid credentials for user '{{email}}'",
		StatusCode:  connect.CodeUnauthenticated,
		Retryable:   false,
	})
	cerr.Register(cerr.Error{
		ErrorCode:   ErrAccountLocked,
		MessageTpl:  "Account '{{email}}' is locked. Try again after {{unlock_at}}",
		StatusCode:  connect.CodePermissionDenied,
		Retryable:   false,
	})
	cerr.Register(cerr.Error{
		ErrorCode:   ErrTokenExpired,
		MessageTpl:  "Token expired at {{expired_at}}",
		StatusCode:  connect.CodeUnauthenticated,
		Retryable:   true,
	})
}

// AuthService handles authentication RPCs.
type AuthService struct{}

// Login authenticates a user with email and password.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	if email == "" || password == "" {
		return "", cerr.NewWithMessage(cerr.ErrInvalidArgument, "email and password are required", nil)
	}

	// Simulate authentication check
	if password != "correct" {
		return "", cerr.New(ErrInvalidCredentials, cerr.M{
			"email": email,
		})
	}

	return "token-abc-123", nil
}

// RefreshToken refreshes an authentication token.
func (s *AuthService) RefreshToken(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", cerr.NewWithMessage(cerr.ErrInvalidArgument, "token is required", nil)
	}

	if token == "expired" {
		return "", cerr.New(ErrTokenExpired, cerr.M{
			"expired_at": "2026-01-01T00:00:00Z",
		})
	}

	return "new-token-xyz", nil
}
