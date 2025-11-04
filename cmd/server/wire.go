//go:build wireinject
// +build wireinject

package main

import (
	"partivo_tickets/internal/app"
	"partivo_tickets/internal/conf"
	"partivo_tickets/internal/dao/mongodb"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/limiter"
	"partivo_tickets/internal/logger"
	"partivo_tickets/internal/logic"
	"partivo_tickets/internal/middleware/http"
	"partivo_tickets/internal/mq"
	"partivo_tickets/internal/mq/rabbitmq"
	"partivo_tickets/internal/provider"
	"partivo_tickets/internal/service"
	"partivo_tickets/internal/worker"
	"partivo_tickets/pkg/snowflake"

	"github.com/google/wire"
)

// ------------------- 1. 定義 Provider 集合 -------------------

// baseProviders 包含所有應用共用的基礎元件 (不含 MQ)
var baseProviders = wire.NewSet(
	wire.FieldsOf(new(*conf.AppConfig), "LogConfig", "MongodbConfig", "KetoConfig", "WorkerConfig", "EventServiceConfig", "JwtConfig", "RedisConfig", "RateLimiterConfig"),
	provider.ProvideAppMode,
	logger.NewLogger,
	mongodb.NewMongoDB,
	provider.ProvideDatabase,
	provider.ProvideRelationClient,
	provider.ProvideMachineID,
	provider.ProvideGenerateTicketsTopic,
	provider.ProvidePaymentEventTopic,
	provider.ProvideTransactionManager,
	provider.ProvideJwtGenerator,
	provider.ProvideEventsClient,
	provider.ProvideRedisNamespace,
	provider.ProvideRedisClient,
	limiter.NewManager,
	snowflake.NewGenerator,
	mongodb.NewTicketsDAO,
	wire.Bind(new(repository.TicketsRepository), new(*mongodb.TicketsDAO)),
	mongodb.NewOrdersDAO,
	wire.Bind(new(repository.OrdersRepository), new(*mongodb.OrdersDAO)),
	mongodb.NewBillDAO,
	wire.Bind(new(repository.BillRepository), new(*mongodb.BillDAO)),
	mongodb.NewAuditLogDAO,
	wire.Bind(new(repository.AuditLogRepository), new(*mongodb.AuditLogDAO)),
	mongodb.NewOutboxDAO,
	wire.Bind(new(repository.OutboxRepository), new(*mongodb.OutboxDAO)),
	logic.NewPaymentEventPublisher,
	logic.NewTicketEventPublisher,
	logic.NewTicketLogic,
	logic.OrderLogicProviderSet,
	logic.NewBillLogic,
)

// rabbitMQProviders 包含 RabbitMQ 的 Publisher 和 Worker
var rabbitMQProviders = wire.NewSet(
	wire.FieldsOf(new(*conf.AppConfig), "RabbitMQConfig"),
	rabbitmq.NewPublisher,
	wire.Bind(new(mq.Publisher), new(*rabbitmq.Publisher)),
	worker.NewOutboxProcessor, // 提供具體的 OutboxProcessor
)

// ------------------- 2. Frontend App 的注入器 -------------------

func provideFrontendServices(ticketService *service.TicketsService, orderService *service.OrderService, billService *service.BillService) []interface{} {
	return []interface{}{ticketService, orderService, billService}
}

// provideFrontendWorkers 為前端應用提供一個空的 Worker 切片
func provideFrontendWorkers() []worker.Worker {
	return []worker.Worker{} // 回傳空切片
}

// provideNilHttpHandlerRegister provides a nil register for apps that don't have custom handlers.
func provideNilHttpHandlerRegister() app.HttpHandlerRegister {
	return nil
}

func InitializeFrontendApp(appConfig *conf.AppConfig) (*app.App, func(), error) {
	wire.Build(
		baseProviders,
		wire.FieldsOf(new(*conf.AppConfig), "Port"),
		service.NewTicketsService,
		service.NewOrderService,
		service.NewBillService,
		provideFrontendServices,
		provideFrontendWorkers, // 提供空的 Worker 切片
		provideNilHttpHandlerRegister,
		conf.NewUnaryInterceptors,
		conf.NewAllowedHeaders,
		app.NewApp,
	)
	return nil, nil, nil
}

// ------------------- 3. Console App 的注入器 -------------------

func provideConsoleServices(
	ordersAdminService *service.OrdersAdminService,
	ticketsAdminService *service.TicketsAdminService,
	billsAdminService *service.BillsAdminService,
) []interface{} {
	return []interface{}{ordersAdminService, ticketsAdminService, billsAdminService}
}

// provideConsoleWorkers 將 OutboxProcessor 包裝成 Worker 切片
func provideConsoleWorkers(p *worker.OutboxProcessor) []worker.Worker {
	return []worker.Worker{p} // 回傳包含 OutboxProcessor 的切片
}

func InitializeConsoleApp(appConfig *conf.AppConfig) (*app.App, func(), error) {
	wire.Build(
		baseProviders,
		rabbitMQProviders,
		wire.FieldsOf(new(*conf.AppConfig), "Port"),
		service.NewOrdersAdminService,
		service.NewTicketsAdminService,
		service.NewBillsAdminService,
		service.NewOrderExportHandler, // Add the new handler
		app.NewHttpHandlerRegister,    // Add the new registrar from app package
		http.NewAuthMiddleware,        // Add the new auth middleware provider
		provideConsoleServices,
		provideConsoleWorkers, // 提供包含 OutboxProcessor 的 Worker 切片
		conf.NewUnaryInterceptors,
		conf.NewAllowedHeaders,
		app.NewApp,
	)
	return nil, nil, nil
}
