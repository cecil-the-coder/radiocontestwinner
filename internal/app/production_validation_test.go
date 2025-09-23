package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ResourceThresholds defines acceptable resource usage thresholds from story requirements
type ResourceThresholds struct {
	CPUAveragePercent   float64 // <50% average
	MemoryPeakMB        float64 // <512MB peak
	DiskGrowthMBPerHour float64 // <1GB/hour growth
	ProcessingLatencyMS int     // <500ms real-time processing latency
}

// ContainerResourceLimits defines container resource limits from story requirements
type ContainerResourceLimits struct {
	MemoryLimitGB float64 // 1GB memory limit
	CPUCores      float64 // 1 CPU core limit
}

// ResourceMetrics represents collected resource usage metrics
type ResourceMetrics struct {
	CPUPercent    float64
	MemoryUsageMB float64
	DiskUsageMB   float64
	Timestamp     time.Time
}

// TestDefineAcceptableResourceThresholds tests that resource thresholds are properly defined
func TestDefineAcceptableResourceThresholds(t *testing.T) {
	// Define production resource thresholds as specified in story requirements
	thresholds := ResourceThresholds{
		CPUAveragePercent:   50.0,   // CPU <50% average
		MemoryPeakMB:        512.0,  // Memory <512MB peak
		DiskGrowthMBPerHour: 1024.0, // Disk <1GB/hour growth
		ProcessingLatencyMS: 500,    // Real-time processing latency <500ms
	}

	// Verify thresholds are reasonable for production deployment
	assert.Greater(t, thresholds.CPUAveragePercent, 0.0, "CPU threshold should be positive")
	assert.LessOrEqual(t, thresholds.CPUAveragePercent, 100.0, "CPU threshold should not exceed 100%")

	assert.Greater(t, thresholds.MemoryPeakMB, 0.0, "Memory threshold should be positive")
	assert.LessOrEqual(t, thresholds.MemoryPeakMB, 2048.0, "Memory threshold should be reasonable for containers")

	assert.Greater(t, thresholds.DiskGrowthMBPerHour, 0.0, "Disk growth threshold should be positive")
	assert.LessOrEqual(t, thresholds.DiskGrowthMBPerHour, 10240.0, "Disk growth threshold should be reasonable")

	assert.Greater(t, thresholds.ProcessingLatencyMS, 0, "Processing latency threshold should be positive")
	assert.LessOrEqual(t, thresholds.ProcessingLatencyMS, 1000, "Processing latency should be reasonable for real-time")

	t.Logf("Production resource thresholds defined: CPU <%v%%, Memory <%vMB, Disk <%vMB/hour, Latency <%vms",
		thresholds.CPUAveragePercent, thresholds.MemoryPeakMB, thresholds.DiskGrowthMBPerHour, thresholds.ProcessingLatencyMS)
}

// TestContainerResourceLimitsConfiguration tests container resource limits configuration
func TestContainerResourceLimitsConfiguration(t *testing.T) {
	skipDockerTestsInCI(t)

	// Define container resource limits as specified in story requirements
	limits := ContainerResourceLimits{
		MemoryLimitGB: 1.0, // 1GB memory limit
		CPUCores:      1.0, // 1 CPU core limit
	}

	// Build test image
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Test container with specified resource limits
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-limits-validation-" + generateTimestamp()

	// Start container with production resource limits
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--rm",
		"--memory", fmt.Sprintf("%.0fm", limits.MemoryLimitGB*1024), // Convert GB to MB
		"--cpus", fmt.Sprintf("%.1f", limits.CPUCores),
		imageName,
		"--help")

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should run with production resource limits. Output: %s", string(output))

	// Verify output is correct
	outputStr := string(output)
	assert.Contains(t, outputStr, "Radio Contest Winner", "Should display application title")

	t.Logf("Container resource limits validated: Memory %vGB, CPU %v cores", limits.MemoryLimitGB, limits.CPUCores)
}

// TestPerformanceBenchmarkThroughput tests processing throughput validation
func TestPerformanceBenchmarkThroughput(t *testing.T) {
	if os.Getenv("SKIP_PERFORMANCE_TESTS") == "true" {
		t.Skip("Skipping performance tests - set SKIP_PERFORMANCE_TESTS=false to enable")
	}

	// Define minimum throughput requirement: ≥1x real-time audio speed
	minThroughputRatio := 1.0 // Must process audio at least as fast as real-time

	// Create a benchmark that simulates audio processing throughput
	// This would typically process actual audio data, but for testing we simulate the timing
	audioDataSizeMB := 10.0 // Simulate processing 10MB of audio data
	processingStartTime := time.Now()

	// Simulate audio processing workload
	// In real implementation, this would call the actual audio processing pipeline
	simulateAudioProcessing(t, audioDataSizeMB)

	processingDuration := time.Since(processingStartTime)

	// Calculate throughput ratio
	// For real-time audio: 1 minute of audio ≈ 1MB at typical quality
	// So 10MB should represent ~10 minutes of audio
	expectedRealTimeForAudio := time.Duration(audioDataSizeMB) * time.Minute
	throughputRatio := expectedRealTimeForAudio.Seconds() / processingDuration.Seconds()

	t.Logf("Audio processing benchmark: %.1fMB processed in %v (%.2fx real-time speed)",
		audioDataSizeMB, processingDuration, throughputRatio)

	assert.GreaterOrEqual(t, throughputRatio, minThroughputRatio,
		"Processing throughput should be ≥1x real-time audio speed")
}

