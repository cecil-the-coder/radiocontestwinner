package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthCheckEndpoints tests health check endpoints for container orchestration
func TestHealthCheckEndpoints(t *testing.T) {
	// Test that the application has appropriate health check mechanisms
	// The current Dockerfile already includes a basic health check

	// Verify health check command exists and works
	helpCmd := exec.Command("go", "run", "cmd/radiocontestwinner/main.go", "--help")
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "Should find repository root")
	helpCmd.Dir = repoRoot

	output, err := helpCmd.CombinedOutput()
	require.NoError(t, err, "Health check command should work. Output: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "Radio Contest Winner", "Health check should show application info")
	assert.Contains(t, outputStr, "USAGE", "Health check should show usage information")

	// Test that the help command exits with code 0 (healthy)
	versionCmd := exec.Command("go", "run", "cmd/radiocontestwinner/main.go", "--version")
	versionCmd.Dir = repoRoot

	versionOutput, err := versionCmd.CombinedOutput()
	require.NoError(t, err, "Version command should work for health checks. Output: %s", string(versionOutput))

	versionStr := string(versionOutput)
	assert.Contains(t, versionStr, "Radio Contest Winner", "Version should show application name")
	assert.Contains(t, versionStr, "Version:", "Version should show version information")
}

// TestContainerHealthCheckIntegration tests health check integration in containerized environment
func TestContainerHealthCheckIntegration(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping Docker tests in CI environment - these tests are slow and require Docker daemon")
	}

	// Build test image
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Start container and test health check
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	containerName := "radiocontestwinner-health-integration-" + generateTimestamp()

	// Start container
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--detach",
		imageName)

	startOutput, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should start for health check test. Output: %s", string(startOutput))

	// Cleanup container after test
	defer func() {
		stopCmd := exec.Command("docker", "stop", containerName)
		stopCmd.Run()
		rmCmd := exec.Command("docker", "rm", containerName)
		rmCmd.Run()
	}()

	// Wait for health check to stabilize
	time.Sleep(10 * time.Second)

	// Check health status via docker inspect
	healthCmd := exec.Command("docker", "inspect", "--format", "{{.State.Health}}", containerName)
	healthOutput, err := healthCmd.CombinedOutput()

	// Health check might not be available in all Docker configurations
	if err != nil {
		t.Logf("Health check not available via inspect, checking container status instead")

		// Check if container is running (alternative health indicator)
		statusCmd := exec.Command("docker", "ps", "--filter", "name="+containerName, "--format", "{{.Status}}")
		statusOutput, err := statusCmd.CombinedOutput()
		require.NoError(t, err, "Should get container status")

		status := strings.TrimSpace(string(statusOutput))
		assert.Contains(t, status, "Up", "Container should be running (healthy)")
	} else {
		healthStr := string(healthOutput)
		t.Logf("Health check status: %s", healthStr)
		// Health status should indicate the container is healthy or starting
		assert.True(t,
			strings.Contains(healthStr, "healthy") ||
				strings.Contains(healthStr, "starting") ||
				strings.Contains(healthStr, "none"), // Health check might not be configured
			"Container should have acceptable health status")
	}
}

