package constants

const (
	BillTypePayment = "payment"
	BillTypeRefund  = "refund"
)

type BillStatus int

const (
	BillStatusUnknown BillStatus = iota
	BillStatusPending
	BillStatusPaid
	BillStatusRefunded
	BillStatusFailed
	BillStatusCanceled
)

func (s BillStatus) String() string {
	switch s {
	case BillStatusPending:
		return "pending"
	case BillStatusPaid:
		return "paid"
	case BillStatusRefunded:
		return "refunded"
	case BillStatusFailed:
		return "failed"
	case BillStatusCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

var billStatusMap = map[string]BillStatus{
	"pending":  BillStatusPending,
	"paid":     BillStatusPaid,
	"refunded": BillStatusRefunded,
	"failed":   BillStatusFailed,
	"canceled": BillStatusCanceled,
	"unknown":  BillStatusUnknown,
}

func ParseBillStatus(s string) BillStatus {
	if status, ok := billStatusMap[s]; ok {
		return status
	}
	return BillStatusUnknown
}
