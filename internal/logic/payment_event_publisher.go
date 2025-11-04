package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PaymentEventPublisher is responsible for creating outbox messages for payment-related business events.
type PaymentEventPublisher struct {
	outboxRepo        repository.OutboxRepository
	paymentEventTopic PaymentEventTopic
}

// NewPaymentEventPublisher creates a new PaymentEventPublisher.
func NewPaymentEventPublisher(outboxRepo repository.OutboxRepository, paymentEventTopic PaymentEventTopic) *PaymentEventPublisher {
	return &PaymentEventPublisher{
		outboxRepo:        outboxRepo,
		paymentEventTopic: paymentEventTopic,
	}
}

// PublishPaymentEvent creates an outbox message for a payment-related event (e.g., create, cancel).
func (p *PaymentEventPublisher) PublishPaymentEvent(ctx context.Context, action constants.PaymentAction, bill *models.Bill, merchantID string) error {
	paymentPayload := map[string]interface{}{
		"action":         action.String(),
		"bill_id":        bill.ID.Hex(),
		"amount":         bill.Amount.String(),
		"order_id":       bill.Order.Hex(),
		"payment_method": bill.PaymentMethod.ID.Hex(),
		"merchant_id":    merchantID,
	}
	payloadBytes, err := json.Marshal(paymentPayload)
	if err != nil {
		// Errors from marshalling are considered fatal for the transaction, as the payload can't be constructed.
		return fmt.Errorf("failed to marshal payment event payload: %w", err)
	}

	outboxMsg := &models.OutboxMessage{
		ID:        primitive.NewObjectID(),
		Topic:     string(p.paymentEventTopic),
		Payload:   string(payloadBytes),
		Status:    models.OutboxStatusPending,
		CreatedAt: time.Now(),
	}

	if err := p.outboxRepo.Create(ctx, outboxMsg); err != nil {
		// Errors from creating the outbox message are also fatal for the transaction.
		return fmt.Errorf("failed to create payment event outbox message: %w", err)
	}
	return nil
}
