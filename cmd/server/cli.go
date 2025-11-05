package main

import (
	"fmt"
	"log"
	"os"
	"partivo_tickets/internal/conf"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "partivo_tickets",
	Short: "Partivo Stocks Service",
	Long:  `The main entry point for the Partivo Stocks service.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func loadConfig(cmd *cobra.Command) (*conf.AppConfig, error) {
	confFile, _ := cmd.Flags().GetString("config")
	appConfig, err := conf.NewConfig(confFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	port, _ := cmd.Flags().GetInt("port")
	if port > 0 {
		appConfig.Port = port
	}

	return appConfig, nil
}

var frontendCmd = &cobra.Command{
	Use:   "serve:frontend",
	Short: "Starts the frontend gRPC server",
	Long:  `Starts the gRPC server for public (frontend) clients.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		appConfig, err := loadConfig(cmd)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}

		// Initialize application using wire-generated function
		app, cleanup, err := InitializeFrontendApp(appConfig)
		if err != nil {
			log.Fatalf("failed to init frontend app: %v", err)
		}
		defer cleanup()

		// Run application
		if err := app.Run(); err != nil {
			log.Fatalf("failed to run frontend app: %v", err)
		}
	},
}

var consoleCmd = &cobra.Command{
	Use:   "serve:console",
	Short: "Starts the console gRPC server",
	Long:  `Starts the gRPC server for internal (console) clients.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		appConfig, err := loadConfig(cmd)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}

		// Initialize application using wire-generated function
		app, cleanup, err := InitializeConsoleApp(appConfig)
		if err != nil {
			log.Fatalf("failed to init console app: %v", err)
		}
		defer cleanup()

		// Run application
		if err := app.Run(); err != nil {
			log.Fatalf("failed to run console app: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(frontendCmd)
	rootCmd.AddCommand(consoleCmd)
	rootCmd.PersistentFlags().IntP("port", "p", 0, "Port for the server to listen on, overrides the value in the config file")
	rootCmd.PersistentFlags().StringP("config", "c", "internal/conf/config.yaml", "path to config file")
}
