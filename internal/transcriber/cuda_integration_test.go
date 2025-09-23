package transcriber

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"radiocontestwinner/internal/config"
	"radiocontestwinner/internal/gpu"
	"radiocontestwinner/internal/performance"
)

// skipCUDATestsInCI skips CUDA integration tests in CI environment
func skipCUDATestsInCI(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping GPU/CUDA integration test in CI environment - no CUDA hardware available")
	}
}

func TestWhisperCppModel_GPUConfiguration(t *testing.T) {
	skipCUDATestsInCI(t)
	t.Run("should initialize with GPU auto-detection enabled", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(true)
		config.SetCUBLASAutoDetect(true)

		model := NewWhisperCppModelWithConfig(logger, config)

		// Assert
		assert.NotNil(t, model)
		assert.Equal(t, true, config.GetCUBLASEnabled())
		assert.Equal(t, true, config.GetCUBLASAutoDetect())
	})

	t.Run("should initialize with explicit GPU configuration", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(true)
		config.SetCUBLASAutoDetect(false)
		config.SetGPUDeviceID(0)

		model := NewWhisperCppModelWithConfig(logger, config)

		// Assert
		assert.NotNil(t, model)
		assert.Equal(t, true, config.GetCUBLASEnabled())
		assert.Equal(t, false, config.GetCUBLASAutoDetect())
		assert.Equal(t, 0, config.GetGPUDeviceID())
	})

	t.Run("should initialize with CPU-only configuration", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(false)

		model := NewWhisperCppModelWithConfig(logger, config)

		// Assert
		assert.NotNil(t, model)
		assert.Equal(t, false, config.GetCUBLASEnabled())
	})
}

func TestWhisperCppModel_GPUStatusReporting(t *testing.T) {
	skipCUDATestsInCI(t)
	t.Run("should report GPU status when GPU is enabled", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(true)
		config.SetCUBLASAutoDetect(false)
		config.SetGPUDeviceID(1)

		model := NewWhisperCppModelWithConfig(logger, config)

		// Act
		useGPU, deviceID := model.GetGPUStatus()

		// Assert
		// When explicitly configured, GPU status reflects configuration even if no physical GPU
		assert.Equal(t, true, useGPU) // Configuration says to use GPU
		assert.Equal(t, 1, deviceID)  // Should return configured device ID
	})

	t.Run("should report CPU status when GPU is disabled", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(false)

		model := NewWhisperCppModelWithConfig(logger, config)

		// Act
		useGPU, deviceID := model.GetGPUStatus()

		// Assert
		assert.Equal(t, false, useGPU)
		// When GPU is disabled, device ID defaults to 0 (not -1)
		assert.Equal(t, 0, deviceID)
	})
}

func TestTranscriptionEngine_GPUPerformanceMonitoring(t *testing.T) {
	skipCUDATestsInCI(t)
	t.Run("should track GPU vs CPU performance metrics", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(false)       // Force CPU for predictable test
		config.SetTranscriptionTimeoutSec(1) // Short timeout for testing

		engine := NewTranscriptionEngineWithConfig(logger, config)
		mockModel := &MockWhisperModel{
			segments: []TranscriptionSegment{
				{
					Text:       "Test segment",
					StartMS:    0,
					EndMS:      1000,
					Confidence: 0.95,
				},
			},
		}
		engine.model = mockModel

		// Act - Process audio to generate performance metrics
		audioReader := strings.NewReader("mock audio data")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		segmentChan, err := engine.ProcessAudio(ctx, audioReader)
		assert.NoError(t, err)

		// Collect segments with timeout
		segments := []TranscriptionSegment{}
		for segment := range segmentChan {
			segments = append(segments, segment)
		}

		// Assert - Check performance metrics
		metrics := engine.GetPerformanceMetrics()
		assert.NotNil(t, metrics)
		assert.Equal(t, int64(1), metrics.TotalTranscriptions)
		assert.Equal(t, int64(1), metrics.CPUTranscriptions)
		assert.Equal(t, int64(0), metrics.GPUTranscriptions)
		assert.GreaterOrEqual(t, metrics.TotalAudioBytes, int64(0))
		assert.GreaterOrEqual(t, metrics.TotalProcessingTime.Milliseconds(), int64(0))
	})

	t.Run("should provide GPU vs CPU performance comparison", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()

		engine := NewTranscriptionEngineWithConfig(logger, config)

		// Act - Get performance comparison before any processing
		comparison := engine.CompareGPUvsCPU()

		// Assert
		assert.Contains(t, comparison, "Insufficient data for GPU vs CPU comparison")
	})
}

