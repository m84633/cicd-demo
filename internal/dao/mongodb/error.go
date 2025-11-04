package mongodb

import "errors"

var (
	ErrInsufficientStock = errors.New("insufficient ticket stock")
	ErrSubtotalMismatch  = errors.New("subtotal mismatch")
	ErrInvalidTickets    = errors.New("order contains invalid tickets")
	ErrNotFound          = errors.New("not found")
)
