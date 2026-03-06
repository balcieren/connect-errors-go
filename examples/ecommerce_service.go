package examples

import (
	"context"

	"connectrpc.com/connect"

	cerr "github.com/balcieren/connect-errors-go"
)

// Error codes for the Ecommerce Service.
const (
	ErrOutOfStock          cerr.ErrorCode = "ERROR_OUT_OF_STOCK"
	ErrCartLimit           cerr.ErrorCode = "ERROR_CART_LIMIT"
	ErrShippingUnavailable cerr.ErrorCode = "ERROR_SHIPPING_UNAVAILABLE"
)

func init() {
	cerr.RegisterAll([]cerr.Error{
		{
			Code:        ErrOutOfStock,
			MessageTpl:  "Product '{{product_id}}' is out of stock",
			ConnectCode: connect.CodeFailedPrecondition,
			Retryable:   false,
		},
		{
			Code:        ErrCartLimit,
			MessageTpl:  "Cart limit exceeded: maximum {{max}} items allowed",
			ConnectCode: connect.CodeResourceExhausted,
			Retryable:   false,
		},
		{
			Code:        ErrShippingUnavailable,
			MessageTpl:  "Shipping to '{{region}}' is not available",
			ConnectCode: connect.CodeFailedPrecondition,
			Retryable:   false,
		},
	})
}

// EcommerceService handles e-commerce RPCs.
type EcommerceService struct{}

// AddToCart adds a product to the shopping cart.
func (s *EcommerceService) AddToCart(ctx context.Context, productID string, quantity int) error {
	if productID == "" {
		return cerr.New(cerr.ErrInvalidArgument, cerr.M{
			"reason": "product_id is required",
		})
	}

	if quantity > 100 {
		return cerr.New(ErrCartLimit, cerr.M{
			"max": "100",
		})
	}

	// Simulate out of stock
	if productID == "DISCONTINUED" {
		return cerr.New(ErrOutOfStock, cerr.M{
			"product_id": productID,
		})
	}

	return nil
}

// SetShippingRegion validates the shipping region.
func (s *EcommerceService) SetShippingRegion(ctx context.Context, region string) error {
	blocked := map[string]bool{"ANTARCTICA": true, "MOON": true}
	if blocked[region] {
		return cerr.New(ErrShippingUnavailable, cerr.M{
			"region": region,
		})
	}
	return nil
}
