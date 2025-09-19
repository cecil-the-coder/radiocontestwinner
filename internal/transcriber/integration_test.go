package transcriber

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// MockAudioProcessor simulates the AudioProcessor for integration testing
type MockAudioProcessor struct {
	audioData []byte
	closed    bool
}

// NewMockAudioProcessor creates a mock audio processor with simulated PCM audio data
func NewMockAudioProcessor(audioData []byte) *MockAudioProcessor {
	return &MockAudioProcessor{
		audioData: audioData,
	}
}

// Read implements io.Reader interface for the mock
func (m *MockAudioProcessor) Read(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}

	if len(m.audioData) == 0 {
		m.closed = true
		return 0, io.EOF
	}

	// Copy available data to the buffer
	n = copy(p, m.audioData)
	m.audioData = m.audioData[n:]

	return n, nil
}

// Close simulates closing the audio processor
func (m *MockAudioProcessor) Close() error {
	m.closed = true
	return nil
}

func TestTranscriptionPipeline_Integration(t *testing.T) {
	t.Run("should process audio through complete transcription pipeline", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)

		// Create mock audio data (simulating 16kHz 16-bit mono PCM for 6 seconds)
		audioData := make([]byte, 16000*2*6) // 6 seconds of audio
		for i := range audioData {
			audioData[i] = byte(i % 256) // Fill with test pattern
		}

		mockAudioProcessor := NewMockAudioProcessor(audioData)
		transcriptionEngine := NewTranscriptionEngine(logger)

		var outputBuffer bytes.Buffer
		jsonOutput := NewJSONOutput(&outputBuffer, logger)

		modelPath := "./models/ggml-base.en.bin"

		// Act
		// Step 1: Load the model
		err := transcriptionEngine.LoadModel(modelPath)
		assert.NoError(t, err)

		// Step 2: Process audio
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		segmentChan, err := transcriptionEngine.ProcessAudio(ctx, mockAudioProcessor)
		assert.NoError(t, err)

		// Step 3: Output segments as JSON
		var receivedSegments []TranscriptionSegment
		for segment := range segmentChan {
			receivedSegments = append(receivedSegments, segment)
			err := jsonOutput.OutputSegment(segment)
			assert.NoError(t, err)
		}

		// Step 4: Cleanup
		err = transcriptionEngine.Close()
		assert.NoError(t, err)
		err = jsonOutput.Close()
		assert.NoError(t, err)
		err = mockAudioProcessor.Close()
		assert.NoError(t, err)

		// Assert
		assert.NotEmpty(t, receivedSegments, "should receive at least one transcription segment")

		// Verify JSON output format
		jsonOutputString := outputBuffer.String()
		assert.NotEmpty(t, jsonOutputString, "should produce JSON output")

		lines := strings.Split(strings.TrimSpace(jsonOutputString), "\n")
		assert.Equal(t, len(receivedSegments), len(lines), "should have one JSON line per segment")

		// Verify each JSON line is valid and matches received segments
		for i, line := range lines {
			var segment TranscriptionSegment
			err := json.Unmarshal([]byte(line), &segment)
			assert.NoError(t, err, "each line should be valid JSON")
			assert.Equal(t, receivedSegments[i], segment, "JSON output should match received segment")

			// Verify segment structure according to acceptance criteria
			assert.NotEmpty(t, segment.Text, "segment should have text")
			assert.GreaterOrEqual(t, segment.StartMS, 0, "start_ms should be non-negative")
			assert.Greater(t, segment.EndMS, segment.StartMS, "end_ms should be greater than start_ms")
			assert.GreaterOrEqual(t, segment.Confidence, float32(0.0), "confidence should be >= 0.0")
			assert.LessOrEqual(t, segment.Confidence, float32(1.0), "confidence should be <= 1.0")
		}
	})

	t.Run("should handle context cancellation gracefully", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		audioData := make([]byte, 16000*2*10) // 10 seconds of audio
		mockAudioProcessor := NewMockAudioProcessor(audioData)
		transcriptionEngine := NewTranscriptionEngine(logger)

		modelPath := "./models/ggml-base.en.bin"
		err := transcriptionEngine.LoadModel(modelPath)
		assert.NoError(t, err)

		// Act
		ctx, cancel := context.WithCancel(context.Background())
		segmentChan, err := transcriptionEngine.ProcessAudio(ctx, mockAudioProcessor)
		assert.NoError(t, err)

		// Cancel context immediately
		cancel()

		// Assert
		// Channel should close due to context cancellation
		var segments []TranscriptionSegment
		for segment := range segmentChan {
			segments = append(segments, segment)
		}

		// Should receive limited or no segments due to cancellation
		assert.LessOrEqual(t, len(segments), 2, "should receive few or no segments due to cancellation")

		// Cleanup
		transcriptionEngine.Close()
		mockAudioProcessor.Close()
	})

	t.Run("should handle empty audio data", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		mockAudioProcessor := NewMockAudioProcessor([]byte{}) // Empty audio
		transcriptionEngine := NewTranscriptionEngine(logger)

		modelPath := "./models/ggml-base.en.bin"
		err := transcriptionEngine.LoadModel(modelPath)
		assert.NoError(t, err)

		// Act
		ctx := context.Background()
		segmentChan, err := transcriptionEngine.ProcessAudio(ctx, mockAudioProcessor)
		assert.NoError(t, err)

		// Assert
		var segments []TranscriptionSegment
		for segment := range segmentChan {
			segments = append(segments, segment)
		}

		// Should handle empty audio gracefully (may produce no segments or an empty segment)
		for _, segment := range segments {
			err := segment.Validate()
			assert.NoError(t, err, "any produced segments should be valid")
		}

		// Cleanup
		transcriptionEngine.Close()
		mockAudioProcessor.Close()
	})
}

func TestJSONOutput_Integration(t *testing.T) {
	t.Run("should match acceptance criteria JSON format exactly", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		var buffer bytes.Buffer
		jsonOutput := NewJSONOutput(&buffer, logger)

		// Create segment matching acceptance criteria example
		segment := TranscriptionSegment{
			Text:       "Hello world",
			StartMS:    1024,
			EndMS:      2048,
			Confidence: 0.92,
		}

		// Act
		err := jsonOutput.OutputSegment(segment)
		assert.NoError(t, err)

		// Assert
		output := strings.TrimSpace(buffer.String())
		expectedFormat := `{"text":"Hello world","start_ms":1024,"end_ms":2048,"confidence":0.92}`

		// Verify JSON structure matches acceptance criteria exactly
		assert.JSONEq(t, expectedFormat, output, "output should match acceptance criteria format")

		// Verify required fields are present
		var parsed map[string]interface{}
		err = json.Unmarshal([]byte(output), &parsed)
		assert.NoError(t, err)

		assert.Contains(t, parsed, "text", "JSON should contain 'text' field")
		assert.Contains(t, parsed, "start_ms", "JSON should contain 'start_ms' field")
		assert.Contains(t, parsed, "end_ms", "JSON should contain 'end_ms' field")
		assert.Contains(t, parsed, "confidence", "JSON should contain 'confidence' field")

		assert.Equal(t, "Hello world", parsed["text"])
		assert.Equal(t, float64(1024), parsed["start_ms"])
		assert.Equal(t, float64(2048), parsed["end_ms"])
		assert.InDelta(t, 0.92, parsed["confidence"], 0.001)
	})
}
