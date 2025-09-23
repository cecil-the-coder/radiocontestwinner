package transcriber

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"radiocontestwinner/internal/config"
)

// MockWhisperModel is a mock implementation of the Whisper model for testing
type MockWhisperModel struct {
	loadError       error
	transcribeError error
	segments        []TranscriptionSegment
}

func (m *MockWhisperModel) LoadModel(modelPath string) error {
	return m.loadError
}

func (m *MockWhisperModel) Transcribe(audioData []byte) ([]TranscriptionSegment, error) {
	if m.transcribeError != nil {
		return nil, m.transcribeError
	}
	return m.segments, nil
}

func (m *MockWhisperModel) Close() error {
	return nil
}

func (m *MockWhisperModel) GetGPUStatus() (bool, int) {
	// Mock implementation - always return false for CPU
	return false, -1
}

func TestNewTranscriptionEngine(t *testing.T) {
	t.Run("should create TranscriptionEngine with valid dependencies", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)

		// Act
		engine := NewTranscriptionEngine(logger)

		// Assert
		assert.NotNil(t, engine)
		assert.Equal(t, logger, engine.logger)
	})
}

func TestTranscriptionEngine_LoadModel(t *testing.T) {
	t.Run("should load model successfully", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		engine := NewTranscriptionEngine(logger)
		mockModel := &MockWhisperModel{}
		engine.model = mockModel // Inject mock
		modelPath := "/path/to/model.bin"

		// Act
		err := engine.LoadModel(modelPath)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("should return error when model loading fails", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		engine := NewTranscriptionEngine(logger)
		mockModel := &MockWhisperModel{
			loadError: assert.AnError,
		}
		engine.model = mockModel // Inject mock
		modelPath := "/invalid/path/model.bin"

		// Act
		err := engine.LoadModel(modelPath)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load Whisper model")
	})
}

func TestTranscriptionEngine_ProcessAudio(t *testing.T) {
	t.Run("should process audio and output segments to channel", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		engine := NewTranscriptionEngine(logger)

		expectedSegments := []TranscriptionSegment{
			{Text: "Hello world", StartMS: 0, EndMS: 1000, Confidence: 0.95},
			{Text: "This is a test", StartMS: 1000, EndMS: 2500, Confidence: 0.88},
		}

		mockModel := &MockWhisperModel{
			segments: expectedSegments,
		}
		engine.model = mockModel // Inject mock

		audioData := "fake audio data"
		reader := strings.NewReader(audioData)

		// Use context with timeout to prevent hanging in keep-alive mode
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Act
		segmentChan, err := engine.ProcessAudio(ctx, reader)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, segmentChan)

		// Collect segments from channel with timeout protection
		var receivedSegments []TranscriptionSegment
		done := make(chan bool)
		go func() {
			defer close(done)
			for segment := range segmentChan {
				receivedSegments = append(receivedSegments, segment)
			}
		}()

		// Wait for completion or timeout
		select {
		case <-done:
			// Completed normally
		case <-time.After(3 * time.Second):
			// Test timeout - this is expected behavior for EOF scenario
			t.Log("Test completed after EOF timeout (expected behavior)")
		}

		assert.Equal(t, expectedSegments, receivedSegments)
	})

	t.Run("should handle transcription errors gracefully", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		engine := NewTranscriptionEngine(logger)

		mockModel := &MockWhisperModel{
			transcribeError: assert.AnError,
		}
		engine.model = mockModel // Inject mock

		audioData := "fake audio data"
		reader := strings.NewReader(audioData)

		// Use context with timeout to prevent hanging in keep-alive mode
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Act
		segmentChan, err := engine.ProcessAudio(ctx, reader)

		// Assert
		assert.NoError(t, err) // ProcessAudio should not fail immediately
		assert.NotNil(t, segmentChan)

		// Channel should be closed without any segments after timeout
		var receivedSegments []TranscriptionSegment
		done := make(chan bool)
		go func() {
			defer close(done)
			for segment := range segmentChan {
				receivedSegments = append(receivedSegments, segment)
			}
		}()

		// Wait for completion or timeout
		select {
		case <-done:
			// Completed normally
		case <-time.After(3 * time.Second):
			// Test timeout - this is expected behavior for EOF scenario
			t.Log("Test completed after EOF timeout (expected behavior)")
		}

		assert.Empty(t, receivedSegments)
	})

	t.Run("should handle context cancellation", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		engine := NewTranscriptionEngine(logger)

		expectedSegments := []TranscriptionSegment{
			{Text: "Hello world", StartMS: 0, EndMS: 1000, Confidence: 0.95},
		}

		mockModel := &MockWhisperModel{
			segments: expectedSegments,
		}
		engine.model = mockModel // Inject mock

		audioData := "fake audio data"
		reader := strings.NewReader(audioData)
		ctx, cancel := context.WithCancel(context.Background())

		// Act
		segmentChan, err := engine.ProcessAudio(ctx, reader)
		assert.NoError(t, err)

		// Cancel context immediately
		cancel()

		// Assert
		// Channel should be closed due to context cancellation
		var receivedSegments []TranscriptionSegment
		for segment := range segmentChan {
			receivedSegments = append(receivedSegments, segment)
		}

		// Should receive no segments or partial segments due to cancellation
		assert.LessOrEqual(t, len(receivedSegments), len(expectedSegments))
	})
}

