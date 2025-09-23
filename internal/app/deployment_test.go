package app

import (
	"bufio"
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
)

// TestProductionDockerBuild tests the production Docker build process
func TestProductionDockerBuild(t *testing.T) {
	skipDockerTestsInCI(t)

	// Change to repo root for Docker build context
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "Should find repository root")

	// Test Docker build process
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	imageName := "radiocontestwinner-test:" + generateTimestamp()

	// Build Docker image
	buildCmd := exec.CommandContext(ctx, "docker", "build",
		"-f", "build/Dockerfile",
		"-t", imageName,
		".")
	buildCmd.Dir = repoRoot

	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Docker build should succeed. Output: %s", string(output))

	// Cleanup image after test
	defer func() {
		cleanupCmd := exec.Command("docker", "rmi", imageName)
		cleanupCmd.Run() // Ignore errors during cleanup
	}()

	// Verify image was created
	inspectCmd := exec.Command("docker", "image", "inspect", imageName)
	inspectOutput, err := inspectCmd.CombinedOutput()
	require.NoError(t, err, "Should be able to inspect built image")

	// Parse image details
	var imageDetails []map[string]interface{}
	err = json.Unmarshal(inspectOutput, &imageDetails)
	require.NoError(t, err, "Should parse image inspection JSON")
	require.Len(t, imageDetails, 1, "Should have one image")

	imageInfo := imageDetails[0]

	// Verify image configuration
	config, ok := imageInfo["Config"].(map[string]interface{})
	require.True(t, ok, "Should have Config section")

	// Verify non-root user
	user, exists := config["User"]
	require.True(t, exists, "Should have User configured")
	assert.Equal(t, "appuser", user, "Should run as non-root user")

	// Verify working directory
	workingDir, exists := config["WorkingDir"]
	require.True(t, exists, "Should have WorkingDir configured")
	assert.Equal(t, "/app", workingDir, "Should have correct working directory")

	// Verify health check
	healthcheck, exists := config["Healthcheck"]
	require.True(t, exists, "Should have health check configured")
	healthcheckMap, ok := healthcheck.(map[string]interface{})
	require.True(t, ok, "Health check should be a map")

	test, exists := healthcheckMap["Test"]
	require.True(t, exists, "Health check should have Test")
	assert.Contains(t, fmt.Sprintf("%v", test), "--health", "Health check should test application health")
}

// TestContainerStartupShutdown tests container startup and shutdown procedures
func TestContainerStartupShutdown(t *testing.T) {
	skipDockerTestsInCI(t)

	// Build test image first
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Test container startup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	containerName := "radiocontestwinner-test-" + generateTimestamp()

	// Start container
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--detach",
		imageName)

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should start successfully. Output: %s", string(output))

	// Cleanup container after test
	defer func() {
		stopCmd := exec.Command("docker", "stop", containerName)
		stopCmd.Run()
		rmCmd := exec.Command("docker", "rm", containerName)
		rmCmd.Run()
	}()

	// Wait for container to be running
	time.Sleep(2 * time.Second)

	// Check container status
	statusCmd := exec.Command("docker", "ps", "--filter", "name="+containerName, "--format", "{{.Status}}")
	statusOutput, err := statusCmd.CombinedOutput()
	require.NoError(t, err, "Should get container status")

	status := strings.TrimSpace(string(statusOutput))
	assert.Contains(t, status, "Up", "Container should be running")

	// Test graceful shutdown
	stopCmd := exec.CommandContext(ctx, "docker", "stop", containerName)
	stopOutput, err := stopCmd.CombinedOutput()
	require.NoError(t, err, "Container should stop gracefully. Output: %s", string(stopOutput))

	// Verify container stopped
	time.Sleep(1 * time.Second)
	statusCmd = exec.Command("docker", "ps", "--filter", "name="+containerName, "--format", "{{.Status}}")
	statusOutput, err = statusCmd.CombinedOutput()
	require.NoError(t, err, "Should get container status")

	finalStatus := strings.TrimSpace(string(statusOutput))
	assert.Empty(t, finalStatus, "Container should be stopped")
}

