package transcriber

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"

	"radiocontestwinner/internal/config"
	"radiocontestwinner/internal/performance"
)

// WhisperModel interface defines the operations needed from Whisper.cpp model
type WhisperModel interface {
	LoadModel(modelPath string) error
	Transcribe(audioData []byte) ([]TranscriptionSegment, error)
	Close() error
	GetGPUStatus() (bool, int)
}

// TranscriptionEngine manages the Whisper.cpp model and processes audio streams
type TranscriptionEngine struct {
	logger             *zap.Logger
	model              WhisperModel
	config             *config.Configuration
	performanceMonitor *performance.PerformanceMonitor
}

// NewTranscriptionEngine creates a new TranscriptionEngine instance
func NewTranscriptionEngine(logger *zap.Logger) *TranscriptionEngine {
	config := config.NewConfiguration()
	return &TranscriptionEngine{
		logger:             logger,
		model:              NewWhisperCppModelWithConfig(logger, config),
		config:             config,
		performanceMonitor: performance.NewPerformanceMonitor(logger),
	}
}

// NewTranscriptionEngineWithConfig creates a new TranscriptionEngine instance with custom configuration
func NewTranscriptionEngineWithConfig(logger *zap.Logger, config *config.Configuration) *TranscriptionEngine {
	return &TranscriptionEngine{
		logger:             logger,
		model:              NewWhisperCppModelWithConfig(logger, config),
		config:             config,
		performanceMonitor: performance.NewPerformanceMonitor(logger),
	}
}

// NewTranscriptionEngineWithBenchmark creates a TranscriptionEngine with benchmarking enabled
func NewTranscriptionEngineWithBenchmark(logger *zap.Logger, config *config.Configuration, benchmark bool) *TranscriptionEngine {
	return &TranscriptionEngine{
		logger:             logger,
		model:              NewWhisperCppModelWithConfig(logger, config),
		config:             config,
		performanceMonitor: performance.NewPerformanceMonitorWithBenchmark(logger, benchmark),
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

		if te.model == nil {
			te.logger.Error("whisper model not initialized")
			return
		}

		// Process audio in chunks for streaming transcription
		// Use configurable chunk duration for more responsive transcription
		chunkDurationSec := te.config.GetTranscriptionChunkDurationSec()
		overlapSec := te.config.GetTranscriptionOverlapSec()
		chunkSize := chunkDurationSec * 16000 * 2 // configurable seconds * 16kHz * 2 bytes per sample
		overlapSize := overlapSec * 16000 * 2     // overlap size in bytes
		stepSize := chunkSize - overlapSize       // step size without overlap

		buffer := make([]byte, chunkSize)
		overlapBuffer := make([]byte, overlapSize)

		chunkCount := 0
		totalSegments := 0

		firstChunk := true

		timeoutDuration := time.Duration(te.config.GetTranscriptionTimeoutSec()) * time.Second
		lastAudioTime := time.Now()
		waitingForAudio := false

		for {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				te.logger.Debug("context cancelled, stopping audio processing")
				return
			default:
			}

			// Check for timeout - stop processing if no audio for extended period
			if time.Since(lastAudioTime) > timeoutDuration {
				te.logger.Info("transcription timeout reached - stopping processing",
					zap.Duration("timeout_duration", timeoutDuration),
					zap.Int("total_chunks", chunkCount),
					zap.Int("total_segments", totalSegments))
				return
			}

			var readSize int
			var readBuffer []byte

			if firstChunk {
				// First chunk: read full chunk size
				readSize = chunkSize
				readBuffer = buffer
				firstChunk = false
			} else {
				// Subsequent chunks: copy overlap from previous chunk, then read new data
				copy(buffer[:overlapSize], overlapBuffer)
				readSize = stepSize
				readBuffer = buffer[overlapSize:]
			}

			// Read audio data with timeout
			bytesRead, err := io.ReadFull(audioReader, readBuffer[:readSize])
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					if bytesRead > 0 {
						// Process the final partial chunk
						totalBytes := overlapSize + bytesRead
						if !firstChunk {
							segments := te.processAudioChunk(buffer[:totalBytes], chunkCount, segmentChan, ctx)
							totalSegments += segments
							chunkCount++
						}
						lastAudioTime = time.Now() // Reset timeout on partial data
					}

					// Log the end of stream but continue waiting for new audio
					if !waitingForAudio {
						te.logger.Info("reached end of audio stream - entering keep-alive mode",
							zap.Int("total_chunks", chunkCount),
							zap.Int("total_segments", totalSegments),
							zap.Duration("timeout_in", timeoutDuration-time.Since(lastAudioTime)))
						waitingForAudio = true
					}

					// Wait a short time before trying to read again
					select {
					case <-ctx.Done():
						te.logger.Debug("context cancelled while waiting for new audio")
						return
					case <-time.After(100 * time.Millisecond):
						// Continue to next iteration to try reading again
						continue
					}
				}
				te.logger.Error("failed to read audio chunk", zap.Error(err))
				return
			}

			// Successfully read audio data - reset timeout timer
			lastAudioTime = time.Now()

			// Log recovery if we were waiting for audio
			if waitingForAudio {
				te.logger.Info("audio stream recovered - resuming normal processing",
					zap.Int("total_chunks", chunkCount),
					zap.Int("total_segments", totalSegments))
				waitingForAudio = false
			}

			chunkCount++

			// Save overlap for next iteration
			copy(overlapBuffer, buffer[chunkSize-overlapSize:chunkSize])

			te.logger.Debug("processing audio chunk",
				zap.Int("chunk_number", chunkCount),
				zap.Int("bytes_read", bytesRead),
				zap.Int("chunk_duration_sec", chunkDurationSec))

			// Process this chunk
			segments := te.processAudioChunk(buffer, chunkCount, segmentChan, ctx)
			totalSegments += segments

			if chunkCount%10 == 0 {
				te.logger.Info("audio processing progress",
					zap.Int("chunks_processed", chunkCount),
					zap.Int("total_segments", totalSegments))
			}
		}
	}()

	return segmentChan, nil
}

