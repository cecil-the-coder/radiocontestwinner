package transcriber

import (
	"encoding/json"
	"fmt"
	"io"

	"go.uber.org/zap"
)

// JSONOutput handles outputting transcription segments as JSON to a writer
type JSONOutput struct {
	writer io.Writer
	logger *zap.Logger
}

// NewJSONOutput creates a new JSONOutput instance
func NewJSONOutput(writer io.Writer, logger *zap.Logger) *JSONOutput {
	return &JSONOutput{
		writer: writer,
		logger: logger,
	}
}

// OutputSegment writes a transcription segment as JSON to the output writer
func (jo *JSONOutput) OutputSegment(segment TranscriptionSegment) error {
	// Validate segment before output
	if err := segment.Validate(); err != nil {
		jo.logger.Error("invalid segment", zap.Error(err))
		return fmt.Errorf("invalid segment: %w", err)
	}

	// Marshal segment to JSON
	jsonBytes, err := json.Marshal(segment)
	if err != nil {
		jo.logger.Error("failed to marshal segment to JSON", zap.Error(err))
		return fmt.Errorf("failed to marshal segment to JSON: %w", err)
	}

	// Write JSON line to output
	if _, err := fmt.Fprintf(jo.writer, "%s\n", jsonBytes); err != nil {
		jo.logger.Error("failed to write JSON output", zap.Error(err))
		return fmt.Errorf("failed to write JSON output: %w", err)
	}

	jo.logger.Debug("output JSON segment",
		zap.String("text", segment.Text),
		zap.Int("start_ms", segment.StartMS),
		zap.Int("end_ms", segment.EndMS),
		zap.Float32("confidence", segment.Confidence))

	return nil
}

// Close closes the JSON output (no-op for basic writers, but required for interface consistency)
func (jo *JSONOutput) Close() error {
	jo.logger.Debug("closing JSON output")
	// For basic writers like buffers or stdout, no explicit close is needed
	// In production, this might be used for closing files or other resources
	return nil
}