func TestTranscriptionEngine_GPUConfigurationIntegration(t *testing.T) {
	skipCUDATestsInCI(t)
	t.Run("should use GPU-aware configuration when available", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(true)
		config.SetCUBLASAutoDetect(true)
		config.SetWhisperThreads(4) // Configure threads for GPU processing

		engine := NewTranscriptionEngineWithConfig(logger, config)

		// Assert
		assert.NotNil(t, engine)
		assert.Equal(t, 4, config.GetWhisperThreads())

		// Verify model has GPU configuration
		useGPU, deviceID := engine.model.GetGPUStatus()
		// In test environment with auto-detection, GPU should be disabled when not available
		assert.False(t, useGPU)      // Auto-detection should disable GPU when not available
		assert.Equal(t, 0, deviceID) // Default device ID
	})

	t.Run("should fall back to CPU when GPU is not available", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(true)
		config.SetCUBLASAutoDetect(true)
		config.SetTranscriptionTimeoutSec(1) // Short timeout for testing

		engine := NewTranscriptionEngineWithConfig(logger, config)

		// Act - Process audio to test fallback behavior
		mockModel := &MockWhisperModel{
			segments: []TranscriptionSegment{
				{
					Text:       "Fallback test",
					StartMS:    0,
					EndMS:      1000,
					Confidence: 0.95,
				},
			},
		}
		engine.model = mockModel

		audioReader := strings.NewReader("mock audio data")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		segmentChan, err := engine.ProcessAudio(ctx, audioReader)
		assert.NoError(t, err)

		// Collect segments
		segments := []TranscriptionSegment{}
		for segment := range segmentChan {
			segments = append(segments, segment)
		}

		// Assert - Should still work with CPU fallback
		assert.Len(t, segments, 1)
		assert.Equal(t, "Fallback test", segments[0].Text)

		// Verify performance metrics show CPU usage
		metrics := engine.GetPerformanceMetrics()
		assert.Equal(t, int64(1), metrics.CPUTranscriptions)
		assert.Equal(t, int64(0), metrics.GPUTranscriptions)
	})
}

func TestGPUDetector_IntegrationWithTranscriber(t *testing.T) {
	skipCUDATestsInCI(t)
	t.Run("should integrate GPU detector with Whisper model", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		gpuDetector := gpu.NewGPUDetector(logger)

		// Act - Get GPU info
		gpuInfo := gpuDetector.GetGPUInfo()

		// Assert
		assert.NotNil(t, gpuInfo)
		assert.NotNil(t, gpuInfo.Available)
		assert.NotNil(t, gpuInfo.DeviceCount)
		assert.NotNil(t, gpuInfo.DeviceName)
		assert.NotNil(t, gpuInfo.CUDAVersion)
		assert.NotNil(t, gpuInfo.DriverVersion)

		// In test environment, GPU should not be available
		assert.False(t, gpuInfo.Available)
		assert.Equal(t, 0, gpuInfo.DeviceCount)
		assert.Equal(t, "", gpuInfo.DeviceName)
		assert.Equal(t, "", gpuInfo.CUDAVersion)
		assert.Equal(t, "", gpuInfo.DriverVersion)
	})

	t.Run("should provide optimal device ID for multi-GPU systems", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		gpuDetector := gpu.NewGPUDetector(logger)

		// Act - Get optimal device ID for different scenarios
		optimalID := gpuDetector.GetOptimalDeviceID(2) // Request device 2

		// Assert - Should return -1 when no GPU available
		assert.Equal(t, -1, optimalID)

		// Test with invalid device ID
		invalidID := gpuDetector.GetOptimalDeviceID(-1)
		assert.Equal(t, -1, invalidID)
	})
}