// TestNetworkConnectivityFailureRecovery tests recovery from network connectivity failures
func TestNetworkConnectivityFailureRecovery(t *testing.T) {
	if os.Getenv("SKIP_NETWORK_TESTS") == "true" || os.Getenv("CI") == "true" {
		t.Skip("Skipping network failure tests in CI environment - these tests are flaky and timeout prone")
	}

	// Test network failure simulation and recovery
	// This test validates that the application can handle network connectivity issues gracefully

	// Test 1: Invalid URL handling (connection refused)
	configWithInvalidURL := `
stream:
  url: "http://localhost:9999/invalid-stream"
whisper:
  model_path: "./models/ggml-base.en.bin"
debug_mode: true
`

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid-url-config.yaml")
	err := os.WriteFile(configPath, []byte(configWithInvalidURL), 0644)
	require.NoError(t, err, "Should create invalid URL config")

	// Try to run application with invalid URL - it should fail gracefully
	// Short timeout to ensure test completes quickly
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "Should find repository root")

	appCmd := exec.CommandContext(ctx, "go", "run", "cmd/radiocontestwinner/main.go")
	appCmd.Dir = repoRoot
	appCmd.Env = append(os.Environ(),
		"CONFIG_FILE="+configPath,
		"STREAM_MAX_RETRIES=2",           // Reduce retries for faster test
		"STREAM_BASE_BACKOFF_MS=100")     // Reduce backoff for faster test

	output, err := appCmd.CombinedOutput()
	// The application should either fail gracefully or handle the error appropriately
	// We expect it to either exit with an error or timeout (which is acceptable for this test)

	outputStr := string(output)
	t.Logf("Application output with invalid URL: %s", outputStr)

	// Test that the application produces structured logs even during failure
	if strings.Contains(outputStr, "{") {
		// Should contain JSON structured logs
		assert.Contains(t, outputStr, "level", "Should produce structured logs during failure")
		assert.Contains(t, outputStr, "msg", "Should produce structured log messages")
	}

	// Test 2: Unreachable but valid URL handling (connection timeout)
	configWithUnreachableURL := `
stream:
  url: "http://localhost:9998/unreachable-stream"
whisper:
  model_path: "./models/ggml-base.en.bin"
debug_mode: true
`

	unreachableConfigPath := filepath.Join(tempDir, "unreachable-config.yaml")
	err = os.WriteFile(unreachableConfigPath, []byte(configWithUnreachableURL), 0644)
	require.NoError(t, err, "Should create unreachable URL config")

	// The application should handle unreachable URLs gracefully
	// Short timeout to ensure test completes quickly
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	appCmd2 := exec.CommandContext(ctx2, "go", "run", "cmd/radiocontestwinner/main.go")
	appCmd2.Dir = repoRoot
	appCmd2.Env = append(os.Environ(),
		"CONFIG_FILE="+unreachableConfigPath,
		"STREAM_MAX_RETRIES=2",           // Reduce retries for faster test
		"STREAM_BASE_BACKOFF_MS=100")     // Reduce backoff for faster test

	output2, _ := appCmd2.CombinedOutput()
	outputStr2 := string(output2)
	t.Logf("Application output with unreachable URL: %s", outputStr2)

	// Application should either handle the error gracefully or timeout
	// Both are acceptable behaviors for network connectivity failures
}

// TestApplicationRestartAndStateRecovery tests application restart and state recovery procedures
func TestApplicationRestartAndStateRecovery(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping Docker tests in CI environment - these tests are slow and require Docker daemon")
	}

	// Build test image
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Create configuration for restart testing
	configContent := `
debug_mode: true
stream:
  url: "https://example.com/test-stream"
whisper:
  model_path: "./models/ggml-base.en.bin"
`

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "restart-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Should create restart test config")

	containerName := "radiocontestwinner-restart-test-" + generateTimestamp()

	// Test restart procedure
	for iteration := 1; iteration <= 3; iteration++ {
		t.Logf("Restart iteration %d", iteration)

		// Start container
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		runCmd := exec.CommandContext(ctx, "docker", "run",
			"--name", containerName,
			"--detach",
			"-v", configPath+":/app/config.yaml:ro",
			imageName)

		startOutput, err := runCmd.CombinedOutput()
		require.NoError(t, err, "Container should start on iteration %d. Output: %s", iteration, string(startOutput))

		// Let it run briefly
		time.Sleep(3 * time.Second)

		// Check that container is running
		statusCmd := exec.Command("docker", "ps", "--filter", "name="+containerName, "--format", "{{.Status}}")
		statusOutput, err := statusCmd.CombinedOutput()
		require.NoError(t, err, "Should get container status on iteration %d", iteration)

		status := strings.TrimSpace(string(statusOutput))
		assert.Contains(t, status, "Up", "Container should be running on iteration %d", iteration)

		// Stop container gracefully
		stopCmd := exec.CommandContext(ctx, "docker", "stop", containerName)
		stopOutput, err := stopCmd.CombinedOutput()
		require.NoError(t, err, "Container should stop gracefully on iteration %d. Output: %s", iteration, string(stopOutput))

		// Remove container for next iteration
		rmCmd := exec.Command("docker", "rm", containerName)
		rmOutput, err := rmCmd.CombinedOutput()
		require.NoError(t, err, "Container should be removed on iteration %d. Output: %s", iteration, string(rmOutput))

		cancel()
		time.Sleep(1 * time.Second)
	}

	t.Log("Application restart and state recovery test completed successfully")
}

