package worker

import (
	"context"
	"partivo_tickets/internal/conf"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/mq"
	"time"

	"go.uber.org/zap"
)

// OutboxProcessor periodically polls the outbox collection and publishes events to a message queue.
type OutboxProcessor struct {
	outboxRepo repository.OutboxRepository // Depend on the interface
	publisher  mq.Publisher
	logger     *zap.Logger
	interval   time.Duration
	batchSize  int
}

// NewOutboxProcessor creates a new instance of the outbox processor.
func NewOutboxProcessor(outboxRepo repository.OutboxRepository, publisher mq.Publisher, logger *zap.Logger, cfg *conf.WorkerConfig) *OutboxProcessor {
	return &OutboxProcessor{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		logger:     logger.Named("OutboxProcessor"),
		interval:   time.Duration(cfg.Outbox.IntervalSeconds) * time.Second,
		batchSize:  cfg.Outbox.BatchSize,
	}
}

// Start begins the worker's polling loop in a new goroutine.
// It respects the context for graceful shutdown.
func (p *OutboxProcessor) Start(ctx context.Context) {
	p.logger.Info("Outbox processor started", zap.Duration("interval", p.interval), zap.Int("batchSize", p.batchSize))
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.processEvents(ctx)
		case <-ctx.Done():
			p.logger.Info("Outbox processor shutting down")
			return
		}
	}
}

// processEvents claims a batch of events and attempts to publish them.
func (p *OutboxProcessor) processEvents(ctx context.Context) {
	// 1. Claim a batch of pending events using the optimized DAO method.
	claimedEvents, err := p.outboxRepo.ClaimAndFetchEvents(ctx, p.batchSize)
	if err != nil {
		p.logger.Error("Failed to claim outbox events", zap.Error(err))
		return
	}

	if len(claimedEvents) > 0 {
		p.logger.Info("Claimed events for processing", zap.Int("count", len(claimedEvents)))
	}

	// 2. Process each claimed event.
	for _, event := range claimedEvents {
		// 3. Publish the event to the message queue.
		// Note: You might want to define specific topics/routing keys in your model or constants.
		err := p.publisher.Publish(ctx, event.Topic, []byte(event.Payload))
		if err != nil {
			p.logger.Error("Failed to publish event to RabbitMQ",
				zap.String("event_id", event.ID.Hex()),
				zap.Error(err),
			)
			// If publishing fails, we can use the IncrementRetry method.
			if err := p.outboxRepo.IncrementRetry(ctx, event.ID, err.Error()); err != nil {
				p.logger.Error("Failed to increment retry for event", zap.String("event_id", event.ID.Hex()), zap.Error(err))
			}
			continue // Continue to the next event
		}

		// 4. Mark the event as processed in the database.
		if err := p.outboxRepo.MarkAsProcessed(ctx, event.ID); err != nil {
			p.logger.Error("Failed to mark event as processed",
				zap.String("event_id", event.ID.Hex()),
				zap.Error(err),
			)
		}
	}
}

var _ Worker = (*OutboxProcessor)(nil)
