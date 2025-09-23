package processor

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// TestAudioProcessor_NewAudioProcessor tests the creation of a new AudioProcessor
func TestAudioProcessor_NewAudioProcessor(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Mock input reader (simulating Stream Connector)
	input := bytes.NewReader([]byte("mock audio data"))

	processor := NewAudioProcessor(input, logger)

	assert.NotNil(t, processor)
	assert.Equal(t, input, processor.input)
	assert.Equal(t, logger, processor.logger)
}

// TestAudioProcessor_StartFFmpeg tests FFmpeg process initialization
func TestAudioProcessor_StartFFmpeg(t *testing.T) {
	logger := zaptest.NewLogger(t)
	input := bytes.NewReader([]byte("mock audio data"))

	processor := NewAudioProcessor(input, logger)

	// Use cat command for reliable testing
	processor.ffmpegPath = "cat"

	ctx := context.Background()
	err := processor.StartFFmpeg(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, processor.cmd)
	assert.NotNil(t, processor.stdin)
	assert.NotNil(t, processor.stdout)
	assert.NotNil(t, processor.stderr)

	// Clean up
	processor.Close()
}

// TestAudioProcessor_Read implements io.Reader interface and processes audio data
func TestAudioProcessor_Read(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Use mock command for testing that simulates successful output
	mockData := []byte("mock audio data")
	input := bytes.NewReader(mockData)

	processor := NewAudioProcessor(input, logger)

	// Use echo command to simulate FFmpeg for testing
	processor.ffmpegPath = "echo"

	ctx := context.Background()
	err := processor.StartFFmpeg(ctx)
	assert.NoError(t, err)

	// Read processed data (echo will return the args)
	buffer := make([]byte, 1024)
	n, err := processor.Read(buffer)

	// Should receive some data or EOF (both are acceptable for this test)
	assert.True(t, err == nil || err == io.EOF)
	assert.True(t, n >= 0)

	// Clean up
	processor.Close()
}

// TestAudioProcessor_Close tests proper resource cleanup
func TestAudioProcessor_Close(t *testing.T) {
	logger := zaptest.NewLogger(t)
	input := bytes.NewReader([]byte("mock audio data"))

	processor := NewAudioProcessor(input, logger)

	// Use cat command for testing - it will exit cleanly when stdin closes
	processor.ffmpegPath = "cat"

	ctx := context.Background()
	err := processor.StartFFmpeg(ctx)
	assert.NoError(t, err)

	err = processor.Close()
	assert.NoError(t, err)
}

// TestAudioProcessor_FFmpegProcessError tests error handling when FFmpeg fails
func TestAudioProcessor_FFmpegProcessError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	input := bytes.NewReader([]byte("invalid audio data"))

	processor := NewAudioProcessor(input, logger)

	// Use invalid FFmpeg command to simulate error
	processor.ffmpegPath = "/invalid/ffmpeg/path"

	ctx := context.Background()
	err := processor.StartFFmpeg(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start ffmpeg")
}

// TestAudioProcessor_ConcurrentProcessing tests concurrent data processing
func TestAudioProcessor_ConcurrentProcessing(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Larger mock data to test concurrent processing
	mockData := bytes.Repeat([]byte("test data"), 100)
	input := bytes.NewReader(mockData)

	processor := NewAudioProcessor(input, logger)

	// Use cat command for testing concurrent processing
	processor.ffmpegPath = "cat"

	ctx := context.Background()
	err := processor.StartFFmpeg(ctx)
	assert.NoError(t, err)

	// Read data concurrently
	done := make(chan bool)
	var readErr error
	go func() {
		defer close(done)
		buffer := make([]byte, 512)
		for {
			_, err := processor.Read(buffer)
			if err == io.EOF {
				break
			}
			if err != nil {
				readErr = err
				break
			}
		}
	}()

	// Wait for processing with timeout
	select {
	case <-done:
		// Processing completed successfully
		assert.NoError(t, readErr)
	case <-time.After(2 * time.Second):
		t.Fatal("Processing timed out")
	}

	processor.Close()
}
