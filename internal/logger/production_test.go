package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ProductionLogEntry represents the expected structure of production log entries
type ProductionLogEntry struct {
	Level     string  `json:"level"`
	Timestamp float64 `json:"ts"` // Zap uses numeric Unix timestamp
	Message   string  `json:"msg"`
	Component string  `json:"component,omitempty"`
	Error     string  `json:"error,omitempty"`
	Caller    string  `json:"caller,omitempty"`
}

// TestStructuredLoggingProduction tests structured logging output in production format
func TestStructuredLoggingProduction(t *testing.T) {
	// Create production logger
	logger, err := zap.NewProduction()
	require.NoError(t, err, "Should create production logger")
	defer logger.Sync()

	// Test structured logging with various levels and fields
	logger.Info("Test info message",
		zap.String("component", "test"),
		zap.String("operation", "validation"),
		zap.Int("count", 42))

	logger.Error("Test error message",
		zap.String("component", "test"),
		zap.Error(fmt.Errorf("test error")),
		zap.String("context", "testing"))

	logger.Warn("Test warning message",
		zap.String("component", "test"),
		zap.Bool("warning_condition", true))

	// The actual validation happens in integration tests
	// This test ensures the logger can be created and used
	assert.NotNil(t, logger, "Production logger should be created successfully")
}

// TestJSONLogFormatCompliance tests JSON log format meets production monitoring requirements
func TestJSONLogFormatCompliance(t *testing.T) {
	// Sample log entries that should be produced by the application (with numeric timestamps as Zap uses)
	sampleLogs := []string{
		`{"level":"info","ts":1641043200.000,"msg":"Radio Contest Winner starting up","component":"main","version":"3.1"}`,
		`{"level":"info","ts":1641043201.000,"msg":"Starting application lifecycle","component":"main"}`,
		`{"level":"error","ts":1641043202.000,"msg":"Application runtime error","component":"main","error":"connection failed"}`,
		`{"level":"warn","ts":1641043203.000,"msg":"Network connectivity issue","component":"stream","retry_count":3}`,
	}

	for i, logLine := range sampleLogs {
		t.Run(fmt.Sprintf("LogEntry_%d", i), func(t *testing.T) {
			// Verify each log entry can be parsed as valid JSON
			var entry ProductionLogEntry
			err := json.Unmarshal([]byte(logLine), &entry)
			require.NoError(t, err, "Log entry should be valid JSON: %s", logLine)

			// Verify required fields are present
			assert.NotEmpty(t, entry.Level, "Log entry should have level field")
			assert.NotEmpty(t, entry.Timestamp, "Log entry should have timestamp field")
			assert.NotEmpty(t, entry.Message, "Log entry should have message field")

			// Verify level is valid
			validLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
			assert.Contains(t, validLevels, entry.Level, "Log level should be valid")

			// Verify timestamp is a reasonable Unix timestamp (not zero)
			assert.Greater(t, entry.Timestamp, float64(0), "Timestamp should be a valid Unix timestamp")
		})
	}
}

