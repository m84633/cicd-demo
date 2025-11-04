package logic

import "errors"

var (
	ErrOrderStatusUpdate           = errors.New("invalid order status operator")
	ErrTicketNotFound              = errors.New("ticket not found")
	ErrTicketNotValid              = errors.New("ticket is not valid")
	ErrTicketNotWithinValidityPeriod = errors.New("ticket is not within its validity period")
	ErrPermissionDenied            = errors.New("permission denied")
	ErrPermanent                   = errors.New("a permanent error occurred that should not be retried")
	ErrBillAlreadyProcessed    = errors.New("bill has already been processed")
)
