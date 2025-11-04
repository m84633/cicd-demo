package mq

import "context"

// Publisher defines the interface for any message queue publisher.
// This allows for different MQ implementations (e.g., RabbitMQ, Kafka) to be used interchangeably.
type Publisher interface {
	Publish(ctx context.Context, topic string, body []byte) error
	Close()
}
