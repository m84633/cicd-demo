package provider

import (
	"context"
	"fmt"
	"os"
	"partivo_tickets/internal/client/events"
	"partivo_tickets/internal/conf"
	"partivo_tickets/internal/db"
	"partivo_tickets/internal/logic"
	"partivo_tickets/pkg/jwt"
	"partivo_tickets/pkg/relation"
	"strconv"
	"strings"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

// --- Type-safe configuration values for dependency injection ---

type AppName string
type AppMode string

// RedisNamespace is a custom type for the Redis key namespace.
type RedisNamespace string

func ProvideAppName(c *conf.AppConfig) AppName {
	return AppName(c.Name)
}

func ProvideAppMode(c *conf.AppConfig) AppMode {
	return AppMode(c.Mode)
}

// --- Providers for application components ---

// ProvideDatabase creates a new database instance from a client and config.
func ProvideDatabase(client *mongo.Client, cfg *conf.MongodbConfig) *mongo.Database {
	return client.Database(cfg.DB)
}

// ProvideRelationClient is a new provider for the relation.Client
func ProvideRelationClient(appConfig *conf.AppConfig) (*relation.Client, func(), error) {
	// Assuming keto addresses are in the config. Adjust if necessary.
	cfg := relation.Config{
		ReadAddr:  appConfig.KetoConfig.ReadAddr,
		WriteAddr: appConfig.KetoConfig.WriteAddr,
	}
	return relation.NewClient(cfg)
}

// ProvideMachineID attempts to parse a numeric id from the hostname (e.g., for StatefulSets).
// It defaults to 1 if parsing fails, which is safe for single-instance/dev environments.
func ProvideMachineID() uint16 {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Printf("WARN: Cannot get hostname, defaulting machine id to 1: %v\n", err)
		return 1
	}

	parts := strings.Split(hostname, "-")
	if len(parts) < 2 {
		fmt.Printf("WARN: Hostname '%s' does not fit 'name-id' format, defaulting machine id to 1\n", hostname)
		return 1
	}

	id, err := strconv.ParseUint(parts[len(parts)-1], 10, 16)
	if err != nil {
		fmt.Printf("WARN: Cannot parse id from hostname '%s', defaulting machine id to 1: %v\n", hostname, err)
		return 1
	}

	return uint16(id)
}

// ProvideGenerateTicketsTopic extracts the specific topic name from the app config.
func ProvideGenerateTicketsTopic(appConfig *conf.AppConfig) logic.GenerateTicketsTopic {
	return logic.GenerateTicketsTopic(appConfig.RabbitMQConfig.GenerateTicketsTopic)
}

// ProvidePaymentEventTopic extracts the specific topic name from the app config.
func ProvidePaymentEventTopic(appConfig *conf.AppConfig) logic.PaymentEventTopic {
	return logic.PaymentEventTopic(appConfig.RabbitMQConfig.PaymentEventTopic)
}

// ProvideTransactionManager decides which TransactionManager to use based on the app mode.
func ProvideTransactionManager(mode AppMode, client *mongo.Client) db.TransactionManager {
	if mode == "dev" || mode == "test" {
		// In dev/test mode, use the one that does nothing.
		return db.NewNoOpTransactionManager()
	}
	// In production, use the real one.
	return db.NewMongoTransactionManager(client)
}

// ProvideJwtGenerator creates a new JWT generator based on the app configuration.
func ProvideJwtGenerator(cfg *conf.AppConfig) (*jwt.Manager, error) {
	issuer := cfg.Name

	switch cfg.JwtConfig.Algorithm {
	case "HS256":
		return jwt.NewSymmetric([]byte(cfg.JwtConfig.Secret), issuer)
	case "RS256":
		// Read private key
		privateKeyData, err := os.ReadFile(cfg.JwtConfig.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
		privateKey, err := gojwt.ParseRSAPrivateKeyFromPEM(privateKeyData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		// Read public key
		publicKeyData, err := os.ReadFile(cfg.JwtConfig.PublicKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read public key file: %w", err)
		}
		publicKey, err := gojwt.ParseRSAPublicKeyFromPEM(publicKeyData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}

		return jwt.NewAsymmetric(privateKey, publicKey, issuer)
	default:
		return nil, fmt.Errorf("unsupported JWT algorithm: %s", cfg.JwtConfig.Algorithm)
	}
}

// ProvideEventsClient creates a new events service client.
func ProvideEventsClient(cfg *conf.AppConfig) (*events.Client, func(), error) {
	return events.NewClient(cfg.EventServiceConfig.Addr)
}

// ProvideTicketExpirerInterval provides the ticket expirer interval from the config.
func ProvideTicketExpirerInterval(cfg *conf.WorkerConfig) time.Duration {
	return time.Duration(cfg.TicketExpirer.IntervalSeconds) * time.Second
}

// ProvideRedisNamespace creates a namespace string for Redis keys.
func ProvideRedisNamespace(cfg *conf.AppConfig) RedisNamespace {
	return RedisNamespace(fmt.Sprintf("%s:%s:", cfg.Name, cfg.Mode))
}

// ProvideRedisClient creates and returns a new Redis client based on the application configuration.
// It also returns a cleanup function to close the connection.
func ProvideRedisClient(cfg *conf.RedisConfig) (*redis.Client, func(), error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Check the connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	cleanup := func() {
		client.Close()
	}

	return client, cleanup, nil
}