// processAudioChunk processes a single chunk of audio data through Whisper
func (te *TranscriptionEngine) processAudioChunk(audioData []byte, chunkNumber int, segmentChan chan<- TranscriptionSegment, ctx context.Context) int {
	// Get GPU status for performance monitoring
	useGPU, deviceID := te.model.GetGPUStatus()

	// Start performance monitoring
	timer := te.performanceMonitor.StartTranscription(int64(len(audioData)), useGPU, deviceID)

	// Transcribe audio chunk
	segments, err := te.model.Transcribe(audioData)

	// End performance monitoring
	te.performanceMonitor.EndTranscription(timer)

	if err != nil {
		te.logger.Error("transcription failed for chunk",
			zap.Error(err),
			zap.Int("chunk_number", chunkNumber))
		return 0
	}

	te.logger.Debug("transcribed audio chunk",
		zap.Int("chunk_number", chunkNumber),
		zap.Int("segments_found", len(segments)))

	// Send segments to channel
	sentCount := 0
	for _, segment := range segments {
		select {
		case <-ctx.Done():
			te.logger.Debug("context cancelled while sending segments")
			return sentCount
		case segmentChan <- segment:
			sentCount++

			// Log debug output if debug mode is enabled
			if te.config.GetDebugMode() {
				te.logger.Debug("Transcription segment",
					zap.String("component", "transcriber"),
					zap.Any("data", map[string]interface{}{
						"text":       segment.Text,
						"start_ms":   segment.StartMS,
						"end_ms":     segment.EndMS,
						"confidence": segment.Confidence,
					}),
					zap.Int("chunk_number", chunkNumber))

				te.logger.Info("🎙️ TRANSCRIPTION COMPLETED",
					zap.String("text", segment.Text),
					zap.Int("start_ms", segment.StartMS),
					zap.Int("end_ms", segment.EndMS),
					zap.Float32("confidence", segment.Confidence),
					zap.Int("chunk_number", chunkNumber))
			}
		}
	}

	return sentCount
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

// GetPerformanceMetrics returns current performance metrics
func (te *TranscriptionEngine) GetPerformanceMetrics() performance.PerformanceMetrics {
	return te.performanceMonitor.GetMetrics()
}

// GetPerformanceSummary returns a formatted performance summary
func (te *TranscriptionEngine) GetPerformanceSummary() string {
	return te.performanceMonitor.GetPerformanceSummary()
}

// CompareGPUvsCPU returns GPU vs CPU performance comparison
func (te *TranscriptionEngine) CompareGPUvsCPU() string {
	return te.performanceMonitor.CompareGPUvsCPU()
}

// ResetPerformanceMetrics clears all accumulated performance metrics
func (te *TranscriptionEngine) ResetPerformanceMetrics() {
	te.performanceMonitor.ResetMetrics()
}

// EnableBenchmarking enables or disables detailed performance logging
func (te *TranscriptionEngine) EnableBenchmarking(enabled bool) {
	te.performanceMonitor.BenchmarkMode(enabled)
}

// LogCurrentPerformanceMetrics logs the current performance metrics
func (te *TranscriptionEngine) LogCurrentPerformanceMetrics() {
	te.performanceMonitor.LogCurrentMetrics()
}