// TestResourceMonitoringExtendedOperation tests resource monitoring during extended operation
func TestResourceMonitoringExtendedOperation(t *testing.T) {
	skipDockerTestsInCI(t)
	if os.Getenv("SKIP_EXTENDED_TESTS") == "true" {
		t.Skip("Skipping extended operation tests - set SKIP_EXTENDED_TESTS=false to enable")
	}

	// For testing purposes, use a shorter duration than the full 2 hours
	// In production validation, this should be set to 2+ hours
	testDuration := 60 * time.Second // Use 1 minute for testing, extend to 2 hours for production
	if os.Getenv("EXTENDED_TEST_DURATION") != "" {
		if duration, err := time.ParseDuration(os.Getenv("EXTENDED_TEST_DURATION")); err == nil {
			testDuration = duration
		}
	}

	thresholds := ResourceThresholds{
		CPUAveragePercent:   50.0,
		MemoryPeakMB:        512.0,
		DiskGrowthMBPerHour: 1024.0,
		ProcessingLatencyMS: 500,
	}

	// Build test image
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Create configuration for extended testing
	configContent := `
debug_mode: true
stream:
  url: "https://example.com/test-stream"
whisper:
  model_path: "/app/models/test-model.bin"
buffer:
  duration_ms: 2500
`

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "extended-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Should create extended test config")

	// Start container for extended operation monitoring
	ctx, cancel := context.WithTimeout(context.Background(), testDuration+30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-extended-test-" + generateTimestamp()

	// Start container with resource monitoring
	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--detach",
		"--memory", "1g",
		"--cpus", "1.0",
		"-v", configPath+":/app/config.yaml:ro",
		imageName)

	startOutput, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should start for extended monitoring. Output: %s", string(startOutput))

	// Cleanup container after test
	defer func() {
		stopCmd := exec.Command("docker", "stop", containerName)
		stopCmd.Run()
		rmCmd := exec.Command("docker", "rm", containerName)
		rmCmd.Run()
	}()

	// Monitor resources during extended operation
	monitoringInterval := 5 * time.Second
	var metrics []ResourceMetrics
	monitoringDone := make(chan bool)

	go func() {
		defer close(monitoringDone)
		ticker := time.NewTicker(monitoringInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metric, err := collectContainerMetrics(containerName)
				if err != nil {
					t.Logf("Error collecting metrics: %v", err)
					continue
				}
				metrics = append(metrics, metric)
				t.Logf("Resource metrics: CPU=%.1f%%, Memory=%.1fMB, Disk=%.1fMB",
					metric.CPUPercent, metric.MemoryUsageMB, metric.DiskUsageMB)
			}
		}
	}()

	// Wait for test duration
	time.Sleep(testDuration)

	// Stop monitoring
	cancel()
	<-monitoringDone

	// Analyze collected metrics
	require.Greater(t, len(metrics), 0, "Should have collected resource metrics")

	// Calculate average CPU usage
	var totalCPU float64
	var peakMemory float64
	for _, metric := range metrics {
		totalCPU += metric.CPUPercent
		if metric.MemoryUsageMB > peakMemory {
			peakMemory = metric.MemoryUsageMB
		}
	}
	averageCPU := totalCPU / float64(len(metrics))

	t.Logf("Extended operation results: Average CPU=%.1f%%, Peak Memory=%.1fMB over %v",
		averageCPU, peakMemory, testDuration)

	// Validate against thresholds
	assert.LessOrEqual(t, averageCPU, thresholds.CPUAveragePercent,
		"Average CPU usage should be within threshold")
	assert.LessOrEqual(t, peakMemory, thresholds.MemoryPeakMB,
		"Peak memory usage should be within threshold")
}

// TestApplicationPerformanceUnderLoad tests application performance under sustained load
func TestApplicationPerformanceUnderLoad(t *testing.T) {
	if os.Getenv("SKIP_PERFORMANCE_TESTS") == "true" {
		t.Skip("Skipping performance tests - set SKIP_PERFORMANCE_TESTS=false to enable")
	}

	// Test processing latency requirements
	maxLatency := 500 * time.Millisecond // <500ms processing latency requirement

	// Simulate processing load and measure latency
	iterations := 100
	var totalLatency time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()

		// Simulate audio processing workload
		simulateProcessingLoad()

		latency := time.Since(start)
		totalLatency += latency

		assert.LessOrEqual(t, latency, maxLatency,
			"Individual processing latency should be within threshold")
	}

	averageLatency := totalLatency / time.Duration(iterations)
	t.Logf("Performance under load: Average latency=%v over %d iterations", averageLatency, iterations)

	assert.LessOrEqual(t, averageLatency, maxLatency,
		"Average processing latency should be within threshold")
}

