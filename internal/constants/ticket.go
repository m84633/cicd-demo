package constants

type TicketStatus int

const (
	TicketStatusValid TicketStatus = iota
	TicketStatusUsed
	TicketInvalidated //退票
	TicketExpired
	TicketStatusUnknown
)

func (s TicketStatus) String() string {
	switch s {
	case TicketStatusValid:
		return "valid"
	case TicketStatusUsed:
		return "used"
	case TicketInvalidated:
		return "invalidated"
	case TicketExpired:
		return "expired"
	default:
		return "unknown"
	}
}

var ticketStatusMap = map[string]TicketStatus{
	"valid":       TicketStatusValid,
	"used":        TicketStatusUsed,
	"invalidated": TicketInvalidated,
	"expired":     TicketExpired,
	"unknown":     TicketStatusUnknown,
}

func ParseTicketStatus(s string) TicketStatus {
	if status, ok := ticketStatusMap[s]; ok {
		return status
	}
	return TicketStatusUnknown
}
