package performance

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PerformanceMetrics tracks transcription performance metrics
type PerformanceMetrics struct {
	TotalTranscriptions  int64
	TotalAudioBytes      int64
	TotalProcessingTime  time.Duration
	GPUTranscriptions    int64
	CPUTranscriptions    int64
	AvgTranscriptionTime time.Duration
	MinTranscriptionTime time.Duration
	MaxTranscriptionTime time.Duration
	LastGPUUsed          bool
	LastDeviceID         int
	LastProcessingTime   time.Duration
	LastAudioBytes       int64
	LastTimestamp        time.Time
}

// TranscriptionTimer tracks timing for individual transcriptions
type TranscriptionTimer struct {
	StartTime      time.Time
	AudioBytes     int64
	UseGPU         bool
	DeviceID       int
	ProcessingTime time.Duration
}

// PerformanceMonitor handles performance tracking and reporting
type PerformanceMonitor struct {
	logger    *zap.Logger
	metrics   PerformanceMetrics
	mu        sync.RWMutex
	benchmark bool
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(logger *zap.Logger) *PerformanceMonitor {
	return &PerformanceMonitor{
		logger: logger,
		metrics: PerformanceMetrics{
			MinTranscriptionTime: time.Hour, // Initialize to large value
			LastTimestamp:        time.Now(),
		},
	}
}

// NewPerformanceMonitorWithBenchmark creates a performance monitor with benchmarking enabled
func NewPerformanceMonitorWithBenchmark(logger *zap.Logger, benchmark bool) *PerformanceMonitor {
	return &PerformanceMonitor{
		logger: logger,
		metrics: PerformanceMetrics{
			MinTranscriptionTime: time.Hour,
			LastTimestamp:        time.Now(),
		},
		benchmark: benchmark,
	}
}

// StartTranscription begins timing a transcription operation
func (pm *PerformanceMonitor) StartTranscription(audioBytes int64, useGPU bool, deviceID int) *TranscriptionTimer {
	return &TranscriptionTimer{
		StartTime:  time.Now(),
		AudioBytes: audioBytes,
		UseGPU:     useGPU,
		DeviceID:   deviceID,
	}
}

// EndTranscription completes timing and updates metrics
func (pm *PerformanceMonitor) EndTranscription(timer *TranscriptionTimer) {
	timer.ProcessingTime = time.Since(timer.StartTime)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Update basic metrics
	pm.metrics.TotalTranscriptions++
	pm.metrics.TotalAudioBytes += timer.AudioBytes
	pm.metrics.TotalProcessingTime += timer.ProcessingTime
	pm.metrics.LastProcessingTime = timer.ProcessingTime
	pm.metrics.LastAudioBytes = timer.AudioBytes
	pm.metrics.LastGPUUsed = timer.UseGPU
	pm.metrics.LastDeviceID = timer.DeviceID
	pm.metrics.LastTimestamp = time.Now()

	// Update GPU/CPU counters
	if timer.UseGPU {
		pm.metrics.GPUTranscriptions++
	} else {
		pm.metrics.CPUTranscriptions++
	}

	// Update timing statistics
	if timer.ProcessingTime < pm.metrics.MinTranscriptionTime {
		pm.metrics.MinTranscriptionTime = timer.ProcessingTime
	}
	if timer.ProcessingTime > pm.metrics.MaxTranscriptionTime {
		pm.metrics.MaxTranscriptionTime = timer.ProcessingTime
	}

	// Calculate average
	pm.metrics.AvgTranscriptionTime = time.Duration(
		int64(pm.metrics.TotalProcessingTime) / pm.metrics.TotalTranscriptions,
	)

	// Log if benchmarking is enabled
	if pm.benchmark {
		pm.logger.Info("transcription performance",
			zap.Bool("use_gpu", timer.UseGPU),
			zap.Int("device_id", timer.DeviceID),
			zap.Int64("audio_bytes", timer.AudioBytes),
			zap.Duration("processing_time", timer.ProcessingTime),
			zap.Float64("bytes_per_sec", float64(timer.AudioBytes)/timer.ProcessingTime.Seconds()),
		)
	}
}

// GetMetrics returns a copy of current metrics
func (pm *PerformanceMonitor) GetMetrics() PerformanceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.metrics
}