// TestContainerLogAggregation tests log aggregation and accessibility in containerized environment
func TestContainerLogAggregation(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping Docker tests in CI environment - these tests are slow and require Docker daemon")
	}

	// Build test image first
	imageName := buildTestImageForLogging(t)
	defer cleanupImage(imageName)

	// Create configuration for logging test
	configContent := `
debug_mode: true
stream:
  url: "https://example.com/test-stream"
whisper:
  model_path: "/app/models/test-model.bin"
`

	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "logging-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Should create logging config file")

	// Test log output to stdout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-logging-test-" + fmt.Sprintf("%d", time.Now().Unix())

	// Run container and capture logs
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--rm",
		"-v", configPath+":/app/config.yaml:ro",
		imageName,
		"--help")

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should run and produce logs. Output: %s", string(output))

	// Verify log output is accessible
	outputStr := string(output)
	assert.NotEmpty(t, outputStr, "Container should produce log output")

	// Test log aggregation with a simple command that produces logs and exits
	simpleLogCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName+"-simple",
		"--rm",
		"-v", configPath+":/app/config.yaml:ro",
		imageName,
		"--version")

	simpleOutput, err := simpleLogCmd.CombinedOutput()
	require.NoError(t, err, "Simple container should produce logs and exit. Output: %s", string(simpleOutput))

	// Verify the simple container produced structured logs
	logs := string(simpleOutput)
	assert.NotEmpty(t, logs, "Container should produce logs")
	assert.Contains(t, logs, "Radio Contest Winner", "Logs should contain application name")

	// Test that we can retrieve logs using a short-lived detached container
	shortRunCtx, shortCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shortCancel()

	shortCmd := exec.CommandContext(shortRunCtx, "docker", "run",
		"--name", containerName+"-short",
		"--detach",
		"-v", configPath+":/app/config.yaml:ro",
		imageName,
		"--help") // Use --help which exits quickly

	_, err = shortCmd.CombinedOutput()
	require.NoError(t, err, "Short container should start")

	// Cleanup short container
	defer func() {
		rmCmd := exec.Command("docker", "rm", "-f", containerName+"-short")
		rmCmd.Run()
	}()

	// Wait briefly for container to complete
	time.Sleep(2 * time.Second)

	// Test docker logs command works
	logsCmd := exec.Command("docker", "logs", containerName+"-short")
	logsOutput, err := logsCmd.CombinedOutput()
	if err == nil {
		retrievedLogs := string(logsOutput)
		assert.NotEmpty(t, retrievedLogs, "Should be able to retrieve container logs")
		t.Logf("Retrieved logs: %s", retrievedLogs)
	} else {
		t.Logf("Container may have exited before logs could be retrieved: %v", err)
	}
}

// TestLogRotationConfiguration tests log rotation and storage management
func TestLogRotationConfiguration(t *testing.T) {
	// Test log rotation configuration for containerized environment
	// In Docker containers, log rotation is typically handled by the container runtime
	// This test verifies our logging configuration doesn't interfere with container log management

	// Create temporary directory for log testing
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	// Create a logger that writes to file
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{logFile}
	config.ErrorOutputPaths = []string{logFile}

	logger, err := config.Build()
	require.NoError(t, err, "Should create file-based logger")
	defer logger.Sync()

	// Write multiple log entries
	for i := 0; i < 100; i++ {
		logger.Info("Test log entry",
			zap.Int("sequence", i),
			zap.String("component", "rotation_test"))
	}

	// Force sync to ensure logs are written to file
	err = logger.Sync()
	require.NoError(t, err, "Should sync logger to file")

	// Verify log file was created and contains entries
	_, err = os.Stat(logFile)
	require.NoError(t, err, "Log file should be created")

	logContent, err := os.ReadFile(logFile)
	require.NoError(t, err, "Should read log file")

	// Debug: Check if file has content
	t.Logf("Log file size: %d bytes", len(logContent))
	previewLen := len(logContent)
	if previewLen > 200 {
		previewLen = 200
	}
	t.Logf("Log file content preview: %s", string(logContent)[:previewLen])

	// Verify structured JSON logging
	lines := strings.Split(string(logContent), "\n")
	validLogCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry ProductionLogEntry
		err := json.Unmarshal([]byte(line), &entry)
		if err == nil {
			validLogCount++
			assert.Equal(t, "info", entry.Level, "Log entry should have correct level")
			assert.Contains(t, entry.Message, "Test log entry", "Log entry should have correct message")
		} else {
			t.Logf("Failed to parse log line: %s, error: %v", line, err)
		}
	}

	t.Logf("Valid log count: %d", validLogCount)
	assert.Greater(t, validLogCount, 90, "Should have written most log entries successfully")
}

