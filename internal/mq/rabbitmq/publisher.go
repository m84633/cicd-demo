package rabbitmq

import (
	"context"
	"fmt"
	"partivo_tickets/internal/conf"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// Publisher holds the connection and channel for publishing messages to RabbitMQ.
// It implements the mq.Publisher interface.
type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	logger  *zap.Logger
}

// NewPublisher creates and returns a new RabbitMQ Publisher.
func NewPublisher(cfg *conf.RabbitMQConfig, logger *zap.Logger) (*Publisher, error) {
	namedLogger := logger.Named("RabbitMQPublisher")

	// Construct the DSN from config parts
	dsn := fmt.Sprintf("amqp://%s:%s@%s:%d/", cfg.User, cfg.Password, cfg.Host, cfg.Port)

	conn, err := amqp.Dial(dsn)
	if err != nil {
		namedLogger.Error("Failed to connect to RabbitMQ", zap.Error(err))
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		namedLogger.Error("Failed to open a channel", zap.Error(err))
		if connErr := conn.Close(); connErr != nil {
			namedLogger.Error("Failed to close connection after channel failure", zap.Error(connErr))
		}
		return nil, err
	}

	namedLogger.Info("Successfully connected to RabbitMQ")

	return &Publisher{
		conn:    conn,
		channel: ch,
		logger:  namedLogger,
	}, nil
}

// Publish sends a message to a specific exchange with a routing key (topic).
func (p *Publisher) Publish(ctx context.Context, topic string, body []byte) error {
	// For simplicity, we use the default exchange. In a real-world scenario, you would
	// likely declare a specific exchange and have consumers bind queues to it.
	exchange := ""

	err := p.channel.PublishWithContext(ctx,
		exchange, // exchange
		topic,    // routing key (often the queue name for the default exchange)
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // Mark messages as persistent
			Body:         body,
		},
	)

	if err != nil {
		p.logger.Error("Failed to publish a message", zap.Error(err), zap.String("topic", topic))
		return err
	}

	p.logger.Debug("Message published", zap.String("topic", topic), zap.ByteString("body", body))
	return nil
}

// Close gracefully closes the channel and the connection.
func (p *Publisher) Close() {
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			p.logger.Error("Failed to close channel", zap.Error(err))
		}
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			p.logger.Error("Failed to close connection", zap.Error(err))
		}
	}
	p.logger.Info("RabbitMQ connection closed.")
}
