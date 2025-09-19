package buffer

import (
	"context"
	"strings"
	"time"

	"radiocontestwinner/internal/transcriber"
)

// ContextBuffer buffers and combines consecutive transcription segments
// into more complete sentences for better contest parsing
type ContextBuffer struct {
	bufferDurationMS int
	inputCh          <-chan transcriber.TranscriptionSegment
	outputCh         chan<- BufferedContext
	buffer           []transcriber.TranscriptionSegment
}

// NewContextBuffer creates a new ContextBuffer instance
func NewContextBuffer(bufferDurationMS int, inputCh <-chan transcriber.TranscriptionSegment, outputCh chan<- BufferedContext) *ContextBuffer {
	return &ContextBuffer{
		bufferDurationMS: bufferDurationMS,
		inputCh:          inputCh,
		outputCh:         outputCh,
		buffer:           make([]transcriber.TranscriptionSegment, 0),
	}
}

// Start begins processing segments with the configured buffer duration
func (cb *ContextBuffer) Start(ctx context.Context) error {
	go cb.processSegments(ctx)
	return nil
}

// processSegments handles the main buffering logic
func (cb *ContextBuffer) processSegments(ctx context.Context) {
	timer := time.NewTimer(time.Duration(cb.bufferDurationMS) * time.Millisecond)
	timer.Stop() // Stop initial timer until we have segments

	for {
		select {
		case <-ctx.Done():
			// Flush any remaining buffer before stopping
			if len(cb.buffer) > 0 {
				cb.flushBuffer()
			}
			return

		case segment, ok := <-cb.inputCh:
			if !ok {
				// Input channel closed, flush remaining buffer
				if len(cb.buffer) > 0 {
					cb.flushBuffer()
				}
				return
			}

			// Add segment to buffer
			cb.buffer = append(cb.buffer, segment)

			// Start timer if this is the first segment
			if len(cb.buffer) == 1 {
				timer.Reset(time.Duration(cb.bufferDurationMS) * time.Millisecond)
			}

		case <-timer.C:
			// Timer expired, flush buffer
			if len(cb.buffer) > 0 {
				cb.flushBuffer()
				timer.Stop()
			}
		}
	}
}

// flushBuffer combines buffered segments into a BufferedContext and sends it
func (cb *ContextBuffer) flushBuffer() {
	if len(cb.buffer) == 0 {
		return
	}

	// Combine text with proper spacing
	var textParts []string
	for _, segment := range cb.buffer {
		textParts = append(textParts, segment.Text)
	}
	combinedText := strings.Join(textParts, " ")

	// Use earliest StartMS and latest EndMS
	startMS := cb.buffer[0].StartMS
	endMS := cb.buffer[len(cb.buffer)-1].EndMS

	// Create BufferedContext
	bufferedContext := BufferedContext{
		Text:    combinedText,
		StartMS: startMS,
		EndMS:   endMS,
	}

	// Send to output channel
	select {
	case cb.outputCh <- bufferedContext:
		// Successfully sent
	default:
		// Output channel full, could log warning here
	}

	// Clear buffer
	cb.buffer = cb.buffer[:0]
}