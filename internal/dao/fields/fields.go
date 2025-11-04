package fields

const (
	FieldObjectId  = "_id"
	FieldCreatedAt = "created_at"
	FieldCreatedBy = "created_by"
	FieldUpdatedAt = "updated_at"
	FieldUpdatedBy = "updated_by"
	FieldPick      = "pick"
	FieldQuantity  = "quantity"
	FieldStatus    = "status"

	FieldTicketUsedAt      = "used_at"
	FieldTicketValidUntil  = "valid_until"
	FieldTicketCheckedInBy = "checked_in_by"

	FieldTicketTypeName        = "name"
	FieldTicketTypePrice       = "price"
	FieldTicketTypeDesc        = "desc"
	FieldTicketTypeVisible     = "visible"
	FieldTicketTypeStartShowAt = "start_show_at"
	FieldTicketTypeStopShowAt  = "stop_show_at"
	FieldTicketTypeEnable      = "enable"
	FieldTicketTypeSaleSetting = "sale_setting"
	FieldTicketTypeEvent       = "event"
	FieldTicketTypeDueDuration = "due_duration"
	FieldTicketTypeDisplay     = "always_display"
	FieldTicketTypeMaxPerOrder = "max_quantity_per_order"
	FieldTicketTypeMinPerOrder = "min_quantity_per_order"

	FieldTicketStockSession     = "session"
	FieldTicketStockType        = "type"
	FieldTicketStockSaleStartAt = "sale_start_at"
	FieldTicketStockSaleEndAt   = "sale_end_at"

	FieldOrdersEvent             = "event"
	FieldOrdersDetails           = "details"
	FieldOrdersSession           = "session"
	FieldOrdersNote              = "note"
	FieldOrderUser               = "user"
	FieldOrderUserUserID         = "user_id"
	FieldOrderTotal              = "total"
	FieldOrderSubtotal           = "subtotal"
	FieldOrderReturned           = "returned"
	FieldOrderPaid               = "paid"
	FieldOrderReturnedItemsCount = "returned_items_count"
	FieldOrderUpdatedBy          = "updated_by"
	FieldOrderItemsCount         = "items_count"

	FieldBillPayment            = "payment"
	FieldBillRefundingPaymentID = "refunding_payment_id"

	FieldOrderItemOrder      = "order"
	FieldOrderItemReturnInfo = "return_info"

	FieldPaymentEnable = "enable"
)
