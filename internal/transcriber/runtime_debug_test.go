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

func TestTranscriptionEngine_DebugModeCanBeToggledAtRuntime(t *testing.T) {
	// Arrange
	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	config := config.NewConfiguration()
	config.SetDebugMode(false) // Start with debug disabled

	engine := NewTranscriptionEngineWithConfig(logger, config)

	// Create a mock audio reader with multiple segments
	audioData := []byte("mock audio data")
	mockModel := &MockWhisperModel{
		segments: []TranscriptionSegment{
			{
				Text:      "First segment",
				StartMS:   1000,
				EndMS:     2000,
				Confidence: 0.92,
			},
			{
				Text:      "Second segment",
				StartMS:   2000,
				EndMS:     3000,
				Confidence: 0.88,
			},
		},
	}
	engine.model = mockModel

	// Act - Process audio with debug mode disabled
	audioReader := strings.NewReader(string(audioData))
	segmentChan, err := engine.ProcessAudio(context.Background(), audioReader)

	// Collect all segments
	var segments []TranscriptionSegment
	for segment := range segmentChan {
		segments = append(segments, segment)
	}

	// Assert - First processing should have no debug logs
	assert.NoError(t, err)
	assert.Len(t, segments, 2, "Should have two segments")

	debugLogsAfterFirst := countDebugLogs(observedLogs.All())
	assert.Equal(t, 0, debugLogsAfterFirst, "Should have no debug logs when debug mode is disabled")

	// Act - Toggle debug mode on and process again
	config.SetDebugMode(true)

	// Create new observer for second processing
	core2, observedLogs2 := observer.New(zapcore.DebugLevel)
	logger2 := zap.New(core2)
	engine2 := NewTranscriptionEngineWithConfig(logger2, config)
	engine2.model = mockModel

	audioReader2 := strings.NewReader(string(audioData))
	segmentChan2, err := engine2.ProcessAudio(context.Background(), audioReader2)

	// Collect all segments again
	var segments2 []TranscriptionSegment
	for segment := range segmentChan2 {
		segments2 = append(segments2, segment)
	}

	// Assert - Second processing should have debug logs
	assert.NoError(t, err)
	assert.Len(t, segments2, 2, "Should have two segments")

	debugLogsAfterSecond := countDebugLogs(observedLogs2.All())
	assert.Equal(t, 2, debugLogsAfterSecond, "Should have two debug logs when debug mode is enabled")
}

func TestTranscriptionEngine_MultipleSegmentsWithDebugEnabled(t *testing.T) {
	// Arrange
	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	config := config.NewConfiguration()
	config.SetDebugMode(true)

	engine := NewTranscriptionEngineWithConfig(logger, config)

	// Create a mock audio reader with multiple segments
	audioData := []byte("mock audio data")
	mockModel := &MockWhisperModel{
		segments: []TranscriptionSegment{
			{
				Text:      "Hello world",
				StartMS:   1000,
				EndMS:     2000,
				Confidence: 0.92,
			},
			{
				Text:      "This is a test",
				StartMS:   2000,
				EndMS:     3000,
				Confidence: 0.85,
			},
			{
				Text:      "Final segment",
				StartMS:   3000,
				EndMS:     4000,
				Confidence: 0.95,
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
	assert.Len(t, segments, 3, "Should have three segments")

	// Check that debug logs were created for each segment
	debugLogCount := countDebugLogs(observedLogs.All())
	assert.Equal(t, 3, debugLogCount, "Should have three debug logs, one for each segment")

	// Verify each segment was logged correctly
	debugLogs := make([]observer.LoggedEntry, 0)
	for _, log := range observedLogs.All() {
		if log.Level == zapcore.DebugLevel &&
		   log.Message == "Transcription segment" &&
		   log.ContextMap()["component"] == "transcriber" {
			debugLogs = append(debugLogs, log)
		}
	}

	assert.Len(t, debugLogs, 3, "Should have exactly 3 debug logs")

	// Verify each segment has a corresponding debug log
	for i, segment := range segments {
		found := false
		for _, log := range debugLogs {
			data := log.ContextMap()["data"].(map[string]interface{})
			if data["text"] == segment.Text &&
			   data["start_ms"] == segment.StartMS &&
			   data["end_ms"] == segment.EndMS &&
			   data["confidence"] == segment.Confidence {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find debug log for segment %d: %s", i, segment.Text)
	}
}

func countDebugLogs(allLogs []observer.LoggedEntry) int {
	count := 0
	for _, log := range allLogs {
		if log.Level == zapcore.DebugLevel &&
		   log.Message == "Transcription segment" &&
		   log.ContextMap()["component"] == "transcriber" {
			count++
		}
	}
	return count
}