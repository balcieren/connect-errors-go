package examples

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	cerr "github.com/balcieren/connect-errors-go"
)

// Error codes for the Payment Service.
const (
	ErrInsufficientFunds cerr.ErrorCode = "ERROR_INSUFFICIENT_FUNDS"
	ErrCardDeclined      cerr.ErrorCode = "ERROR_CARD_DECLINED"
	ErrPaymentTimeout    cerr.ErrorCode = "ERROR_PAYMENT_TIMEOUT"
)

func init() {
	cerr.RegisterAll([]cerr.Error{
		{
			Code:        ErrInsufficientFunds,
			MessageTpl:  "Insufficient funds: requested {{amount}}, available {{balance}}",
			ConnectCode: connect.CodeFailedPrecondition,
			Retryable:   false,
		},
		{
			Code:        ErrCardDeclined,
			MessageTpl:  "Card ending in {{last4}} was declined: {{reason}}",
			ConnectCode: connect.CodeFailedPrecondition,
			Retryable:   false,
		},
		{
			Code:        ErrPaymentTimeout,
			MessageTpl:  "Payment processing timed out for order '{{order_id}}'",
			ConnectCode: connect.CodeDeadlineExceeded,
			Retryable:   true,
		},
	})
}

// PaymentService handles payment RPCs.
type PaymentService struct{}

// ProcessPayment processes a payment.
func (s *PaymentService) ProcessPayment(ctx context.Context, orderID string, amount float64, cardLast4 string) error {
	if amount <= 0 {
		return cerr.New(cerr.ErrInvalidArgument, cerr.M{
			"reason": "amount must be positive",
		})
	}

	balance := 50.0
	if amount > balance {
		return cerr.New(ErrInsufficientFunds, cerr.M{
			"amount":  fmt.Sprintf("%.2f", amount),
			"balance": fmt.Sprintf("%.2f", balance),
		})
	}

	return nil
}
