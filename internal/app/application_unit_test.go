package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"radiocontestwinner/internal/buffer"
	"radiocontestwinner/internal/parser"
	"radiocontestwinner/internal/transcriber"
)

// TestApplication_HealthTracking tests the health tracking methods
func TestApplication_HealthTrackingMethods(t *testing.T) {
	app, err := NewApplication()
	require.NoError(t, err)

	t.Run("should update stream health", func(t *testing.T) {
		// Initially inactive
		assert.False(t, app.pipelineHealth.streamConnectionActive)

		// Update to active
		app.updateStreamHealth(true)
		assert.True(t, app.pipelineHealth.streamConnectionActive)

		// Update to inactive
		app.updateStreamHealth(false)
		assert.False(t, app.pipelineHealth.streamConnectionActive)
	})

	t.Run("should update audio processing health", func(t *testing.T) {
		// Initially inactive
		assert.False(t, app.pipelineHealth.audioProcessingActive)

		// Update to active
		app.updateAudioProcessingHealth(true)
		assert.True(t, app.pipelineHealth.audioProcessingActive)

		// Update to inactive
		app.updateAudioProcessingHealth(false)
		assert.False(t, app.pipelineHealth.audioProcessingActive)
	})

	t.Run("should update transcription health", func(t *testing.T) {
		oldTime := app.pipelineHealth.lastTranscriptionTime
		oldCount := app.pipelineHealth.totalTranscriptions

		// Update transcription health
		app.updateTranscriptionHealth()

		// Should increment count and update timestamp
		assert.Greater(t, app.pipelineHealth.totalTranscriptions, oldCount)
		assert.True(t, app.pipelineHealth.lastTranscriptionTime.After(oldTime))
		assert.True(t, app.pipelineHealth.transcriptionActive)
	})

	t.Run("should update buffered context health", func(t *testing.T) {
		oldTime := app.pipelineHealth.lastBufferedContextTime

		// Update buffered context health
		app.updateBufferedContextHealth()

		// Should update timestamp
		assert.True(t, app.pipelineHealth.lastBufferedContextTime.After(oldTime))
	})

	t.Run("should update contest cue health", func(t *testing.T) {
		oldTime := app.pipelineHealth.lastContestCueTime
		oldCount := app.pipelineHealth.totalContestCues

		// Update contest cue health
		app.updateContestCueHealth()

		// Should increment count and update timestamp
		assert.Greater(t, app.pipelineHealth.totalContestCues, oldCount)
		assert.True(t, app.pipelineHealth.lastContestCueTime.After(oldTime))
	})
}

// TestApplication_TranscriptionPerformanceMetrics tests performance metrics tracking
func TestApplication_TranscriptionPerformanceMetrics(t *testing.T) {
	app, err := NewApplication()
	require.NoError(t, err)

	t.Run("should track transcription performance metrics", func(t *testing.T) {
		// Create test segment
		segment := transcriber.TranscriptionSegment{
			Text:       "Test transcription",
			StartMS:    1000,
			EndMS:      2000,
			Confidence: 0.95,
		}

		startTime := time.Now().Add(-100 * time.Millisecond)

		// Update performance metrics
		app.updateTranscriptionPerformance(segment, startTime)

		// Should update performance metrics
		assert.Greater(t, app.pipelineHealth.totalAudioDurationMS, int64(0))
		assert.Greater(t, app.pipelineHealth.averageLatencyMS, float64(0))
	})

	t.Run("should calculate moving average latency", func(t *testing.T) {
		segment := transcriber.TranscriptionSegment{
			Text:       "Test",
			StartMS:    0,
			EndMS:      1000,
			Confidence: 0.8,
		}

		// Record multiple latencies to test averaging
		for i := 0; i < 3; i++ {
			startTime := time.Now().Add(-time.Duration(i*10+50) * time.Millisecond)
			app.updateTranscriptionPerformance(segment, startTime)
		}

		// Should have a reasonable average latency
		assert.Greater(t, app.pipelineHealth.averageLatencyMS, 0.0)
		assert.Less(t, app.pipelineHealth.averageLatencyMS, 1000.0) // Should be less than 1 second
	})
}

// TestApplication_HealthStatus tests health status reporting
func TestApplication_HealthStatus(t *testing.T) {
	app, err := NewApplication()
	require.NoError(t, err)

	t.Run("should return comprehensive health status", func(t *testing.T) {
		// Update some health metrics
		app.updateStreamHealth(true)
		app.updateAudioProcessingHealth(true)
		app.updateTranscriptionHealth()

		// Get health status
		status := app.getPipelineHealthStatus()

		// Should contain all expected fields
		expectedFields := []string{
			"stream_connected", "audio_processing_active", "transcription_active",
			"total_transcriptions", "total_contest_cues", "last_transcription_time",
			"last_buffered_context_time", "last_contest_cue_time", "average_latency_ms",
			"total_audio_duration_ms", "current_backlog_size", "is_real_time",
		}

		for _, field := range expectedFields {
			assert.Contains(t, status, field, "Health status should contain field %s", field)
		}

		// Check some specific values
		assert.True(t, status["stream_connected"].(bool))
		assert.True(t, status["audio_processing_active"].(bool))
		assert.True(t, status["transcription_active"].(bool))
		assert.Greater(t, status["total_transcriptions"].(int64), int64(0))
	})
}

