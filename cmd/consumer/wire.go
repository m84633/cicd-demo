//go:build wireinject
// +build wireinject

package main

import (
	"partivo_tickets/cmd/consumer/handlers"
	"partivo_tickets/internal/conf"
	"partivo_tickets/internal/dao/mongodb"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/logger"
	"partivo_tickets/internal/logic"
	"partivo_tickets/internal/mq/rabbitmq"
	"partivo_tickets/internal/provider"
	"partivo_tickets/internal/worker"
	"partivo_tickets/pkg/snowflake"

	"github.com/google/wire"
)

// provideHandlers collects all individual MessageHandlers into a slice.
func provideHandlers(ticketHandler *handlers.TicketHandler, billHandler *handlers.BillStatusUpdateHandler) []handlers.MessageHandler {
	return []handlers.MessageHandler{
		ticketHandler,
		billHandler,
	}
}

// InitializeConsumerApp creates the consumer application and its dependencies.
func InitializeConsumerApp(appConfig *conf.AppConfig) (*ConsumerApp, func(), error) {
	wire.Build(
		// Config Providers
		wire.FieldsOf(new(*conf.AppConfig), "LogConfig", "MongodbConfig", "RabbitMQConfig", "WorkerConfig"),
		provider.ProvideAppMode,

		// Common Components
		logger.NewLogger,
		mongodb.NewMongoDB,
		provider.ProvideDatabase,
		provider.ProvideRelationClient,
		provider.ProvideTransactionManager,
		provider.ProvideJwtGenerator,
		provider.ProvideEventsClient,
		provider.ProvideMachineID,
		provider.ProvidePaymentEventTopic,
		snowflake.NewGenerator,
		provider.ProvideGenerateTicketsTopic,

		// DAO Layer (Only those needed by TicketLogic's new method)
		mongodb.NewTicketsDAO,
		wire.Bind(new(repository.TicketsRepository), new(*mongodb.TicketsDAO)),
		mongodb.NewOrdersDAO,
		wire.Bind(new(repository.OrdersRepository), new(*mongodb.OrdersDAO)),
		mongodb.NewAuditLogDAO,
		wire.Bind(new(repository.AuditLogRepository), new(*mongodb.AuditLogDAO)),
		mongodb.NewBillDAO,
		wire.Bind(new(repository.BillRepository), new(*mongodb.BillDAO)),
		mongodb.NewOutboxDAO,
		wire.Bind(new(repository.OutboxRepository), new(*mongodb.OutboxDAO)),

		// Logic Layer (Only TicketLogic is needed for now)
		logic.NewPaymentEventPublisher,
		logic.NewTicketEventPublisher,
		logic.NewTicketLogic,
		logic.NewOrderLogic,
		logic.NewBillLogic,

		// MQ Consumer & Workers
		rabbitmq.NewConsumer,
		provider.ProvideTicketExpirerInterval,
		worker.NewTicketExpirer,

		// Handlers
		handlers.NewTicketHandler,
		handlers.NewBillStatusUpdateHandler,
		provideHandlers,

		// Final App
		NewConsumerApp,
	)
	return nil, nil, nil
}
