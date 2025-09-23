package buffer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferedContext_Creation(t *testing.T) {
	// Arrange
	text := "Hello world test segment"
	startMS := 1000
	endMS := 2500

	// Act
	bc := BufferedContext{
		Text:    text,
		StartMS: startMS,
		EndMS:   endMS,
	}

	// Assert
	assert.Equal(t, text, bc.Text)
	assert.Equal(t, startMS, bc.StartMS)
	assert.Equal(t, endMS, bc.EndMS)
}

func TestBufferedContext_JSONMarshaling(t *testing.T) {
	// Arrange
	bc := BufferedContext{
		Text:    "Test segment",
		StartMS: 1000,
		EndMS:   2000,
	}

	// Act
	jsonData, err := json.Marshal(bc)

	// Assert
	assert.NoError(t, err)

	expected := `{"text":"Test segment","start_ms":1000,"end_ms":2000}`
	assert.JSONEq(t, expected, string(jsonData))
}

func TestBufferedContext_JSONUnmarshaling(t *testing.T) {
	// Arrange
	jsonStr := `{"text":"Test segment","start_ms":1000,"end_ms":2000}`

	// Act
	var bc BufferedContext
	err := json.Unmarshal([]byte(jsonStr), &bc)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Test segment", bc.Text)
	assert.Equal(t, 1000, bc.StartMS)
	assert.Equal(t, 2000, bc.EndMS)
}

func TestBufferedContext_Validate_ValidData(t *testing.T) {
	// Arrange
	bc := BufferedContext{
		Text:    "Valid test segment",
		StartMS: 1000,
		EndMS:   2000,
	}

	// Act
	err := bc.Validate()

	// Assert
	assert.NoError(t, err)
}

func TestBufferedContext_Validate_EmptyText(t *testing.T) {
	// Arrange
	bc := BufferedContext{
		Text:    "",
		StartMS: 1000,
		EndMS:   2000,
	}

	// Act
	err := bc.Validate()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "text cannot be empty")
}

func TestBufferedContext_Validate_NegativeStartMS(t *testing.T) {
	// Arrange
	bc := BufferedContext{
		Text:    "Test segment",
		StartMS: -100,
		EndMS:   2000,
	}

	// Act
	err := bc.Validate()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start_ms cannot be negative")
}

func TestBufferedContext_Validate_EndMSBeforeStartMS(t *testing.T) {
	// Arrange
	bc := BufferedContext{
		Text:    "Test segment",
		StartMS: 2000,
		EndMS:   1000,
	}

	// Act
	err := bc.Validate()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "end_ms must be greater than start_ms")
}

func TestBufferedContext_Validate_EqualStartAndEndMS(t *testing.T) {
	// Arrange
	bc := BufferedContext{
		Text:    "Test segment",
		StartMS: 1000,
		EndMS:   1000,
	}

	// Act
	err := bc.Validate()

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "end_ms must be greater than start_ms")
}