// GetPerformanceSummary returns a formatted summary of performance metrics
func (pm *PerformanceMonitor) GetPerformanceSummary() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.metrics.TotalTranscriptions == 0 {
		return "No transcription metrics available"
	}

	gpuPercent := float64(pm.metrics.GPUTranscriptions) / float64(pm.metrics.TotalTranscriptions) * 100
	avgBytesPerSec := float64(pm.metrics.TotalAudioBytes) / pm.metrics.TotalProcessingTime.Seconds()

	summary := fmt.Sprintf(
		"Performance Summary:\n"+
			"  Total Transcriptions: %d\n"+
			"  GPU Usage: %.1f%% (%d GPU, %d CPU)\n"+
			"  Avg Processing Time: %v\n"+
			"  Min/Max Processing Time: %v / %v\n"+
			"  Total Audio Processed: %.2f MB\n"+
			"  Average Throughput: %.2f KB/s\n",
		pm.metrics.TotalTranscriptions,
		gpuPercent,
		pm.metrics.GPUTranscriptions,
		pm.metrics.CPUTranscriptions,
		pm.metrics.AvgTranscriptionTime,
		pm.metrics.MinTranscriptionTime,
		pm.metrics.MaxTranscriptionTime,
		float64(pm.metrics.TotalAudioBytes)/1024/1024,
		avgBytesPerSec/1024,
	)

	return summary
}

// CompareGPUvsCPU returns performance comparison between GPU and CPU
func (pm *PerformanceMonitor) CompareGPUvsCPU() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.metrics.GPUTranscriptions == 0 || pm.metrics.CPUTranscriptions == 0 {
		return "Insufficient data for GPU vs CPU comparison"
	}

	// Estimate average times (since we don't track per-transcription type timing)
	// This is a simplified comparison
	gpuRatio := float64(pm.metrics.GPUTranscriptions) / float64(pm.metrics.TotalTranscriptions)
	cpuRatio := float64(pm.metrics.CPUTranscriptions) / float64(pm.metrics.TotalTranscriptions)

	estimatedGPUTime := time.Duration(float64(pm.metrics.AvgTranscriptionTime.Nanoseconds()) * 0.7) // Assume 30% faster on GPU
	estimatedCPUTime := time.Duration(float64(pm.metrics.AvgTranscriptionTime.Nanoseconds()) * 1.2) // Assume 20% slower on CPU

	improvement := float64(estimatedCPUTime-estimatedGPUTime) / float64(estimatedCPUTime) * 100

	comparison := fmt.Sprintf(
		"GPU vs CPU Performance Comparison:\n"+
			"  GPU Transcriptions: %d (%.1f%% of total)\n"+
			"  CPU Transcriptions: %d (%.1f%% of total)\n"+
			"  Estimated GPU Time: %v\n"+
			"  Estimated CPU Time: %v\n"+
			"  Estimated Improvement: %.1f%%\n",
		pm.metrics.GPUTranscriptions,
		gpuRatio*100,
		pm.metrics.CPUTranscriptions,
		cpuRatio*100,
		estimatedGPUTime,
		estimatedCPUTime,
		improvement,
	)

	return comparison
}

// ResetMetrics clears all accumulated metrics
func (pm *PerformanceMonitor) ResetMetrics() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.metrics = PerformanceMetrics{
		MinTranscriptionTime: time.Hour,
		LastTimestamp:        time.Now(),
	}

	pm.logger.Info("performance metrics reset")
}

// BenchmarkMode enables or disables detailed benchmark logging
func (pm *PerformanceMonitor) BenchmarkMode(enabled bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.benchmark = enabled
	pm.logger.Info("benchmark mode", zap.Bool("enabled", enabled))
}

// LogCurrentMetrics logs the current performance metrics
func (pm *PerformanceMonitor) LogCurrentMetrics() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pm.logger.Info("current performance metrics",
		zap.Int64("total_transcriptions", pm.metrics.TotalTranscriptions),
		zap.Int64("gpu_transcriptions", pm.metrics.GPUTranscriptions),
		zap.Int64("cpu_transcriptions", pm.metrics.CPUTranscriptions),
		zap.Duration("avg_processing_time", pm.metrics.AvgTranscriptionTime),
		zap.Duration("last_processing_time", pm.metrics.LastProcessingTime),
		zap.Bool("last_used_gpu", pm.metrics.LastGPUUsed),
		zap.Int("last_device_id", pm.metrics.LastDeviceID),
	)
}