// TestApplication_SystemHealthCheck tests overall system health evaluation
func TestApplication_SystemHealthCheck(t *testing.T) {
	app, err := NewApplication()
	require.NoError(t, err)

	t.Run("should report healthy when all systems active", func(t *testing.T) {
		// Set up healthy state
		app.updateStreamHealth(true)
		app.updateAudioProcessingHealth(true)
		app.updateTranscriptionHealth()

		status := app.getPipelineHealthStatus()
		healthy := app.isSystemHealthy(status)

		assert.True(t, healthy)
	})

	t.Run("should report unhealthy when stream disconnected but processing active", func(t *testing.T) {
		// Set up unhealthy state - processing without stream
		app.updateStreamHealth(false)
		app.updateAudioProcessingHealth(true)

		status := app.getPipelineHealthStatus()
		healthy := app.isSystemHealthy(status)

		assert.False(t, healthy)
	})

	t.Run("should report unhealthy when transcriptions started but transcription inactive", func(t *testing.T) {
		// Set up state where we have transcriptions but transcription is inactive
		app.pipelineHealth.totalTranscriptions = 5
		app.pipelineHealth.transcriptionActive = false

		status := app.getPipelineHealthStatus()
		healthy := app.isSystemHealthy(status)

		assert.False(t, healthy)
	})

	t.Run("should report healthy when no activity yet", func(t *testing.T) {
		// Reset to initial state
		app.pipelineHealth.streamConnectionActive = false
		app.pipelineHealth.audioProcessingActive = false
		app.pipelineHealth.transcriptionActive = false
		app.pipelineHealth.totalTranscriptions = 0

		status := app.getPipelineHealthStatus()
		healthy := app.isSystemHealthy(status)

		assert.True(t, healthy) // Should be healthy when nothing started yet
	})
}

// TestApplication_DebugTranscriptionFileWriting tests debug file writing
func TestApplication_DebugTranscriptionFileWriting(t *testing.T) {
	app, err := NewApplication()
	require.NoError(t, err)

	t.Run("should handle debug file write permission errors gracefully", func(t *testing.T) {
		segment := transcriber.TranscriptionSegment{
			Text:       "Test transcription",
			StartMS:    1000,
			EndMS:      2000,
			Confidence: 0.95,
		}

		// This should not panic even if it can't write to /app/logs/
		assert.NotPanics(t, func() {
			app.writeTranscriptionToDebugFile(segment)
		})
	})
}

// TestApplication_ChannelWrapping tests health tracking channel wrappers
func TestApplication_ChannelWrapping(t *testing.T) {
	app, err := NewApplication()
	require.NoError(t, err)

	t.Run("should wrap transcription channel with health tracking", func(t *testing.T) {
		// Create original channel
		originalCh := make(chan transcriber.TranscriptionSegment, 1)

		// Wrap with health tracking
		wrappedCh := app.wrapTranscriptionChannelWithHealthTracking(originalCh)

		assert.NotNil(t, wrappedCh)

		// Should be different channels
		assert.NotEqual(t, originalCh, wrappedCh)
	})

	t.Run("should wrap buffered context channel with health tracking", func(t *testing.T) {
		// Create original channel
		originalCh := make(chan buffer.BufferedContext, 1)

		// Wrap with health tracking
		wrappedCh := app.wrapBufferedContextChannelWithHealthTracking(originalCh)

		assert.NotNil(t, wrappedCh)

		// Should be different channels
		assert.NotEqual(t, originalCh, wrappedCh)
	})

	t.Run("should wrap contest cue channel with health tracking", func(t *testing.T) {
		// Create original channel
		originalCh := make(chan parser.ContestCue, 1)

		// Wrap with health tracking
		wrappedCh := app.wrapContestCueChannelWithHealthTracking(originalCh)

		assert.NotNil(t, wrappedCh)

		// Should be different channels
		assert.NotEqual(t, originalCh, wrappedCh)
	})
}

// TestApplication_HealthFileWritingFunc tests health file writing functionality
func TestApplication_HealthFileWritingFunc(t *testing.T) {
	app, err := NewApplication()
	require.NoError(t, err)

	t.Run("should handle health file write errors gracefully", func(t *testing.T) {
		// Set up some health data
		app.updateStreamHealth(true)
		app.updateTranscriptionHealth()

		// This should not panic even if it can't write to /app/health.json
		err := app.writeHealthStatusFile()

		// May return error due to permissions, but should not panic
		// In test environment, /app/ directory likely doesn't exist or isn't writable
		if err != nil {
			assert.Contains(t, err.Error(), "permission denied")
		}
	})
}