package transcriber

import (
	"context"
	"fmt"
	"io"

	"go.uber.org/zap"
)

// WhisperModel interface defines the operations needed from Whisper.cpp model
type WhisperModel interface {
	LoadModel(modelPath string) error
	Transcribe(audioData []byte) ([]TranscriptionSegment, error)
	Close() error
}

// TranscriptionEngine manages the Whisper.cpp model and processes audio streams
type TranscriptionEngine struct {
	logger *zap.Logger
	model  WhisperModel
}

// NewTranscriptionEngine creates a new TranscriptionEngine instance
func NewTranscriptionEngine(logger *zap.Logger) *TranscriptionEngine {
	return &TranscriptionEngine{
		logger: logger,
		model:  NewWhisperCppModel(logger),
	}
}

// LoadModel loads the Whisper model from the specified path
func (te *TranscriptionEngine) LoadModel(modelPath string) error {
	te.logger.Info("loading Whisper model", zap.String("path", modelPath))

	if te.model == nil {
		return fmt.Errorf("whisper model not initialized")
	}

	if err := te.model.LoadModel(modelPath); err != nil {
		return fmt.Errorf("failed to load Whisper model from %s: %w", modelPath, err)
	}

	te.logger.Info("Whisper model loaded successfully", zap.String("path", modelPath))
	return nil
}

// ProcessAudio processes audio data from the reader and outputs transcription segments to a channel
func (te *TranscriptionEngine) ProcessAudio(ctx context.Context, audioReader io.Reader) (<-chan TranscriptionSegment, error) {
	te.logger.Info("starting audio processing for transcription")

	segmentChan := make(chan TranscriptionSegment)

	go func() {
		defer close(segmentChan)
		defer func() {
			if r := recover(); r != nil {
				te.logger.Error("panic recovered in audio processing", zap.Any("panic", r))
			}
		}()

		// Read all audio data (in a real implementation, this would be chunked)
		audioData, err := io.ReadAll(audioReader)
		if err != nil {
			te.logger.Error("failed to read audio data", zap.Error(err))
			return
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			te.logger.Debug("context cancelled, stopping audio processing")
			return
		default:
		}

		if te.model == nil {
			te.logger.Error("whisper model not initialized")
			return
		}

		// Transcribe audio
		segments, err := te.model.Transcribe(audioData)
		if err != nil {
			te.logger.Error("transcription failed", zap.Error(err))
			return
		}

		// Send segments to channel
		for _, segment := range segments {
			select {
			case <-ctx.Done():
				te.logger.Debug("context cancelled while sending segments")
				return
			case segmentChan <- segment:
				te.logger.Debug("sent transcription segment",
					zap.String("text", segment.Text),
					zap.Int("start_ms", segment.StartMS),
					zap.Int("end_ms", segment.EndMS),
					zap.Float32("confidence", segment.Confidence))
			}
		}

		te.logger.Info("audio processing completed", zap.Int("segments_count", len(segments)))
	}()

	return segmentChan, nil
}

// Close cleans up resources and closes the Whisper model
func (te *TranscriptionEngine) Close() error {
	te.logger.Info("closing transcription engine")

	if te.model != nil {
		if err := te.model.Close(); err != nil {
			te.logger.Error("failed to close Whisper model", zap.Error(err))
			return fmt.Errorf("failed to close Whisper model: %w", err)
		}
	}

	te.logger.Info("transcription engine closed successfully")
	return nil
}