func TestPerformanceMonitoring_GPUvsCPU(t *testing.T) {
	skipCUDATestsInCI(t)
	t.Run("should track GPU and CPU transcription performance separately", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		performanceMonitor := performance.NewPerformanceMonitor(logger)

		// Act - Simulate GPU transcription
		gpuTimer := performanceMonitor.StartTranscription(1000, true, 0)
		time.Sleep(10 * time.Millisecond) // Simulate processing time
		performanceMonitor.EndTranscription(gpuTimer)

		// Simulate CPU transcription
		cpuTimer := performanceMonitor.StartTranscription(1000, false, -1)
		time.Sleep(15 * time.Millisecond) // Simulate processing time
		performanceMonitor.EndTranscription(cpuTimer)

		// Get metrics
		metrics := performanceMonitor.GetMetrics()

		// Assert
		assert.Equal(t, int64(2), metrics.TotalTranscriptions)
		assert.Equal(t, int64(1), metrics.GPUTranscriptions)
		assert.Equal(t, int64(1), metrics.CPUTranscriptions)
		assert.Greater(t, metrics.TotalProcessingTime.Milliseconds(), int64(0))
		assert.Greater(t, metrics.AvgTranscriptionTime.Milliseconds(), int64(0))
	})

	t.Run("should calculate performance improvement metrics", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		performanceMonitor := performance.NewPerformanceMonitor(logger)

		// Act - Add some performance data
		// Simulate faster GPU processing
		gpuTimer := performanceMonitor.StartTranscription(1000, true, 0)
		time.Sleep(5 * time.Millisecond)
		performanceMonitor.EndTranscription(gpuTimer)

		// Simulate slower CPU processing
		cpuTimer := performanceMonitor.StartTranscription(1000, false, -1)
		time.Sleep(15 * time.Millisecond)
		performanceMonitor.EndTranscription(cpuTimer)

		// Get comparison
		comparison := performanceMonitor.CompareGPUvsCPU()

		// Assert
		assert.Contains(t, comparison, "GPU vs CPU Performance Comparison")
		assert.Contains(t, comparison, "GPU Transcriptions")
		assert.Contains(t, comparison, "CPU Transcriptions")
		assert.Contains(t, comparison, "Estimated Improvement")
	})
}

func TestCUDAIntegration_ErrorHandling(t *testing.T) {
	skipCUDATestsInCI(t)
	t.Run("should handle GPU initialization errors gracefully", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(true)
		config.SetCUBLASAutoDetect(false)
		config.SetGPUDeviceID(999) // Invalid device ID

		model := NewWhisperCppModelWithConfig(logger, config)

		// Act & Assert - Should not panic and should initialize successfully
		assert.NotNil(t, model)

		// Should report GPU status based on configuration even if GPU not available
		useGPU, deviceID := model.GetGPUStatus()
		// When explicitly configured, useGPU should remain true even if GPU not detected
		// The actual availability check happens at runtime
		assert.True(t, useGPU)         // Configuration says to use GPU
		assert.Equal(t, 999, deviceID) // Preserves configured device ID
	})

	t.Run("should handle transcription errors with GPU configuration", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		config.SetCUBLASEnabled(true)
		config.SetTranscriptionTimeoutSec(1) // Short timeout for testing

		engine := NewTranscriptionEngineWithConfig(logger, config)
		mockModel := &MockWhisperModel{
			transcribeError: assert.AnError,
		}
		engine.model = mockModel

		// Act
		audioReader := strings.NewReader("mock audio data")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		segmentChan, err := engine.ProcessAudio(ctx, audioReader)

		// Assert
		assert.NoError(t, err) // ProcessAudio should not fail immediately
		assert.NotNil(t, segmentChan)

		// Channel should be closed without any segments due to error
		segments := []TranscriptionSegment{}
		for segment := range segmentChan {
			segments = append(segments, segment)
		}
		assert.Empty(t, segments)
	})
}
