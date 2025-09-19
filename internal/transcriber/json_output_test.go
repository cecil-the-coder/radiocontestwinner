package transcriber

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestJSONOutput_OutputSegment(t *testing.T) {
	t.Run("should output valid JSON for single segment", func(t *testing.T) {
		// Arrange
		var buffer bytes.Buffer
		logger := zaptest.NewLogger(t)
		jsonOutput := NewJSONOutput(&buffer, logger)

		segment := TranscriptionSegment{
			Text:       "Hello world",
			StartMS:    1024,
			EndMS:      2048,
			Confidence: 0.92,
		}

		// Act
		err := jsonOutput.OutputSegment(segment)

		// Assert
		assert.NoError(t, err)

		output := buffer.String()
		assert.NotEmpty(t, output)

		// Verify it's valid JSON
		var parsedSegment TranscriptionSegment
		err = json.Unmarshal([]byte(strings.TrimSpace(output)), &parsedSegment)
		assert.NoError(t, err)
		assert.Equal(t, segment, parsedSegment)
	})

	t.Run("should output multiple segments as separate JSON lines", func(t *testing.T) {
		// Arrange
		var buffer bytes.Buffer
		logger := zaptest.NewLogger(t)
		jsonOutput := NewJSONOutput(&buffer, logger)

		segments := []TranscriptionSegment{
			{Text: "First segment", StartMS: 0, EndMS: 1000, Confidence: 0.95},
			{Text: "Second segment", StartMS: 1000, EndMS: 2000, Confidence: 0.88},
			{Text: "Third segment", StartMS: 2000, EndMS: 3000, Confidence: 0.91},
		}

		// Act
		for _, segment := range segments {
			err := jsonOutput.OutputSegment(segment)
			assert.NoError(t, err)
		}

		// Assert
		output := buffer.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Len(t, lines, 3)

		// Verify each line is valid JSON
		for i, line := range lines {
			var parsedSegment TranscriptionSegment
			err := json.Unmarshal([]byte(line), &parsedSegment)
			assert.NoError(t, err)
			assert.Equal(t, segments[i], parsedSegment)
		}
	})

	t.Run("should handle segments with special characters", func(t *testing.T) {
		// Arrange
		var buffer bytes.Buffer
		logger := zaptest.NewLogger(t)
		jsonOutput := NewJSONOutput(&buffer, logger)

		segment := TranscriptionSegment{
			Text:       "Text with \"quotes\" and \n newlines",
			StartMS:    500,
			EndMS:      1500,
			Confidence: 0.75,
		}

		// Act
		err := jsonOutput.OutputSegment(segment)

		// Assert
		assert.NoError(t, err)

		output := buffer.String()
		var parsedSegment TranscriptionSegment
		err = json.Unmarshal([]byte(strings.TrimSpace(output)), &parsedSegment)
		assert.NoError(t, err)
		assert.Equal(t, segment, parsedSegment)
	})

	t.Run("should validate segment before output", func(t *testing.T) {
		// Arrange
		var buffer bytes.Buffer
		logger := zaptest.NewLogger(t)
		jsonOutput := NewJSONOutput(&buffer, logger)

		invalidSegment := TranscriptionSegment{
			Text:       "", // Invalid: empty text
			StartMS:    1000,
			EndMS:      500, // Invalid: end before start
			Confidence: 1.5, // Invalid: confidence > 1.0
		}

		// Act
		err := jsonOutput.OutputSegment(invalidSegment)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid segment")
		assert.Empty(t, buffer.String()) // No output for invalid segment
	})
}

func TestJSONOutput_Close(t *testing.T) {
	t.Run("should close output without error", func(t *testing.T) {
		// Arrange
		var buffer bytes.Buffer
		logger := zaptest.NewLogger(t)
		jsonOutput := NewJSONOutput(&buffer, logger)

		// Act
		err := jsonOutput.Close()

		// Assert
		assert.NoError(t, err)
	})
}