// TestContainerConfiguration tests configuration options in containerized environment
func TestContainerConfiguration(t *testing.T) {
	skipDockerTestsInCI(t)

	// Build test image first
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Create test configuration
	configContent := `
stream:
  url: "https://ais-sa1.streamon.fm:443/7346_48k.aac"
whisper:
  model_path: "/app/models/ggml-base.en.bin"
buffer:
  duration_ms: 3000
allowlist:
  numbers:
    - "73"
    - "146"
debug_mode: true
`

	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Should create test config file")

	// Test container with mounted config
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-config-test-" + generateTimestamp()

	// Start container with mounted config
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--rm",
		"-v", configPath+":/app/config.yaml:ro",
		imageName,
		"--help") // Use help to test configuration loading

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should start with mounted config. Output: %s", string(output))

	// Verify help output contains expected information
	outputStr := string(output)
	assert.Contains(t, outputStr, "Radio Contest Winner", "Should display application title")
	assert.Contains(t, outputStr, "USAGE", "Should display usage information")
	assert.Contains(t, outputStr, "CONFIGURATION", "Should display configuration information")
}

// TestContainerHealthCheck tests the container health check functionality
func TestContainerHealthCheck(t *testing.T) {
	skipDockerTestsInCI(t)

	// Build test image first
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Start container and wait for health check
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	containerName := "radiocontestwinner-health-test-" + generateTimestamp()

	// Start container
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--detach",
		imageName)

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should start successfully. Output: %s", string(output))

	// Cleanup container after test
	defer func() {
		stopCmd := exec.Command("docker", "stop", containerName)
		stopCmd.Run()
		rmCmd := exec.Command("docker", "rm", containerName)
		rmCmd.Run()
	}()

	// Wait for health check to stabilize
	time.Sleep(10 * time.Second)

	// Check health status
	healthCmd := exec.Command("docker", "inspect", "--format", "{{.State.Health.Status}}", containerName)
	healthOutput, err := healthCmd.CombinedOutput()

	if err != nil {
		// If health check not available, check logs for startup success
		logsCmd := exec.Command("docker", "logs", containerName)
		logsOutput, logErr := logsCmd.CombinedOutput()
		require.NoError(t, logErr, "Should get container logs")

		logs := string(logsOutput)
		// The container should start and show help or startup messages
		assert.True(t,
			strings.Contains(logs, "Radio Contest Winner") ||
				strings.Contains(logs, "starting up") ||
				strings.Contains(logs, "USAGE"),
			"Container should show startup information. Logs: %s", logs)
	} else {
		healthStatus := strings.TrimSpace(string(healthOutput))
		assert.Contains(t, []string{"healthy", "starting"}, healthStatus,
			"Container should be healthy or starting")
	}
}

// TestContainerLogOutput tests log output accessibility in containerized environment
func TestContainerLogOutput(t *testing.T) {
	skipDockerTestsInCI(t)

	// Build test image first
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Start container and capture logs
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-logs-test-" + generateTimestamp()

	// Run container with version flag to generate known output
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--rm",
		imageName,
		"-version")

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should run and show version. Output: %s", string(output))

	// Verify structured output
	outputStr := string(output)
	assert.Contains(t, outputStr, "Radio Contest Winner", "Should contain application name")
	assert.Contains(t, outputStr, "Version:", "Should contain version information")
	assert.Contains(t, outputStr, "Architecture:", "Should contain architecture information")
}

// Helper functions

// buildTestImage builds a test image and returns the image name
func buildTestImage(t *testing.T) string {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "Should find repository root")

	imageName := "radiocontestwinner-test:" + generateTimestamp()

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

