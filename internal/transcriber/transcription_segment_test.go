package transcriber

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranscriptionSegment_JSONMarshaling(t *testing.T) {
	// Arrange
	segment := TranscriptionSegment{
		Text:       "Hello world",
		StartMS:    1024,
		EndMS:      2048,
		Confidence: 0.92,
	}
	expected := `{"text":"Hello world","start_ms":1024,"end_ms":2048,"confidence":0.92}`

	// Act
	jsonBytes, err := json.Marshal(segment)

	// Assert
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(jsonBytes))
}

func TestTranscriptionSegment_JSONUnmarshaling(t *testing.T) {
	// Arrange
	jsonData := `{"text":"Hello world","start_ms":1024,"end_ms":2048,"confidence":0.92}`
	expected := TranscriptionSegment{
		Text:       "Hello world",
		StartMS:    1024,
		EndMS:      2048,
		Confidence: 0.92,
	}

	// Act
	var segment TranscriptionSegment
	err := json.Unmarshal([]byte(jsonData), &segment)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expected, segment)
}

func TestTranscriptionSegment_Validation(t *testing.T) {
	tests := []struct {
		name          string
		segment       TranscriptionSegment
		expectedValid bool
		expectedError string
	}{
		{
			name: "valid segment",
			segment: TranscriptionSegment{
				Text:       "Valid text",
				StartMS:    100,
				EndMS:      200,
				Confidence: 0.85,
			},
			expectedValid: true,
		},
		{
			name: "empty text",
			segment: TranscriptionSegment{
				Text:       "",
				StartMS:    100,
				EndMS:      200,
				Confidence: 0.85,
			},
			expectedValid: false,
			expectedError: "text cannot be empty",
		},
		{
			name: "negative start time",
			segment: TranscriptionSegment{
				Text:       "Test",
				StartMS:    -1,
				EndMS:      200,
				Confidence: 0.85,
			},
			expectedValid: false,
			expectedError: "start_ms cannot be negative",
		},
		{
			name: "end time before start time",
			segment: TranscriptionSegment{
				Text:       "Test",
				StartMS:    200,
				EndMS:      100,
				Confidence: 0.85,
			},
			expectedValid: false,
			expectedError: "end_ms must be greater than start_ms",
		},
		{
			name: "confidence out of range - negative",
			segment: TranscriptionSegment{
				Text:       "Test",
				StartMS:    100,
				EndMS:      200,
				Confidence: -0.1,
			},
			expectedValid: false,
			expectedError: "confidence must be between 0.0 and 1.0",
		},
		{
			name: "confidence out of range - too high",
			segment: TranscriptionSegment{
				Text:       "Test",
				StartMS:    100,
				EndMS:      200,
				Confidence: 1.1,
			},
			expectedValid: false,
			expectedError: "confidence must be between 0.0 and 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := tt.segment.Validate()

			// Assert
			if tt.expectedValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}