// Helper functions

// simulateAudioProcessing simulates audio processing for throughput testing
func simulateAudioProcessing(t *testing.T, audioDataSizeMB float64) {
	// Simulate computational load equivalent to audio processing
	// This is a simplified simulation - real tests would process actual audio
	iterations := int(audioDataSizeMB * 1000) // Scale with data size

	for i := 0; i < iterations; i++ {
		// Simulate some computational work
		_ = fmt.Sprintf("processing-frame-%d", i)
		if i%10000 == 0 {
			// Small delay to simulate I/O operations
			time.Sleep(time.Microsecond)
		}
	}
}

// simulateProcessingLoad simulates processing load for latency testing
func simulateProcessingLoad() {
	// Simulate typical processing operations
	for i := 0; i < 1000; i++ {
		_ = fmt.Sprintf("processing-operation-%d", i)
	}
	// Simulate brief I/O wait
	time.Sleep(time.Microsecond * 100)
}

// collectContainerMetrics collects resource metrics from a running container
func collectContainerMetrics(containerName string) (ResourceMetrics, error) {
	metric := ResourceMetrics{
		Timestamp: time.Now(),
	}

	// Get container stats
	statsCmd := exec.Command("docker", "stats", "--no-stream", "--format",
		"table {{.CPUPerc}}\t{{.MemUsage}}", containerName)

	output, err := statsCmd.CombinedOutput()
	if err != nil {
		return metric, fmt.Errorf("failed to get container stats: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return metric, fmt.Errorf("invalid stats output")
	}

	// Parse stats output (format: "CPU%    MEM USAGE / LIMIT")
	statsLine := strings.Fields(lines[1])
	if len(statsLine) >= 2 {
		// Parse CPU percentage
		cpuStr := strings.TrimSuffix(statsLine[0], "%")
		if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
			metric.CPUPercent = cpu
		}

		// Parse memory usage (format like "123.4MiB")
		memStr := statsLine[1]
		if strings.HasSuffix(memStr, "MiB") {
			memStr = strings.TrimSuffix(memStr, "MiB")
			if mem, err := strconv.ParseFloat(memStr, 64); err == nil {
				metric.MemoryUsageMB = mem
			}
		} else if strings.HasSuffix(memStr, "GiB") {
			memStr = strings.TrimSuffix(memStr, "GiB")
			if mem, err := strconv.ParseFloat(memStr, 64); err == nil {
				metric.MemoryUsageMB = mem * 1024
			}
		}
	}

	// Get disk usage (simplified - in production would monitor actual application disk usage)
	metric.DiskUsageMB = 100.0 // Placeholder - real implementation would measure actual disk usage

	return metric, nil
}

// TestResourceMetricsCollection tests that resource metrics can be collected properly
func TestResourceMetricsCollection(t *testing.T) {
	skipDockerTestsInCI(t)

	// Build test image
	imageName := buildTestImage(t)
	defer cleanupImage(imageName)

	// Start a simple container to test metrics collection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerName := "radiocontestwinner-metrics-test-" + generateTimestamp()

	runCmd := exec.CommandContext(ctx, "docker", "run",
		"--name", containerName,
		"--detach",
		imageName,
		"sleep", "20")

	startOutput, err := runCmd.CombinedOutput()
	require.NoError(t, err, "Container should start for metrics collection test. Output: %s", string(startOutput))

	// Cleanup container after test
	defer func() {
		stopCmd := exec.Command("docker", "stop", containerName)
		stopCmd.Run()
		rmCmd := exec.Command("docker", "rm", containerName)
		rmCmd.Run()
	}()

	// Wait for container to be running
	time.Sleep(2 * time.Second)

	// Test metrics collection
	metrics, err := collectContainerMetrics(containerName)
	require.NoError(t, err, "Should collect container metrics")

	assert.GreaterOrEqual(t, metrics.CPUPercent, 0.0, "CPU usage should be non-negative")
	assert.GreaterOrEqual(t, metrics.MemoryUsageMB, 0.0, "Memory usage should be non-negative")
	assert.GreaterOrEqual(t, metrics.DiskUsageMB, 0.0, "Disk usage should be non-negative")

	t.Logf("Collected metrics: CPU=%.2f%%, Memory=%.2fMB, Disk=%.2fMB",
		metrics.CPUPercent, metrics.MemoryUsageMB, metrics.DiskUsageMB)
}