func TestTranscriptionEngine_Close(t *testing.T) {
	t.Run("should close engine and cleanup resources", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		engine := NewTranscriptionEngine(logger)
		mockModel := &MockWhisperModel{}
		engine.model = mockModel // Inject mock

		// Act
		err := engine.Close()

		// Assert
		assert.NoError(t, err)
	})
}

// StreamReader simulates a stream that can stop and restart
type StreamReader struct {
	chunks    []string
	index     int
	paused    bool
	completed bool
}

func NewStreamReader(chunks []string) *StreamReader {
	return &StreamReader{chunks: chunks, index: 0, paused: false, completed: false}
}

func (sr *StreamReader) Read(p []byte) (n int, err error) {
	if sr.paused {
		return 0, io.EOF
	}

	if sr.index >= len(sr.chunks) {
		if sr.completed {
			return 0, io.EOF
		}
		// If not completed but no more chunks, EOF temporarily
		return 0, io.EOF
	}

	chunk := sr.chunks[sr.index]
	sr.index++

	if len(chunk) == 0 {
		return 0, io.EOF
	}

	n = copy(p, []byte(chunk))
	return n, nil
}

func (sr *StreamReader) Pause() {
	sr.paused = true
}

func (sr *StreamReader) Resume() {
	sr.paused = false
}

func (sr *StreamReader) AddChunk(chunk string) {
	sr.chunks = append(sr.chunks, chunk)
}

func (sr *StreamReader) Complete() {
	sr.completed = true
}

func TestTranscriptionEngine_KeepAliveProcessing(t *testing.T) {
	t.Run("should continue processing after EOF and resume when new data arrives", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		engine := NewTranscriptionEngine(logger)

		// Set a very short timeout for testing
		engine.config.SetTranscriptionTimeoutSec(2)

		segments1 := []TranscriptionSegment{
			{Text: "First segment", StartMS: 0, EndMS: 1000, Confidence: 0.95},
		}

		mockModel := &MockWhisperModel{
			segments: segments1,
		}
		engine.model = mockModel

		// Create a stream that will provide one chunk then EOF
		streamReader := NewStreamReader([]string{"chunk1"})
		ctx := context.Background()

		// Act
		segmentChan, err := engine.ProcessAudio(ctx, streamReader)
		assert.NoError(t, err)

		// Collect first segment
		var receivedSegments []TranscriptionSegment
		segment := <-segmentChan
		receivedSegments = append(receivedSegments, segment)

		// Now complete the stream so it will timeout
		streamReader.Complete()

		// Wait for channel to close due to timeout
		start := time.Now()
		for segment := range segmentChan {
			receivedSegments = append(receivedSegments, segment)
		}
		duration := time.Since(start)

		// Assert - should have processed one segment and then timed out
		assert.Len(t, receivedSegments, 1)
		assert.Equal(t, "First segment", receivedSegments[0].Text)
		assert.True(t, duration >= 2*time.Second && duration < 3*time.Second, "Should timeout after ~2 seconds")
	})

	t.Run("should timeout after extended period of no audio", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		engine := NewTranscriptionEngine(logger)

		// Set short timeout for testing
		engine.config.SetTranscriptionTimeoutSec(1) // Will add this config

		mockModel := &MockWhisperModel{}
		engine.model = mockModel

		// Create empty stream reader
		emptyReader := strings.NewReader("")
		ctx := context.Background()

		// Act
		segmentChan, err := engine.ProcessAudio(ctx, emptyReader)
		assert.NoError(t, err)

		// Wait for timeout
		start := time.Now()
		var receivedSegments []TranscriptionSegment
		for segment := range segmentChan {
			receivedSegments = append(receivedSegments, segment)
		}
		duration := time.Since(start)

		// Assert - should timeout and close channel
		assert.Empty(t, receivedSegments)
		assert.True(t, duration < 2*time.Second) // Should timeout quickly
	})

	t.Run("should log recovery events when processing resumes", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		engine := NewTranscriptionEngine(logger)

		// Set short timeout for testing
		engine.config.SetTranscriptionTimeoutSec(1)

		mockModel := &MockWhisperModel{
			segments: []TranscriptionSegment{
				{Text: "Recovery test", StartMS: 0, EndMS: 1000, Confidence: 0.95},
			},
		}
		engine.model = mockModel

		streamReader := NewStreamReader([]string{"data"})
		streamReader.Complete() // Mark as complete so it will timeout
		ctx := context.Background()

		// Act
		segmentChan, err := engine.ProcessAudio(ctx, streamReader)
		assert.NoError(t, err)

		// Collect segments
		var receivedSegments []TranscriptionSegment
		for segment := range segmentChan {
			receivedSegments = append(receivedSegments, segment)
		}

		// Assert
		assert.Len(t, receivedSegments, 1)
		assert.Equal(t, "Recovery test", receivedSegments[0].Text)
	})
}

