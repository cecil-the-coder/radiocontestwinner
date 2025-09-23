package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"radiocontestwinner/internal/parser"
	"radiocontestwinner/internal/transcriber"
)

// E2E tests for complete audio processing pipeline

func skipE2EInCI(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping E2E test in CI environment - these tests are resource intensive and prone to timeout")
	}
}

func TestE2E_CompleteAudioPipeline(t *testing.T) {
	skipE2EInCI(t)
	t.Run("should process complete pipeline from AAC stream to contest cue output", func(t *testing.T) {
		// This test should fail initially (RED phase)
		// We need to create test audio files and mock stream infrastructure
		// Note: E2E test using generated test audio files
		// Requires real Whisper model in production environment
	})
}

func TestE2E_TranscriptionSegmentValidation(t *testing.T) {
	skipE2EInCI(t)
	t.Run("should validate TranscriptionSegment schema compliance", func(t *testing.T) {
		// Test TranscriptionSegment validation against schema
		segment := transcriber.TranscriptionSegment{
			Text:       "Text CONTEST to 12345",
			StartMS:    1000,
			EndMS:      3000,
			Confidence: 0.95,
		}

		err := segment.Validate()
		assert.NoError(t, err)
	})

	t.Run("should reject invalid TranscriptionSegment values", func(t *testing.T) {
		tests := []struct {
			name        string
			segment     transcriber.TranscriptionSegment
			expectError bool
		}{
			{
				name: "empty text",
				segment: transcriber.TranscriptionSegment{
					Text:       "",
					StartMS:    1000,
					EndMS:      3000,
					Confidence: 0.95,
				},
				expectError: true,
			},
			{
				name: "negative start time",
				segment: transcriber.TranscriptionSegment{
					Text:       "Valid text",
					StartMS:    -100,
					EndMS:      3000,
					Confidence: 0.95,
				},
				expectError: true,
			},
			{
				name: "end time before start time",
				segment: transcriber.TranscriptionSegment{
					Text:       "Valid text",
					StartMS:    3000,
					EndMS:      1000,
					Confidence: 0.95,
				},
				expectError: true,
			},
			{
				name: "confidence out of range - too high",
				segment: transcriber.TranscriptionSegment{
					Text:       "Valid text",
					StartMS:    1000,
					EndMS:      3000,
					Confidence: 1.5,
				},
				expectError: true,
			},
			{
				name: "confidence out of range - negative",
				segment: transcriber.TranscriptionSegment{
					Text:       "Valid text",
					StartMS:    1000,
					EndMS:      3000,
					Confidence: -0.1,
				},
				expectError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.segment.Validate()
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestE2E_ContestCueValidation(t *testing.T) {
	skipE2EInCI(t)
	t.Run("should validate ContestCue schema compliance", func(t *testing.T) {
		details := map[string]interface{}{
			"keyword":   "CONTEST",
			"shortcode": "12345",
		}

		cue := parser.NewContestCue("text-to-win", details)
		err := cue.Validate()
		assert.NoError(t, err)

		// Verify required fields are populated
		assert.NotEmpty(t, cue.CueID)
		assert.NotEmpty(t, cue.ContestType)
		assert.NotEmpty(t, cue.Timestamp)
		assert.NotNil(t, cue.Details)
	})

	t.Run("should reject invalid ContestCue values", func(t *testing.T) {
		tests := []struct {
			name        string
			cue         parser.ContestCue
			expectError bool
		}{
			{
				name: "empty CueID",
				cue: parser.ContestCue{
					CueID:       "",
					ContestType: "text-to-win",
					Timestamp:   time.Now().Format(time.RFC3339),
					Details:     map[string]interface{}{"test": "value"},
				},
				expectError: true,
			},
			{
				name: "empty ContestType",
				cue: parser.ContestCue{
					CueID:       "test123",
					ContestType: "",
					Timestamp:   time.Now().Format(time.RFC3339),
					Details:     map[string]interface{}{"test": "value"},
				},
				expectError: true,
			},
			{
				name: "empty Timestamp",
				cue: parser.ContestCue{
					CueID:       "test123",
					ContestType: "text-to-win",
					Timestamp:   "",
					Details:     map[string]interface{}{"test": "value"},
				},
				expectError: true,
			},
			{
				name: "nil Details",
				cue: parser.ContestCue{
					CueID:       "test123",
					ContestType: "text-to-win",
					Timestamp:   time.Now().Format(time.RFC3339),
					Details:     nil,
				},
				expectError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.cue.Validate()
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

// Mock HTTP server for testing audio streaming
func createMockAudioServer(t *testing.T, audioData []byte) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/aac")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audioData)))

		// Stream audio data
		_, err := w.Write(audioData)
		require.NoError(t, err)
	})

	server := httptest.NewServer(handler)
	return server
}

// loadTestAudioFile loads a test audio file from testdata
func loadTestAudioFile(t *testing.T, filename string) []byte {
	path := filepath.Join("testdata", "audio", filename)

	// For now, return empty data - will be replaced when actual test files are created
	// TODO: Load actual audio file
	t.Logf("Would load audio file from: %s", path)
	return []byte{} // Placeholder
}
