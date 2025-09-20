package transcriber

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"radiocontestwinner/internal/config"
)

func TestTranscriptionEngine_LogsDebugOutput_WhenDebugModeEnabled(t *testing.T) {
	// Arrange
	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	config := config.NewConfiguration()
	config.SetDebugMode(true)

	engine := NewTranscriptionEngineWithConfig(logger, config)

	// Create a mock audio reader
	audioData := []byte("mock audio data")
	mockModel := &MockWhisperModel{
		segments: []TranscriptionSegment{
			{
				Text:      "Hello world",
				StartMS:   1000,
				EndMS:     2000,
				Confidence: 0.92,
			},
		},
	}
	engine.model = mockModel

	// Act
	audioReader := strings.NewReader(string(audioData))
	segmentChan, err := engine.ProcessAudio(context.Background(), audioReader)

	// Collect all segments
	var segments []TranscriptionSegment
	for segment := range segmentChan {
		segments = append(segments, segment)
	}

	// Assert
	assert.NoError(t, err)
	assert.Len(t, segments, 1, "Should have one segment")

	// Check that debug log was created
	debugLogs := 0
	for _, log := range observedLogs.All() {
		if log.Level == zapcore.DebugLevel &&
		   log.Message == "Transcription segment" &&
		   log.ContextMap()["component"] == "transcriber" {
			debugLogs++

			// Verify the debug log contains expected data
			data := log.ContextMap()["data"].(map[string]interface{})
			assert.Equal(t, "Hello world", data["text"])
			assert.Equal(t, 1000, data["start_ms"])
			assert.Equal(t, 2000, data["end_ms"])
			assert.Equal(t, float32(0.92), data["confidence"])
		}
	}
	assert.Equal(t, 1, debugLogs, "Should have exactly one debug log for transcription segment")
}

func TestTranscriptionEngine_DoesNotLogDebugOutput_WhenDebugModeDisabled(t *testing.T) {
	// Arrange
	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	config := config.NewConfiguration()
	config.SetDebugMode(false) // Explicitly disabled

	engine := NewTranscriptionEngineWithConfig(logger, config)

	// Create a mock audio reader
	audioData := []byte("mock audio data")
	mockModel := &MockWhisperModel{
		segments: []TranscriptionSegment{
			{
				Text:      "Hello world",
				StartMS:   1000,
				EndMS:     2000,
				Confidence: 0.92,
			},
		},
	}
	engine.model = mockModel

	// Act
	audioReader := strings.NewReader(string(audioData))
	segmentChan, err := engine.ProcessAudio(context.Background(), audioReader)

	// Collect all segments
	var segments []TranscriptionSegment
	for segment := range segmentChan {
		segments = append(segments, segment)
	}

	// Assert
	assert.NoError(t, err)
	assert.Len(t, segments, 1, "Should have one segment")

	// Check that NO debug log was created
	debugLogs := 0
	for _, log := range observedLogs.All() {
		if log.Level == zapcore.DebugLevel &&
		   log.Message == "Transcription segment" &&
		   log.ContextMap()["component"] == "transcriber" {
			debugLogs++
		}
	}
	assert.Equal(t, 0, debugLogs, "Should have no debug logs when debug mode is disabled")
}

