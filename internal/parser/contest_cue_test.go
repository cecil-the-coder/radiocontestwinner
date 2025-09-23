package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContestCue_Validate(t *testing.T) {
	t.Run("should validate a properly constructed ContestCue", func(t *testing.T) {
		// Arrange
		cue := &ContestCue{
			CueID:       "test-cue-123",
			ContestType: "POTA",
			Timestamp:   "2023-01-01T12:00:00Z",
			Details: map[string]interface{}{
				"keyword": "POTA",
				"number":  "1234",
			},
		}

		// Act
		err := cue.Validate()

		// Assert
		assert.NoError(t, err, "should not return error for valid ContestCue")
	})

	t.Run("should return error for empty CueID", func(t *testing.T) {
		// Arrange
		cue := &ContestCue{
			CueID:       "",
			ContestType: "POTA",
			Timestamp:   "2023-01-01T12:00:00Z",
			Details: map[string]interface{}{
				"keyword": "POTA",
				"number":  "1234",
			},
		}

		// Act
		err := cue.Validate()

		// Assert
		assert.Error(t, err, "should return error for empty CueID")
		assert.Contains(t, err.Error(), "CueID cannot be empty")
	})

	t.Run("should return error for empty ContestType", func(t *testing.T) {
		// Arrange
		cue := &ContestCue{
			CueID:       "test-cue-123",
			ContestType: "",
			Timestamp:   "2023-01-01T12:00:00Z",
			Details: map[string]interface{}{
				"keyword": "POTA",
				"number":  "1234",
			},
		}

		// Act
		err := cue.Validate()

		// Assert
		assert.Error(t, err, "should return error for empty ContestType")
		assert.Contains(t, err.Error(), "ContestType cannot be empty")
	})

	t.Run("should return error for empty Timestamp", func(t *testing.T) {
		// Arrange
		cue := &ContestCue{
			CueID:       "test-cue-123",
			ContestType: "POTA",
			Timestamp:   "",
			Details: map[string]interface{}{
				"keyword": "POTA",
				"number":  "1234",
			},
		}

		// Act
		err := cue.Validate()

		// Assert
		assert.Error(t, err, "should return error for empty Timestamp")
		assert.Contains(t, err.Error(), "Timestamp cannot be empty")
	})

	t.Run("should return error for nil Details", func(t *testing.T) {
		// Arrange
		cue := &ContestCue{
			CueID:       "test-cue-123",
			ContestType: "POTA",
			Timestamp:   "2023-01-01T12:00:00Z",
			Details:     nil,
		}

		// Act
		err := cue.Validate()

		// Assert
		assert.Error(t, err, "should return error for nil Details")
		assert.Contains(t, err.Error(), "Details cannot be nil")
	})
}

func TestNewContestCue(t *testing.T) {
	t.Run("should create ContestCue with generated CueID and current timestamp", func(t *testing.T) {
		// Arrange
		contestType := "POTA"
		details := map[string]interface{}{
			"keyword": "POTA",
			"number":  "1234",
		}

		// Act
		cue := NewContestCue(contestType, details)

		// Assert
		assert.NotEmpty(t, cue.CueID, "should generate CueID")
		assert.Equal(t, contestType, cue.ContestType, "should set ContestType")
		assert.NotEmpty(t, cue.Timestamp, "should generate Timestamp")
		assert.Equal(t, details, cue.Details, "should set Details")

		// Verify timestamp is valid RFC3339 format
		_, err := time.Parse(time.RFC3339, cue.Timestamp)
		assert.NoError(t, err, "timestamp should be valid RFC3339 format")

		// Verify CueID is unique for multiple calls
		cue2 := NewContestCue(contestType, details)
		assert.NotEqual(t, cue.CueID, cue2.CueID, "should generate unique CueIDs")
	})
}

func TestGenerateCueID(t *testing.T) {
	t.Run("should generate unique CueID based on input parameters", func(t *testing.T) {
		// Arrange
		contestType := "POTA"
		details := map[string]interface{}{
			"keyword": "POTA",
			"number":  "1234",
		}
		timestamp := "2023-01-01T12:00:00Z"

		// Act
		cueID1 := GenerateCueID(contestType, details, timestamp)
		cueID2 := GenerateCueID(contestType, details, timestamp)

		// Assert
		assert.NotEmpty(t, cueID1, "should generate non-empty CueID")
		assert.Equal(t, cueID1, cueID2, "should generate same CueID for same inputs")

		// Test with different inputs
		cueID3 := GenerateCueID("WWFF", details, timestamp)
		assert.NotEqual(t, cueID1, cueID3, "should generate different CueID for different contestType")

		cueID4 := GenerateCueID(contestType, details, "2023-01-01T13:00:00Z")
		assert.NotEqual(t, cueID1, cueID4, "should generate different CueID for different timestamp")
	})
}
