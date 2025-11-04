package constants

// PaymentAction defines the type for payment-related actions in messaging.
// Using a dedicated type enhances type safety.
type PaymentAction string

const (
	PaymentActionCreate PaymentAction = "create"
	PaymentActionCancel PaymentAction = "cancel"
	PaymentActionRefund PaymentAction = "refund"
)

// String returns the string representation of the PaymentAction.
func (p PaymentAction) String() string {
	return string(p)
}