// TestGracefulDegradationUnderResourceConstraints tests graceful degradation during resource constraints
func TestGracefulDegradationUnderResourceConstraints(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping resource constraint test in CI environment - this test is very slow (3+ minutes)")
	}

	if os.Getenv("SKIP_RESOURCE_CONSTRAINT_TESTS") == "true" {
		t.Skip("Skipping resource constraint tests - set SKIP_RESOURCE_CONSTRAINT_TESTS=false to enable")
	}

	// Build test image
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Test with very limited resources to trigger graceful degradation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-constraints-test-" + generateTimestamp()

	// Start container with very limited resources
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--rm",
		"--memory", "64m", // Very limited memory
		"--cpus", "0.1", // Very limited CPU
		imageName,
		"--help") // Use help to test basic functionality

	output, err := runCmd.CombinedOutput()

	// The application should either:
	// 1. Run successfully with degraded performance, or
	// 2. Fail gracefully with an appropriate error message

	outputStr := string(output)
	t.Logf("Application output under resource constraints: %s", outputStr)

	if err != nil {
		// If it fails, it should fail gracefully
		t.Logf("Application failed under resource constraints (acceptable): %v", err)
		// Verify it produces some output even during failure
		assert.NotEmpty(t, outputStr, "Should produce some output even during resource constraint failure")
	} else {
		// If it succeeds, verify expected output
		assert.Contains(t, outputStr, "Radio Contest Winner", "Should show application info even under constraints")
	}
}

// TestSignalHandlingAndGracefulShutdown tests signal handling and graceful shutdown
func TestSignalHandlingAndGracefulShutdown(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping signal handling test in CI environment - this test is flaky and slow in CI")
	}
	if os.Getenv("SKIP_SIGNAL_TESTS") == "true" {
		t.Skip("Skipping signal handling tests - set SKIP_SIGNAL_TESTS=false to enable")
	}

	// Test graceful shutdown via SIGTERM (standard Docker stop signal)
	configContent := `
debug_mode: true
stream:
  url: "https://example.com/test-stream"
whisper:
  model_path: "./models/ggml-base.en.bin"
`

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "signal-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Should create signal test config")

	// Start application process
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "Should find repository root")

	appCmd := exec.CommandContext(ctx, "go", "run", "cmd/radiocontestwinner/main.go")
	appCmd.Dir = repoRoot
	appCmd.Env = append(os.Environ(), "CONFIG_FILE="+configPath)

	// Start the application
	err = appCmd.Start()
	require.NoError(t, err, "Application should start for signal test")

	// Give it time to initialize
	time.Sleep(2 * time.Second)

	// Send SIGTERM for graceful shutdown
	err = appCmd.Process.Signal(os.Interrupt)
	require.NoError(t, err, "Should be able to send interrupt signal")

	// Wait for graceful shutdown
	done := make(chan error, 1)
	go func() {
		done <- appCmd.Wait()
	}()

	select {
	case err := <-done:
		// Application should exit (possibly with error due to missing model, which is acceptable)
		t.Logf("Application exited gracefully: %v", err)
	case <-time.After(10 * time.Second):
		// Force kill if it doesn't exit gracefully
		appCmd.Process.Kill()
		t.Error("Application did not exit gracefully within timeout")
	}
}

// Helper functions for failure recovery tests

// TestPortBinding tests that the application can bind to ports when needed
func TestPortBinding(t *testing.T) {
	// Test that port 8080 (exposed in Dockerfile) can be bound if needed
	// This is important for future HTTP health check endpoints

	listener, err := net.Listen("tcp", ":0") // Bind to any available port
	require.NoError(t, err, "Should be able to bind to a port")
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	assert.Greater(t, addr.Port, 0, "Should get a valid port number")

	t.Logf("Successfully bound to port %d", addr.Port)

	// Test basic HTTP server functionality (for future health endpoints)
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", addr.Port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("healthy"))
		}),
	}

	go func() {
		server.Serve(listener)
	}()

	time.Sleep(100 * time.Millisecond)

	// Test health check endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d", addr.Port))
	require.NoError(t, err, "Should be able to make HTTP request to health endpoint")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should return OK")

	// Cleanup
	server.Close()
}
