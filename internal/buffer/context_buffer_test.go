package buffer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"radiocontestwinner/internal/transcriber"
)

func TestContextBuffer_Creation(t *testing.T) {
	// Arrange
	bufferDurationMS := 2500
	inputCh := make(chan transcriber.TranscriptionSegment, 10)
	outputCh := make(chan BufferedContext, 10)

	// Act
	cb := NewContextBuffer(bufferDurationMS, inputCh, outputCh)

	// Assert
	assert.NotNil(t, cb)
	assert.Equal(t, bufferDurationMS, cb.bufferDurationMS)
}

func TestContextBuffer_Start_Stop(t *testing.T) {
	// Arrange
	bufferDurationMS := 100 // Short duration for testing
	inputCh := make(chan transcriber.TranscriptionSegment, 10)
	outputCh := make(chan BufferedContext, 10)
	cb := NewContextBuffer(bufferDurationMS, inputCh, outputCh)
	ctx, cancel := context.WithCancel(context.Background())

	// Act
	err := cb.Start(ctx)

	// Assert
	assert.NoError(t, err)

	// Stop
	cancel()
	time.Sleep(10 * time.Millisecond) // Allow goroutine to stop
}

func TestContextBuffer_ProcessSingleSegment(t *testing.T) {
	// Arrange
	bufferDurationMS := 100
	inputCh := make(chan transcriber.TranscriptionSegment, 10)
	outputCh := make(chan BufferedContext, 10)
	cb := NewContextBuffer(bufferDurationMS, inputCh, outputCh)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	segment := transcriber.TranscriptionSegment{
		Text:       "Hello world",
		StartMS:    1000,
		EndMS:      1500,
		Confidence: 0.9,
	}

	// Act
	err := cb.Start(ctx)
	assert.NoError(t, err)

	inputCh <- segment
	close(inputCh)

	// Wait for buffer to process
	select {
	case result := <-outputCh:
		// Assert
		assert.Equal(t, "Hello world", result.Text)
		assert.Equal(t, 1000, result.StartMS)
		assert.Equal(t, 1500, result.EndMS)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Expected output within timeout")
	}
}

func TestContextBuffer_ProcessMultipleSegments(t *testing.T) {
	// Arrange
	bufferDurationMS := 150
	inputCh := make(chan transcriber.TranscriptionSegment, 10)
	outputCh := make(chan BufferedContext, 10)
	cb := NewContextBuffer(bufferDurationMS, inputCh, outputCh)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	segment1 := transcriber.TranscriptionSegment{
		Text:       "Hello",
		StartMS:    1000,
		EndMS:      1200,
		Confidence: 0.9,
	}
	segment2 := transcriber.TranscriptionSegment{
		Text:       "world",
		StartMS:    1200,
		EndMS:      1400,
		Confidence: 0.8,
	}

	// Act
	err := cb.Start(ctx)
	assert.NoError(t, err)

	inputCh <- segment1
	inputCh <- segment2
	close(inputCh)

	// Wait for buffer to process
	select {
	case result := <-outputCh:
		// Assert
		assert.Equal(t, "Hello world", result.Text)
		assert.Equal(t, 1000, result.StartMS)
		assert.Equal(t, 1400, result.EndMS)
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Expected output within timeout")
	}
}

func TestContextBuffer_BufferTimeout(t *testing.T) {
	// Arrange
	bufferDurationMS := 50 // Very short timeout
	inputCh := make(chan transcriber.TranscriptionSegment, 10)
	outputCh := make(chan BufferedContext, 10)
	cb := NewContextBuffer(bufferDurationMS, inputCh, outputCh)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	segment := transcriber.TranscriptionSegment{
		Text:       "Test timeout",
		StartMS:    1000,
		EndMS:      1200,
		Confidence: 0.9,
	}

	// Act
	err := cb.Start(ctx)
	assert.NoError(t, err)

	inputCh <- segment
	// Don't close channel, let timeout trigger

	// Wait for buffer timeout to trigger output
	select {
	case result := <-outputCh:
		// Assert
		assert.Equal(t, "Test timeout", result.Text)
		assert.Equal(t, 1000, result.StartMS)
		assert.Equal(t, 1200, result.EndMS)
	case <-time.After(150 * time.Millisecond):
		t.Fatal("Expected output within timeout")
	}
}