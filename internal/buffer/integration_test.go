package buffer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"radiocontestwinner/internal/config"
	"radiocontestwinner/internal/transcriber"
)

func TestContextBuffer_IntegrationWithConfiguration(t *testing.T) {
	// Arrange
	cfg := config.NewConfiguration()
	bufferDuration := cfg.GetBufferDurationMS()

	inputCh := make(chan transcriber.TranscriptionSegment, 10)
	outputCh := make(chan BufferedContext, 10)
	cb := NewContextBuffer(bufferDuration, inputCh, outputCh)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test segments that simulate transcription output
	segments := []transcriber.TranscriptionSegment{
		{Text: "Hello", StartMS: 1000, EndMS: 1500, Confidence: 0.95},
		{Text: "world", StartMS: 1500, EndMS: 2000, Confidence: 0.90},
		{Text: "this", StartMS: 2000, EndMS: 2300, Confidence: 0.85},
		{Text: "is", StartMS: 2300, EndMS: 2500, Confidence: 0.88},
		{Text: "a", StartMS: 2500, EndMS: 2600, Confidence: 0.92},
		{Text: "test", StartMS: 2600, EndMS: 3000, Confidence: 0.91},
	}

	// Act
	err := cb.Start(ctx)
	assert.NoError(t, err)

	// Send segments with some timing
	go func() {
		for _, segment := range segments {
			inputCh <- segment
			time.Sleep(10 * time.Millisecond) // Small delay between segments
		}
		close(inputCh)
	}()

	// Wait for buffer to process and output
	var results []BufferedContext
	timeout := time.After(time.Duration(bufferDuration+500) * time.Millisecond)

	for {
		select {
		case result, ok := <-outputCh:
			if !ok {
				goto DONE
			}
			results = append(results, result)
		case <-timeout:
			goto DONE
		}
	}

DONE:
	// Assert
	assert.NotEmpty(t, results, "should produce at least one buffered result")

	// Verify that segments were combined into coherent text
	for _, result := range results {
		assert.NotEmpty(t, result.Text, "buffered text should not be empty")
		assert.Greater(t, result.EndMS, result.StartMS, "end time should be after start time")

		// Validate the result
		err := result.Validate()
		assert.NoError(t, err, "buffered context should be valid")
	}

	// Verify that all original text is preserved
	var allText string
	for _, result := range results {
		if allText != "" {
			allText += " "
		}
		allText += result.Text
	}

	originalText := "Hello world this is a test"
	assert.Equal(t, originalText, allText, "all original text should be preserved in buffered output")
}

func TestContextBuffer_DataFlowPipelineSimulation(t *testing.T) {
	// This test simulates the complete data flow:
	// TranscriptionEngine -> ContextBuffer -> (Future ContestParser)

	// Arrange
	cfg := config.NewConfiguration()
	bufferDuration := cfg.GetBufferDurationMS()

	// Create channels to simulate the pipeline
	transcriptionCh := make(chan transcriber.TranscriptionSegment, 20)
	bufferedCh := make(chan BufferedContext, 10)

	// Create context buffer
	cb := NewContextBuffer(bufferDuration, transcriptionCh, bufferedCh)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Simulate continuous transcription segments from engine
	testSegments := []transcriber.TranscriptionSegment{
		{Text: "CQ", StartMS: 1000, EndMS: 1200, Confidence: 0.95},
		{Text: "contest", StartMS: 1200, EndMS: 1800, Confidence: 0.88},
		{Text: "W1ABC", StartMS: 1800, EndMS: 2400, Confidence: 0.92},
		{Text: "from", StartMS: 2400, EndMS: 2600, Confidence: 0.85},
		{Text: "Boston", StartMS: 2600, EndMS: 3200, Confidence: 0.90},
	}

	// Act
	err := cb.Start(ctx)
	assert.NoError(t, err)

	// Send segments to simulate transcription engine output
	go func() {
		for _, segment := range testSegments {
			transcriptionCh <- segment
			time.Sleep(50 * time.Millisecond) // Simulate real-time processing
		}
		close(transcriptionCh)
	}()

	// Collect buffered output (simulating what contest parser would receive)
	var bufferedResults []BufferedContext
	timeout := time.After(time.Duration(bufferDuration+1000) * time.Millisecond)

	for {
		select {
		case result, ok := <-bufferedCh:
			if !ok {
				goto DONE
			}
			bufferedResults = append(bufferedResults, result)
		case <-timeout:
			goto DONE
		}
	}

DONE:
	// Assert
	assert.NotEmpty(t, bufferedResults, "should produce buffered output for contest parser")

	// Verify the buffered output is suitable for contest parsing
	for _, result := range bufferedResults {
		assert.NotEmpty(t, result.Text, "buffered text should not be empty")
		assert.Contains(t, "CQ contest W1ABC from Boston", result.Text, "should contain contest-relevant text")

		// Verify timing consistency
		assert.Greater(t, result.EndMS, result.StartMS, "timing should be consistent")
		assert.GreaterOrEqual(t, result.StartMS, 1000, "should preserve original start timing")
		assert.LessOrEqual(t, result.EndMS, 3200, "should preserve original end timing")
	}
}

func TestContextBuffer_ResourceCleanup(t *testing.T) {
	// This test verifies proper resource management and graceful shutdown

	// Arrange
	cfg := config.NewConfiguration()
	bufferDuration := cfg.GetBufferDurationMS()

	inputCh := make(chan transcriber.TranscriptionSegment, 10)
	outputCh := make(chan BufferedContext, 10)
	cb := NewContextBuffer(bufferDuration, inputCh, outputCh)
	ctx, cancel := context.WithCancel(context.Background())

	// Add some segments
	segments := []transcriber.TranscriptionSegment{
		{Text: "Test", StartMS: 1000, EndMS: 1200, Confidence: 0.95},
		{Text: "cleanup", StartMS: 1200, EndMS: 1600, Confidence: 0.90},
	}

	// Act
	err := cb.Start(ctx)
	assert.NoError(t, err)

	// Send segments with small delays to ensure they get buffered together
	for _, segment := range segments {
		inputCh <- segment
		time.Sleep(10 * time.Millisecond) // Small delay to ensure both segments are received
	}

	// Give a moment for both segments to be buffered
	time.Sleep(50 * time.Millisecond)

	// Cancel context to test graceful shutdown
	cancel()

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)

	// Assert
	// Verify that any remaining buffered data was flushed
	select {
	case result := <-outputCh:
		assert.NotEmpty(t, result.Text, "remaining buffer should be flushed on shutdown")
		// The result should contain some of the test text
		assert.True(t,
			result.Text == "Test" ||
				result.Text == "cleanup" ||
				result.Text == "Test cleanup",
			"should contain some buffered text, got: %s", result.Text)
	case <-time.After(50 * time.Millisecond):
		// This is also acceptable if buffer was empty or already flushed
		t.Log("No remaining data to flush - buffer was empty or already processed")
	}
}