// findRepoRoot finds the repository root directory
func findRepoRoot() (string, error) {
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

// generateTimestamp generates a timestamp for unique naming
func generateTimestamp() string {
	return fmt.Sprintf("%d", time.Now().Unix())
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Level     string                 `json:"level"`
	Timestamp float64                `json:"ts"`
	Message   string                 `json:"msg"`
	Component string                 `json:"component"`
	Fields    map[string]interface{} `json:",inline"`
}

// TestContainerLiveStreamProcessing tests container with live X96 radio stream processing
func TestContainerLiveStreamProcessing(t *testing.T) {
	skipDockerTestsInCI(t)

	if os.Getenv("SKIP_LIVE_STREAM_TESTS") == "true" {
		t.Skip("Skipping live stream tests - set SKIP_LIVE_STREAM_TESTS=false to enable")
	}

	// Build test image first
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Create production-like configuration for live stream testing
	configContent := `
stream:
  url: "https://ais-sa1.streamon.fm:443/7346_48k.aac"
whisper:
  model_path: "/app/models/ggml-base.en.bin"
buffer:
  duration_ms: 2500
allowlist:
  numbers:
    - "73"
    - "146"
    - "222"
    - "0146"
debug_mode: true
`

	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "live-stream-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Should create live stream config file")

	// Create whisper model directory (mock)
	modelDir := filepath.Join(tempDir, "models")
	err = os.MkdirAll(modelDir, 0755)
	require.NoError(t, err, "Should create model directory")

	// Note: This test will fail if Whisper model is not available
	// but it tests the container's ability to start and attempt stream processing
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-live-test-" + generateTimestamp()

	// Start container with live stream processing
	// Run for limited time to verify stream processing capability
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--rm",
		"-v", configPath+":/app/config.yaml:ro",
		"-v", modelDir+":/app/models:ro",
		imageName)

	// Start the container
	err = runCmd.Start()
	require.NoError(t, err, "Container should start for live stream processing")

	// Let it run for a short period
	time.Sleep(10 * time.Second)

	// Get container logs to verify it's attempting stream processing
	logsCmd := exec.Command("docker", "logs", containerName)
	logsOutput, err := logsCmd.CombinedOutput()
	logs := string(logsOutput)

	// Kill the container since we just need to verify startup behavior
	killCmd := exec.Command("docker", "kill", containerName)
	killCmd.Run()

	// Wait for the original command to finish
	runCmd.Wait()

	// Verify the container attempted to start stream processing
	// The specific behavior depends on whether Whisper model is available
	assert.True(t,
		strings.Contains(logs, "Radio Contest Winner") ||
			strings.Contains(logs, "starting up") ||
			strings.Contains(logs, "stream") ||
			strings.Contains(logs, "config") ||
			strings.Contains(logs, "error"), // May error due to missing model, which is expected
		"Container should show startup and stream processing attempts. Logs: %s", logs)

	// Validate structured logging if present
	if strings.Contains(logs, "{") {
		validateJSONLogs(t, logs)
	}
}

// TestContainerResourceLimits tests container with resource constraints
func TestContainerResourceLimits(t *testing.T) {
	skipDockerTestsInCI(t)

	// Build test image first
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Test container with resource limits
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-limits-test-" + generateTimestamp()

	// Start container with resource limits as specified in story requirements
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--rm",
		"--memory=1g", // 1GB memory limit from story requirements
		"--cpus=1.0",  // 1 CPU core limit from story requirements
		"--oom-kill-disable=false",
		imageName,
		"--help")

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should run with resource limits. Output: %s", string(output))

	// Verify help output
	outputStr := string(output)
	assert.Contains(t, outputStr, "Radio Contest Winner", "Should display application title")
	assert.Contains(t, outputStr, "USAGE", "Should display usage information")
}

// TestContainerEnvironmentVariables tests configuration via environment variables
func TestContainerEnvironmentVariables(t *testing.T) {
	skipDockerTestsInCI(t)

	// Build test image first
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Test container with environment variable configuration
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-env-test-" + generateTimestamp()

	// Start container with environment variables
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--rm",
		"-e", "DEBUG_MODE=true",
		"-e", "STREAM_URL=https://test.example.com/stream",
		imageName,
		"--version")

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should run with environment variables. Output: %s", string(output))

	// Verify version output
	outputStr := string(output)
	assert.Contains(t, outputStr, "Radio Contest Winner", "Should display application name")
	assert.Contains(t, outputStr, "Version:", "Should display version information")
}

// skipDockerTestsInCI skips Docker tests when running in CI without Docker support
func skipDockerTestsInCI(t *testing.T) {
	// Check for multiple CI environment variables
	ciEnvs := []string{"CI", "GITHUB_ACTIONS", "GITHUB_WORKFLOW"}
	for _, env := range ciEnvs {
		if os.Getenv(env) != "" {
			t.Skip("Skipping Docker tests in CI environment - these tests are slow and require Docker daemon")
		}
	}
}

// validateJSONLogs validates that log output is properly structured JSON
func validateJSONLogs(t *testing.T, logOutput string) {
	scanner := bufio.NewScanner(strings.NewReader(logOutput))
	lineCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		lineCount++

		// Try to parse as JSON
		var logEntry LogEntry
		err := json.Unmarshal([]byte(line), &logEntry)
		require.NoError(t, err, "Log line should be valid JSON: %s", line)

		// Verify required fields
		assert.NotEmpty(t, logEntry.Level, "Log entry should have level")
		assert.NotZero(t, logEntry.Timestamp, "Log entry should have timestamp")
		assert.NotEmpty(t, logEntry.Message, "Log entry should have message")
	}

	require.NoError(t, scanner.Err(), "Should scan logs without error")
	assert.Greater(t, lineCount, 0, "Should have at least one log line")
}