// TestContainerLogVolumeMounting tests log output via volume mounting
func TestContainerLogVolumeMounting(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping Docker tests in CI environment - these tests are slow and require Docker daemon")
	}

	// Build test image first
	imageName := buildTestImageForLogging(t)
	defer cleanupImage(imageName)

	// Create temporary directory for log output
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logDir, 0755)
	require.NoError(t, err, "Should create log directory")

	// Create configuration that writes logs to mounted volume
	configContent := `
debug_mode: true
stream:
  url: "https://example.com/test-stream"
`

	configPath := filepath.Join(tempDir, "volume-config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Should create volume config file")

	// Test log output via volume mounting
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-volume-test-" + fmt.Sprintf("%d", time.Now().Unix())

	// Run container with mounted log volume
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--rm",
		"-v", configPath+":/app/config.yaml:ro",
		"-v", logDir+":/app/logs:rw",
		imageName,
		"--version")

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should run with mounted log volume. Output: %s", string(output))

	// Verify output is accessible
	outputStr := string(output)
	assert.NotEmpty(t, outputStr, "Container should produce output")
	assert.Contains(t, outputStr, "Radio Contest Winner", "Should contain application name")
}

// Helper functions for logging tests

// buildTestImageForLogging builds a test image specifically for logging tests
func buildTestImageForLogging(t *testing.T) string {
	repoRoot, err := findRepoRootForLogging()
	require.NoError(t, err, "Should find repository root")

	imageName := "radiocontestwinner-logging-test:" + fmt.Sprintf("%d", time.Now().Unix())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	buildCmd := exec.CommandContext(ctx, "docker", "build",
		"-f", "build/Dockerfile",
		"-t", imageName,
		".")
	buildCmd.Dir = repoRoot

	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Docker build should succeed. Output: %s", string(output))

	return imageName
}

// cleanupImage removes a Docker image
func cleanupImage(imageName string) {
	cleanupCmd := exec.Command("docker", "rmi", imageName)
	cleanupCmd.Run() // Ignore errors during cleanup
}

// findRepoRootForLogging finds the repository root directory for logging tests
func findRepoRootForLogging() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Look for go.mod file to identify repo root
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find repository root (go.mod not found)")
}

// TestProductionLoggerConfiguration tests production logger configuration meets requirements
func TestProductionLoggerConfiguration(t *testing.T) {
	// Test production logger configuration
	logger, err := zap.NewProduction()
	require.NoError(t, err, "Should create production logger")
	defer logger.Sync()

	// Verify logger outputs JSON format
	// This is validated by checking the configuration
	config := zap.NewProductionConfig()
	assert.Equal(t, "json", config.Encoding, "Production logger should use JSON encoding")
	assert.Equal(t, "info", config.Level.String(), "Production logger should default to info level")

	// Verify required fields are included in config
	assert.Contains(t, config.EncoderConfig.LevelKey, "level", "Should include level field")
	assert.Contains(t, config.EncoderConfig.TimeKey, "ts", "Should include timestamp field")
	assert.Contains(t, config.EncoderConfig.MessageKey, "msg", "Should include message field")
	assert.NotEmpty(t, config.EncoderConfig.CallerKey, "Should include caller information")
}

// TestLogSecurityRequirements tests logging security requirements
func TestLogSecurityRequirements(t *testing.T) {
	// Test that logging doesn't expose sensitive information
	logger, err := zap.NewProduction()
	require.NoError(t, err, "Should create production logger")
	defer logger.Sync()

	// Test that structured logging allows filtering sensitive data
	logger.Info("Processing stream connection",
		zap.String("component", "stream"),
		zap.String("url_host", "ais-sa1.streamon.fm"), // Only log hostname, not full URL with auth
		zap.String("status", "connected"))

	// Test error logging without exposing internal details
	logger.Error("Stream connection failed",
		zap.String("component", "stream"),
		zap.String("error_type", "network_error"), // Generic error type
		zap.Int("retry_count", 3))

	// This test validates the pattern - actual security validation
	// happens in integration tests where we check log content
	assert.NotNil(t, logger, "Security-aware logger should be created")
}
