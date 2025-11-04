package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TicketEventPublisher is responsible for creating outbox messages for ticket-related business events.
type TicketEventPublisher struct {
	outboxRepo           repository.OutboxRepository
	generateTicketsTopic GenerateTicketsTopic
}

// NewTicketEventPublisher creates a new TicketEventPublisher.
func NewTicketEventPublisher(outboxRepo repository.OutboxRepository, generateTicketsTopic GenerateTicketsTopic) *TicketEventPublisher {
	return &TicketEventPublisher{
		outboxRepo:           outboxRepo,
		generateTicketsTopic: generateTicketsTopic,
	}
}

// PublishTicketGenerationTask creates an outbox message to trigger the generation of tickets for an order.
func (p *TicketEventPublisher) PublishTicketGenerationTask(ctx context.Context, orderID primitive.ObjectID) error {
	payload := map[string]string{"order_id": orderID.Hex()}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal ticket generation payload: %w", err)
	}

	outboxMsg := &models.OutboxMessage{
		ID:        primitive.NewObjectID(),
		Topic:     string(p.generateTicketsTopic),
		Payload:   string(payloadBytes),
		Status:    models.OutboxStatusPending,
		CreatedAt: time.Now(),
	}

	if err := p.outboxRepo.Create(ctx, outboxMsg); err != nil {
		return fmt.Errorf("failed to create ticket generation outbox message: %w", err)
	}
	return nil
}
