package conf

import (
	"fmt"
	"github.com/joho/godotenv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// KetoConfig holds the Ory Keto configuration.
type KetoConfig struct {
	ReadAddr  string `mapstructure:"read_addr"`
	WriteAddr string `mapstructure:"write_addr"`
}

// AppConfig holds the application configuration.
type AppConfig struct {
	Mode                string `mapstructure:"mode"`
	Port                int    `mapstructure:"port"`
	Name                string `mapstructure:"name"`
	Version             string `mapstructure:"version"`
	TimeZone            string `mapstructure:"time_zone"`
	*LogConfig          `mapstructure:"log"`
	*MongodbConfig      `mapstructure:"mongodb"`
	*KetoConfig         `mapstructure:"keto"`
	*WorkerConfig       `mapstructure:"worker"`
	*RabbitMQConfig     `mapstructure:"rabbitmq"`
	*EventServiceConfig `mapstructure:"event_service"`
	*JwtConfig          `mapstructure:"jwt"`
	*RedisConfig        `mapstructure:"redis"`
	*RateLimiterConfig  `mapstructure:"rate_limiter"`
}

// JwtConfig holds the JWT configuration.
type JwtConfig struct {
	Algorithm      string `mapstructure:"algorithm"`
	Secret         string `mapstructure:"secret"`
	PrivateKeyFile string `mapstructure:"private_key_file"`
	PublicKeyFile  string `mapstructure:"public_key_file"`
}

// EventServiceConfig holds the event service client configuration.
type EventServiceConfig struct {
	Addr string `mapstructure:"addr"`
}

// MongodbConfig holds the MongoDB configuration.
type MongodbConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DB       string `mapstructure:"db"`
}

// LogConfig holds the logger configuration.
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
}

// WorkerConfig holds all background worker configurations.
type WorkerConfig struct {
	Outbox        OutboxWorkerConfig  `mapstructure:"outbox"`
	TicketExpirer TicketExpirerConfig `mapstructure:"ticket_expirer"`
}

// TicketExpirerConfig holds the configuration for the ticket expirer worker.
type TicketExpirerConfig struct {
	IntervalSeconds int `mapstructure:"interval_seconds"`
}

// OutboxWorkerConfig holds the configuration for the outbox polling worker.
type OutboxWorkerConfig struct {
	IntervalSeconds int `mapstructure:"interval_seconds"`
	BatchSize       int `mapstructure:"batch_size"`
}

// RabbitMQConfig holds the RabbitMQ configuration.
type RabbitMQConfig struct {
	Host                  string `mapstructure:"host"`
	Port                  int    `mapstructure:"port"`
	User                  string `mapstructure:"user"`
	Password              string `mapstructure:"password"`
	GenerateTicketsTopic  string `mapstructure:"generate_tickets_topic"`
	PaymentEventTopic     string `mapstructure:"payment_event_topic"`
	UpdateBillStatusTopic string `mapstructure:"update_bill_status_topic"`
}

// RedisConfig holds the Redis client configuration.
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// RateLimiterPolicy defines the limit and interval for a policy.
type RateLimiterPolicy struct {
	Interval string `mapstructure:"interval"` // e.g., "1s", "1m", "1h"
	Limit    int    `mapstructure:"limit"`
}

// RateLimiterConfig holds all rate limiting policies.
type RateLimiterConfig struct {
	Default  RateLimiterPolicy            `mapstructure:"default"`
	Policies map[string]RateLimiterPolicy `mapstructure:"policies"`
}

// NewConfig loads the application configuration from a file.
func NewConfig(confFile string) (*AppConfig, error) {
	// Load .env file. It's okay if it doesn't exist. Errors are ignored.
	// This is mainly for local development.
	_ = godotenv.Load()

	v := viper.New()
	v.SetConfigFile(confFile)

	// Replace dots in keys with underscores for environment variables (e.g., `mongodb.host` -> `MONGODB_HOST`).
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// Enable automatic reading of environment variables.
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var conf AppConfig
	if err := v.Unmarshal(&conf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set timezone
	loc, err := time.LoadLocation(conf.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone: %w", err)
	}
	time.Local = loc

	return &conf, nil
}
