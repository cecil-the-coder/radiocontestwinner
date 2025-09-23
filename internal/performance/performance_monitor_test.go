package performance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestPerformanceMonitorCreation(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.logger)
	assert.False(t, monitor.benchmark)
}

func TestPerformanceMonitorWithBenchmark(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitorWithBenchmark(logger, true)

	assert.NotNil(t, monitor)
	assert.True(t, monitor.benchmark)
}

func TestStartTranscription(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	timer := monitor.StartTranscription(1024, true, 0)
	assert.NotNil(t, timer)
	assert.True(t, timer.UseGPU)
	assert.Equal(t, int64(1024), timer.AudioBytes)
	assert.Equal(t, 0, timer.DeviceID)
	assert.False(t, timer.StartTime.IsZero())
}

func TestEndTranscriptionUpdatesMetrics(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	// Start and end a transcription
	timer := monitor.StartTranscription(2048, false, 0)
	time.Sleep(10 * time.Millisecond) // Small delay to measure
	monitor.EndTranscription(timer)

	// Check metrics were updated
	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalTranscriptions)
	assert.Equal(t, int64(2048), metrics.TotalAudioBytes)
	assert.Equal(t, int64(1), metrics.CPUTranscriptions)
	assert.Equal(t, int64(0), metrics.GPUTranscriptions)
	assert.True(t, metrics.TotalProcessingTime > 0)
	assert.Equal(t, false, metrics.LastGPUUsed)
}

func TestEndTranscriptionWithGPU(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	// Start and end a GPU transcription
	timer := monitor.StartTranscription(4096, true, 1)
	time.Sleep(10 * time.Millisecond)
	monitor.EndTranscription(timer)

	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalTranscriptions)
	assert.Equal(t, int64(1), metrics.GPUTranscriptions)
	assert.Equal(t, int64(0), metrics.CPUTranscriptions)
	assert.True(t, metrics.LastGPUUsed)
	assert.Equal(t, 1, metrics.LastDeviceID)
}

func TestMultipleTranscriptions(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	// Simulate multiple transcriptions
	for i := 0; i < 5; i++ {
		useGPU := i%2 == 0 // Alternate GPU/CPU
		timer := monitor.StartTranscription(int64(1024*(i+1)), useGPU, i%2)
		time.Sleep(time.Duration(i+1) * time.Millisecond)
		monitor.EndTranscription(timer)
	}

	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(5), metrics.TotalTranscriptions)
	assert.Equal(t, int64(3), metrics.GPUTranscriptions) // 0, 2, 4
	assert.Equal(t, int64(2), metrics.CPUTranscriptions) // 1, 3
	assert.True(t, metrics.AvgTranscriptionTime > 0)
	assert.True(t, metrics.MaxTranscriptionTime >= metrics.MinTranscriptionTime)
}

func TestGetPerformanceSummary(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	// Empty metrics
	summary := monitor.GetPerformanceSummary()
	assert.Contains(t, summary, "No transcription metrics available")

	// Add some transcriptions
	timer := monitor.StartTranscription(1024, true, 0)
	time.Sleep(1 * time.Millisecond)
	monitor.EndTranscription(timer)

	summary = monitor.GetPerformanceSummary()
	assert.Contains(t, summary, "Performance Summary")
	assert.Contains(t, summary, "Total Transcriptions: 1")
	assert.Contains(t, summary, "GPU Usage:")
}

func TestCompareGPUvsCPU(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	// Empty comparison
	comparison := monitor.CompareGPUvsCPU()
	assert.Contains(t, comparison, "Insufficient data")

	// Add both GPU and CPU transcriptions
	timer1 := monitor.StartTranscription(1024, true, 0)
	time.Sleep(1 * time.Millisecond)
	monitor.EndTranscription(timer1)

	timer2 := monitor.StartTranscription(1024, false, 0)
	time.Sleep(1 * time.Millisecond)
	monitor.EndTranscription(timer2)

	comparison = monitor.CompareGPUvsCPU()
	assert.Contains(t, comparison, "GPU vs CPU Performance Comparison")
	assert.Contains(t, comparison, "GPU Transcriptions: 1")
	assert.Contains(t, comparison, "CPU Transcriptions: 1")
}

func TestResetMetrics(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	// Add some transcriptions
	timer := monitor.StartTranscription(1024, true, 0)
	time.Sleep(1 * time.Millisecond)
	monitor.EndTranscription(timer)

	// Verify metrics exist
	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalTranscriptions)

	// Reset metrics
	monitor.ResetMetrics()

	// Verify metrics are reset
	metrics = monitor.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalTranscriptions)
	assert.Equal(t, int64(0), metrics.TotalAudioBytes)
	assert.Equal(t, time.Hour, metrics.MinTranscriptionTime)
}

func TestBenchmarkMode(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	assert.False(t, monitor.benchmark)

	monitor.BenchmarkMode(true)
	assert.True(t, monitor.benchmark)

	monitor.BenchmarkMode(false)
	assert.False(t, monitor.benchmark)
}

func TestLogCurrentMetrics(t *testing.T) {
	logger := zap.NewNop()
	monitor := NewPerformanceMonitor(logger)

	// This should not panic
	monitor.LogCurrentMetrics()

	// Add some metrics and log again
	timer := monitor.StartTranscription(1024, true, 0)
	time.Sleep(1 * time.Millisecond)
	monitor.EndTranscription(timer)

	// This should also not panic
	monitor.LogCurrentMetrics()
}
