package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"

	"partivo_tickets/internal/conf"

	"go.uber.org/zap"
)

func main() {
	// Command-line flag for config file path
	confPath := flag.String("c", "internal/conf/config.yaml", "path to config file")
	flag.Parse()

	// Load configuration
	appConfig, err := conf.NewConfig(*confPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	// Initialize app using Wire
	app, cleanup, err := InitializeConsumerApp(appConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize consumer app: %v", err))
	}
	defer cleanup()

	// Create a context that is cancelled on interruption signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run the application
	app.logger.Info("Starting consumer application")
	if err := app.Run(ctx); err != nil {
		app.logger.Error("Consumer application exited with error", zap.Error(err))
	}

	app.logger.Info("Consumer application shut down gracefully")
}
