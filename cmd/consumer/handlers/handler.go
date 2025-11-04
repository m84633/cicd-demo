package handlers

import (
	"context"
	amqp "github.com/rabbitmq/amqp091-go"
)

// MessageHandler defines the interface for any message queue handler.
// It allows for a collection of different handlers to be injected as a set.
type MessageHandler interface {
	// QueueName returns the name of the queue this handler should subscribe to.
	QueueName() string
	// Handle processes the delivered message.
	Handle(ctx context.Context, d amqp.Delivery) error
}