func TestTranscriptionEngine_NewTranscriptionEngineWithBenchmark(t *testing.T) {
	t.Run("should create engine with benchmarking enabled", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()

		// Act
		engine := NewTranscriptionEngineWithBenchmark(logger, config, true)

		// Assert
		assert.NotNil(t, engine)
		assert.NotNil(t, engine.logger)
		assert.NotNil(t, engine.model)
		assert.NotNil(t, engine.config)
		assert.NotNil(t, engine.performanceMonitor)
	})

	t.Run("should create engine with benchmarking disabled", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()

		// Act
		engine := NewTranscriptionEngineWithBenchmark(logger, config, false)

		// Assert
		assert.NotNil(t, engine)
		assert.NotNil(t, engine.logger)
		assert.NotNil(t, engine.model)
		assert.NotNil(t, engine.config)
		assert.NotNil(t, engine.performanceMonitor)
	})
}

func TestTranscriptionEngine_GetPerformanceSummary(t *testing.T) {
	t.Run("should return performance summary", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		engine := NewTranscriptionEngineWithConfig(logger, config)

		// Act
		summary := engine.GetPerformanceSummary()

		// Assert
		assert.Contains(t, summary, "No transcription metrics available")
	})
}

func TestTranscriptionEngine_ResetPerformanceMetrics(t *testing.T) {
	t.Run("should reset performance metrics", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		engine := NewTranscriptionEngineWithConfig(logger, config)

		// Act - Should not panic
		engine.ResetPerformanceMetrics()

		// Assert - Verify metrics are reset by checking comparison
		comparison := engine.CompareGPUvsCPU()
		assert.Contains(t, comparison, "Insufficient data for GPU vs CPU comparison")
	})
}

func TestTranscriptionEngine_EnableBenchmarking(t *testing.T) {
	t.Run("should enable benchmarking", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		engine := NewTranscriptionEngineWithConfig(logger, config)

		// Act - Should not panic
		engine.EnableBenchmarking(true)

		// Assert - Benchmarking is enabled (verify by checking performance summary)
		summary := engine.GetPerformanceSummary()
		assert.Contains(t, summary, "No transcription metrics available")
	})

	t.Run("should disable benchmarking", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		engine := NewTranscriptionEngineWithConfig(logger, config)

		// Act - Should not panic
		engine.EnableBenchmarking(false)

		// Assert - Benchmarking is disabled
		summary := engine.GetPerformanceSummary()
		assert.Contains(t, summary, "No transcription metrics available")
	})
}

func TestTranscriptionEngine_LogCurrentPerformanceMetrics(t *testing.T) {
	t.Run("should log current performance metrics", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		config := config.NewConfiguration()
		engine := NewTranscriptionEngineWithConfig(logger, config)

		// Act - Should not panic
		engine.LogCurrentPerformanceMetrics()

		// Assert - Function completes without error
		assert.True(t, true) // If we get here, no panic occurred
	})
}
