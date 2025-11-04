package main

import (
	"context"
	"partivo_tickets/cmd/consumer/handlers"
	"partivo_tickets/internal/mq/rabbitmq"
	"partivo_tickets/internal/worker"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// ConsumerApp holds the components of the consumer application.
type ConsumerApp struct {
	consumer      *rabbitmq.Consumer
	ticketExpirer *worker.TicketExpirer
	logger        *zap.Logger
}

// NewConsumerApp creates a new consumer application and registers all handlers.
func NewConsumerApp(consumer *rabbitmq.Consumer, ticketExpirer *worker.TicketExpirer, logger *zap.Logger, handlers []handlers.MessageHandler) *ConsumerApp {
	// Register all handlers passed by Wire
	for _, h := range handlers {
		logger.Info("Registering handler", zap.String("queue", h.QueueName()))
		consumer.RegisterHandler(h.QueueName(), h.Handle)
	}

	return &ConsumerApp{
		consumer:      consumer,
		ticketExpirer: ticketExpirer,
		logger:        logger,
	}
}

// Run starts all background workers and blocks until the context is cancelled or a worker fails.
func (a *ConsumerApp) Run(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)

	// Start RabbitMQ consumer
	g.Go(func() error {
		a.logger.Info("Starting RabbitMQ consumer")
		return a.consumer.Start(gCtx)
	})

	// Start Ticket Expirer worker
	g.Go(func() error {
		a.ticketExpirer.Start()
		<-gCtx.Done() // Wait for cancellation signal
		a.ticketExpirer.Stop()
		return nil
	})

	return g.Wait()
}
