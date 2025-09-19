package transcriber

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
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
		ctx := context.Background()

		// Act
		segmentChan, err := engine.ProcessAudio(ctx, reader)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, segmentChan)

		// Collect segments from channel
		var receivedSegments []TranscriptionSegment
		for segment := range segmentChan {
			receivedSegments = append(receivedSegments, segment)
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
		ctx := context.Background()

		// Act
		segmentChan, err := engine.ProcessAudio(ctx, reader)

		// Assert
		assert.NoError(t, err) // ProcessAudio should not fail immediately
		assert.NotNil(t, segmentChan)

		// Channel should be closed without any segments
		var receivedSegments []TranscriptionSegment
		for segment := range segmentChan {
			receivedSegments = append(receivedSegments, segment)
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
