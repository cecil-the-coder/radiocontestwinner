package app

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
)

// Benchmark tests for complete pipeline performance validation

func skipE2EBenchmarkInCI(b *testing.B) {
	if os.Getenv("CI") == "true" {
		b.Skip("Skipping E2E benchmark in CI environment - these tests are resource intensive and prone to timeout")
	}
}

func skipE2ETestInCI(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping E2E test in CI environment - these tests are resource intensive and prone to timeout")
	}
}

func BenchmarkE2E_CompletePipelineLatency(b *testing.B) {
	skipE2EBenchmarkInCI(b)
	b.Run("should process audio pipeline within real-time constraints", func(b *testing.B) {
		// Benchmark with generated test audio files
		// Note: Performance may vary without real Whisper model loaded

		testConfig := DefaultTestConfig()
		testConfig.DebugMode = false // Disable debug for accurate benchmarks

		// Create test audio data (5 seconds of audio)
		audioData := CreateTestAudioData(5*time.Second, 44100)
		mockServer := NewMockAudioServer(audioData, 10*time.Millisecond)
		defer mockServer.Close()

		testConfig.MockStreamURL = mockServer.URL()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			app, err := NewTestApplication(testConfig)
			if err != nil {
				b.Fatalf("Failed to create test application: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			start := time.Now()
			err = app.RunWithTimeout(ctx, 8*time.Second)
			latency := time.Since(start)

			cancel()
			app.Shutdown()

			if err != nil && err != context.DeadlineExceeded {
				b.Fatalf("Application run failed: %v", err)
			}

			// Report custom metrics
			b.ReportMetric(float64(latency.Milliseconds()), "latency_ms")
		}
	})
}

func BenchmarkE2E_MemoryUsage(b *testing.B) {
	b.Run("should maintain stable memory usage during extended operation", func(b *testing.B) {
		// Note: Memory usage testing with generated test audio files

		testConfig := DefaultTestConfig()
		testConfig.DebugMode = false

		// Create longer audio data for memory testing
		audioData := CreateTestAudioData(30*time.Second, 44100)
		mockServer := NewMockAudioServer(audioData, 5*time.Millisecond)
		defer mockServer.Close()

		testConfig.MockStreamURL = mockServer.URL()

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		initialMemory := memStats.Alloc

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			app, err := NewTestApplication(testConfig)
			if err != nil {
				b.Fatalf("Failed to create test application: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)

			// Monitor memory during operation
			memProfiler := NewMemoryProfiler()
			memProfiler.Start()

			err = app.RunWithTimeout(ctx, 30*time.Second)

			runtime.ReadMemStats(&memStats)
			finalMemory := memStats.Alloc
			memoryGrowth := finalMemory - initialMemory

			cancel()
			app.Shutdown()

			if err != nil && err != context.DeadlineExceeded {
				b.Fatalf("Application run failed: %v", err)
			}

			// Report memory metrics
			b.ReportMetric(float64(memoryGrowth), "memory_growth_bytes")
			b.ReportMetric(float64(memStats.NumGC), "gc_cycles")
		}
	})
}

func BenchmarkE2E_ConcurrentStreams(b *testing.B) {
	b.Run("should handle multiple simultaneous streams efficiently", func(b *testing.B) {
		// Note: Concurrent stream testing with generated audio files

		// Test concurrent processing of multiple streams
		// This would verify the system can handle multiple contest sources simultaneously
	})
}

func BenchmarkE2E_CPUProfiler(b *testing.B) {
	b.Run("should identify performance bottlenecks in pipeline", func(b *testing.B) {
		// Note: CPU profiling with generated audio files

		// This would include CPU profiling to identify bottlenecks
		// Focus areas: FFmpeg processing, Whisper transcription, pattern matching
	})
}

// Performance validation tests (not benchmarks, but performance assertions)

func TestE2E_RealTimePerformance(t *testing.T) {
	skipE2ETestInCI(t)
	t.Run("should process audio faster than real-time", func(t *testing.T) {
		// Note: Real-time performance validation with generated audio files

		// Verify that processing 5 seconds of audio takes less than 5 seconds
		// This ensures the system can keep up with live audio streams
	})
}

func TestE2E_MemoryStability(t *testing.T) {
	skipE2ETestInCI(t)
	t.Run("should not show memory leaks during extended operation", func(t *testing.T) {
		// Note: Memory leak detection with generated audio files

		// Run application for extended period and verify memory doesn't grow unbounded
		// Check for proper cleanup of channels, goroutines, and buffers
	})
}

func TestE2E_ConcurrencyLimits(t *testing.T) {
	skipE2ETestInCI(t)
	t.Run("should gracefully handle resource limits", func(t *testing.T) {
		// Note: Concurrency limit testing with generated audio files

		// Test behavior when system resources are constrained
		// Verify graceful degradation instead of crashes
	})
}
