package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"radiocontestwinner/internal/app"
)

// main is the application entry point and orchestrator setup
func main() {
	// Parse command line flags
	var (
		helpFlag    = flag.Bool("help", false, "Show help message")
		versionFlag = flag.Bool("version", false, "Show version information")
		healthFlag  = flag.Bool("health", false, "Check application health status")
	)
	flag.Parse()

	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	if *versionFlag {
		printVersion()
		os.Exit(0)
	}

	if *healthFlag {
		exitCode := checkHealth()
		os.Exit(exitCode)
	}

	// Run the main application logic
	if err := runApplication(); err != nil {
		fmt.Fprintf(os.Stderr, "Application error: %v\n", err)
		os.Exit(1)
	}
}

// runApplication contains the core application logic that can be tested
func runApplication() error {
	// Create structured logger for main
	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Sync()

	// Log application startup
	logger.Info("Radio Contest Winner starting up",
		zap.String("component", "main"),
		zap.String("version", "3.1"))

	// Create application instance using orchestrator
	application, err := app.NewApplication()
	if err != nil {
		logger.Error("Failed to create application",
			zap.Error(err),
			zap.String("component", "main"))
		return fmt.Errorf("failed to create application: %w", err)
	}

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start signal handler goroutine
	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal",
			zap.String("signal", sig.String()),
			zap.String("component", "main"))
		cancel()
	}()

	// Run the application
	logger.Info("Starting application lifecycle",
		zap.String("component", "main"))

	if err := application.Run(ctx); err != nil {
		logger.Error("Application runtime error",
			zap.Error(err),
			zap.String("component", "main"))
		return fmt.Errorf("application runtime error: %w", err)
	}

	// Perform explicit shutdown
	logger.Info("Performing application shutdown",
		zap.String("component", "main"))

	if err := application.Shutdown(); err != nil {
		logger.Error("Error during application shutdown",
			zap.Error(err),
			zap.String("component", "main"))
		return fmt.Errorf("application shutdown error: %w", err)
	}

	logger.Info("Radio Contest Winner stopped successfully",
		zap.String("component", "main"))
	return nil
}

// printHelp displays command line usage information
func printHelp() {
	fmt.Println("Radio Contest Winner - Audio Stream Transcription and Contest Detection")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("    radiocontestwinner [OPTIONS]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("    -help      Show this help message")
	fmt.Println("    -version   Show version information")
	fmt.Println("    -health    Check application health status")
	fmt.Println()
	fmt.Println("CONFIGURATION:")
	fmt.Println("    Configuration is loaded from environment variables.")
	fmt.Println("    See config.example.yaml for available options.")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("    radiocontestwinner              # Run with default configuration")
	fmt.Println("    radiocontestwinner -help        # Show this help")
	fmt.Println("    radiocontestwinner -version     # Show version")
	fmt.Println("    radiocontestwinner -health      # Check health (for Docker healthcheck)")
}

// printVersion displays version and build information
func printVersion() {
	fmt.Println("Radio Contest Winner")
	fmt.Println("Version: 3.1")
	fmt.Println("Build: Application Orchestrator Implementation")
	fmt.Println("Architecture: Go 1.24 + FFmpeg + Whisper.cpp")
}

// checkHealth checks the application health status by reading the health file
func checkHealth() int {
	return checkHealthWithFile("/tmp/radiocontestwinner-health.json")
}

// checkHealthWithFile checks the application health status by reading the specified health file
func checkHealthWithFile(healthFile string) int {

	// Check if health file exists
	if _, err := os.Stat(healthFile); os.IsNotExist(err) {
		fmt.Printf("UNHEALTHY: Health status file not found (%s)\n", healthFile)
		return 1
	}

	// Read health file
	data, err := os.ReadFile(healthFile)
	if err != nil {
		fmt.Printf("UNHEALTHY: Failed to read health file: %v\n", err)
		return 1
	}

	// Parse health status
	var healthStatus map[string]interface{}
	if err := json.Unmarshal(data, &healthStatus); err != nil {
		fmt.Printf("UNHEALTHY: Failed to parse health file: %v\n", err)
		return 1
	}

	// Check if health check timestamp is recent (within last 90 seconds)
	timestampStr, ok := healthStatus["health_check_timestamp"].(string)
	if !ok {
		fmt.Println("UNHEALTHY: Health file missing timestamp")
		return 1
	}

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		fmt.Printf("UNHEALTHY: Invalid timestamp format: %v\n", err)
		return 1
	}

	timeSinceUpdate := time.Since(timestamp)
	if timeSinceUpdate > 90*time.Second {
		fmt.Printf("UNHEALTHY: Health file is stale (last update: %v ago)\n", timeSinceUpdate)
		return 1
	}

	// Check overall health status
	healthy, ok := healthStatus["healthy"].(bool)
	if !ok {
		fmt.Println("UNHEALTHY: Health status missing healthy field")
		return 1
	}

	if !healthy {
		fmt.Println("UNHEALTHY: Application reported unhealthy status")
		fmt.Printf("Health details: %s\n", string(data))
		return 1
	}

	// System is healthy
	fmt.Printf("HEALTHY: Application is functioning normally (last check: %v ago)\n", timeSinceUpdate)
	return 0
}
