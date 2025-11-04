package rabbitmq

import (
	"context"
	"fmt"
	"partivo_tickets/internal/conf"
	"runtime/debug"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// HandlerFunc defines the signature for a function that handles a delivered message.
// It returns an error to indicate whether the message processing was successful.
// Returning an error will cause the message to be negatively-acknowledged (Nacked).
type HandlerFunc func(ctx context.Context, delivery amqp.Delivery) error

// Consumer handles the connection and consumption of messages from RabbitMQ.
type Consumer struct {
	conn     *amqp.Connection
	logger   *zap.Logger
	handlers map[string]HandlerFunc // Maps queue names to handler functions
	done     chan error
}

// NewConsumer creates and returns a new Consumer.
func NewConsumer(cfg *conf.RabbitMQConfig, logger *zap.Logger) (*Consumer, error) {
	namedLogger := logger.Named("RabbitMQConsumer")
	dsn := fmt.Sprintf("amqp://%s:%s@%s:%d/", cfg.User, cfg.Password, cfg.Host, cfg.Port)

	conn, err := amqp.Dial(dsn)
	if err != nil {
		namedLogger.Error("Failed to connect to RabbitMQ", zap.Error(err))
		return nil, err
	}

	namedLogger.Info("Successfully connected to RabbitMQ")

	return &Consumer{
		conn:     conn,
		logger:   namedLogger,
		handlers: make(map[string]HandlerFunc),
		done:     make(chan error),
	}, nil
}

// RegisterHandler registers a handler function for a specific queue.
func (c *Consumer) RegisterHandler(queueName string, handler HandlerFunc) {
	c.handlers[queueName] = handler
}

// Start begins consuming messages from all registered queues.
// It starts a new goroutine for each queue.
func (c *Consumer) Start(ctx context.Context) error {
	if len(c.handlers) == 0 {
		return fmt.Errorf("no handlers registered, consumer will not start")
	}

	for queueName, handler := range c.handlers {
		go c.consumeQueue(ctx, queueName, handler)
	}

	// Wait for a shutdown signal
	return <-c.done
}

func (c *Consumer) consumeQueue(ctx context.Context, queueName string, handler HandlerFunc) {
	ch, err := c.conn.Channel()
	if err != nil {
		c.logger.Error("Failed to open a channel", zap.Error(err), zap.String("queue", queueName))
		c.done <- err
		return
	}
	defer ch.Close()

	// Declare a durable queue to ensure messages are not lost if the server restarts.
	q, err := ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		c.logger.Error("Failed to declare a queue", zap.Error(err), zap.String("queue", queueName))
		c.done <- err
		return
	}

	// Set Quality of Service: only fetch 1 message at a time.
	// This prevents a single consumer from being overwhelmed with messages.
	if err := ch.Qos(1, 0, false); err != nil {
		c.logger.Error("Failed to set QoS", zap.Error(err), zap.String("queue", queueName))
		c.done <- err
		return
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack (we want to manually ack/nack)
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		c.logger.Error("Failed to register a consumer", zap.Error(err), zap.String("queue", queueName))
		c.done <- err
		return
	}

	c.logger.Info("Started consuming from queue", zap.String("queue", q.Name))

	for {
		select {
		case d := <-msgs:
			// Wrap handler execution in a function with a deferred recovery.
			func() {
				defer func() {
					if r := recover(); r != nil {
						c.logger.Error("Panic recovered in message handler",
							zap.Any("panic", r),
							zap.String("stack", string(debug.Stack())),
							zap.String("queue", q.Name),
						)
						// Nack the message so it can be retried or sent to a dead-letter queue.
						if d.Acknowledger != nil {
							d.Nack(false, false) // Do not requeue immediately to avoid panic loops.
						}
					}
				}()

				c.logger.Debug("Received a message", zap.String("queue", q.Name), zap.ByteString("body", d.Body))
				err := handler(ctx, d)
				if err != nil {
					c.logger.Error("Handler failed to process message", zap.Error(err), zap.String("queue", q.Name))
					// Negatively acknowledge the message and requeue it for another attempt.
					// In a real-world scenario, you might want a dead-letter queue for repeated failures.
					if d.Acknowledger != nil {
						d.Nack(false, true) // requeue
					}
				} else {
					// Acknowledge the message, confirming it has been processed successfully.
					if d.Acknowledger != nil {
						d.Ack(false)
					}
				}
			}()
		case <-ctx.Done():
			c.logger.Info("Context cancelled, stopping consumer", zap.String("queue", q.Name))
			c.done <- nil
			return
		}
	}
}

// Close gracefully closes the connection.
func (c *Consumer) Close() {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.logger.Error("Failed to close connection", zap.Error(err))
		}
	}
}
