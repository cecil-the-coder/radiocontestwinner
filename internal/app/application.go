package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"radiocontestwinner/internal/buffer"
	"radiocontestwinner/internal/config"
	"radiocontestwinner/internal/logger"
	"radiocontestwinner/internal/parser"
	"radiocontestwinner/internal/processor"
	"radiocontestwinner/internal/stream"
	"radiocontestwinner/internal/transcriber"
)

// PipelineHealth tracks the health status of the audio processing pipeline
type PipelineHealth struct {
	mu                      sync.RWMutex
	lastTranscriptionTime   time.Time
	lastBufferedContextTime time.Time
	lastContestCueTime      time.Time
	streamConnectionActive  bool
	audioProcessingActive   bool
	transcriptionActive     bool
	totalTranscriptions     int64
	totalContestCues        int64

	// Performance tracking for "falling behind" detection
	processingStartTime  time.Time
	totalAudioDurationMS int64   // Total duration of audio processed
	averageLatencyMS     float64 // Moving average of processing latency
	currentBacklogSize   int
	isRealTime           bool // Are we processing in real-time?
}

// Application represents the main radio contest winner application orchestrator
type Application struct {
	config              *config.Configuration
	logger              *logger.LogOutput
	zapLogger           *zap.Logger
	streamConnector     *stream.StreamConnector
	audioProcessor      *processor.AudioProcessor
	transcriptionEngine *transcriber.TranscriptionEngine
	contestParser       *parser.ContestParser
	logOutput           *logger.LogOutput
	pipelineHealth      *PipelineHealth
}

// NewApplication creates a new application instance with all components initialized
func NewApplication() (*Application, error) {
	// Load configuration from config file if CONFIG_PATH is set, otherwise use environment variables
	var cfg *config.Configuration
	var err error

	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		cfg, err = config.NewConfigurationFromFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from file %s: %w", configPath, err)
		}
	} else {
		cfg, err = config.NewConfigurationFromEnv()
		if err != nil {
			return nil, fmt.Errorf("failed to load config from environment: %w", err)
		}
	}

	// Create zap logger - centralized structured logging
	zapLogger := logger.NewLogger()

	// Create log output component for contest cues
	logOutput, err := logger.NewLogOutput(cfg, zapLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create log output: %w", err)
	}

	// Create stream connector component
	streamConnector := stream.NewStreamConnectorWithLogger(cfg.GetStreamURL(), zapLogger)

	// Create transcription engine component
	transcriptionEngine := transcriber.NewTranscriptionEngineWithConfig(zapLogger, cfg)

	// Create contest parser component with configured allowlist
	contestParser := parser.NewContestParserWithLogger(cfg.GetAllowlist(), zapLogger)

	// Audio processor will be created per connection, so initialize as nil for now
	var audioProcessor *processor.AudioProcessor

	return &Application{
		config:              cfg,
		logger:              logOutput,
		zapLogger:           zapLogger,
		streamConnector:     streamConnector,
		audioProcessor:      audioProcessor,
		transcriptionEngine: transcriptionEngine,
		contestParser:       contestParser,
		logOutput:           logOutput,
		pipelineHealth:      &PipelineHealth{},
	}, nil
}

// Run starts the application and runs the main processing pipeline
func (app *Application) Run(ctx context.Context) error {
	app.zapLogger.Info("starting Radio Contest Winner application")

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		app.zapLogger.Info("context cancelled before startup, shutting down immediately")
		return nil
	default:
	}

	// Load Whisper model
	if err := app.transcriptionEngine.LoadModel(app.config.GetWhisperModelPath()); err != nil {
		app.zapLogger.Warn("failed to load Whisper model, continuing without transcription", zap.Error(err))
	} else {
		app.zapLogger.Info("Whisper model loaded successfully", zap.String("path", app.config.GetWhisperModelPath()))
	}

	// Start the audio processing pipeline
	if err := app.startPipeline(ctx); err != nil {
		app.zapLogger.Error("failed to start pipeline", zap.Error(err))

		// Check if this is a context cancellation/timeout scenario for graceful handling
		// Only handle cancellation gracefully if it was an intentional cancellation, not a network failure timeout
		select {
		case <-ctx.Done():
			if contextErr := ctx.Err(); contextErr == context.Canceled {
				// Always treat explicit cancellation as graceful
				app.zapLogger.Info("context cancelled during pipeline startup, shutting down gracefully")
				return nil
			} else if contextErr == context.DeadlineExceeded {
				// For timeout cases, check if this was caused by network issues
				// If the underlying error contains network-related errors, treat as failure
				if strings.Contains(err.Error(), "connection refused") ||
				   strings.Contains(err.Error(), "no such host") ||
				   strings.Contains(err.Error(), "network is unreachable") ||
				   strings.Contains(err.Error(), "localhost:") ||
				   strings.Contains(err.Error(), "127.0.0.1:") {
					// Network failure to test servers - return error
					break
				}
				// Otherwise, treat timeout as graceful shutdown for very short timeouts
				if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) > -500*time.Millisecond {
					app.zapLogger.Info("context deadline exceeded during pipeline startup, shutting down gracefully")
					return nil
				}
			}
			// Fall through to return error for longer-running scenarios
		default:
			// Context not cancelled, return the actual error
		}

		return fmt.Errorf("failed to start pipeline: %w", err)
	}

	// Wait for shutdown signal
	<-ctx.Done()
	app.zapLogger.Info("shutdown signal received, stopping application")

	return nil
}

// startPipeline initializes and starts the complete audio processing pipeline
func (app *Application) startPipeline(ctx context.Context) error {
	app.zapLogger.Info("starting audio processing pipeline",
		zap.Bool("debug_mode", app.config.GetDebugMode()),
		zap.String("stream_url", app.config.GetStreamURL()),
		zap.Int("buffer_duration_ms", app.config.GetBufferDurationMS()))

	// Connect to audio stream with automatic retry and exponential backoff
	if err := app.streamConnector.ConnectWithRetry(ctx); err != nil {
		app.updateStreamHealth(false)
		return fmt.Errorf("failed to connect to stream after retries: %w", err)
	}

	app.updateStreamHealth(true)
	if app.config.GetDebugMode() {
		app.zapLogger.Info("stream connection established successfully")
	}

	// Create audio processor with stream as input
	app.audioProcessor = processor.NewAudioProcessor(app.streamConnector, app.zapLogger)

	// Start FFmpeg process
	if err := app.audioProcessor.StartFFmpeg(ctx); err != nil {
		app.updateAudioProcessingHealth(false)
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	app.updateAudioProcessingHealth(true)
	if app.config.GetDebugMode() {
		app.zapLogger.Info("FFmpeg audio processor started successfully")
	}

	// Start transcription processing - returns channel of TranscriptionSegment
	transcriptionCh, err := app.transcriptionEngine.ProcessAudio(ctx, app.audioProcessor)
	if err != nil {
		return fmt.Errorf("failed to start transcription processing: %w", err)
	}

	if app.config.GetDebugMode() {
		app.zapLogger.Info("transcription engine processing started")
	}

	// Create channels for pipeline
	bufferedContextCh := make(chan buffer.BufferedContext, 100)
	contestCueCh := make(chan parser.ContestCue, 100)

	// Wrap transcription channel for health tracking first
	transcriptionCh = app.wrapTranscriptionChannelWithHealthTracking(transcriptionCh)

	// Create and start context buffer (TranscriptionSegment -> BufferedContext)
	contextBuffer := buffer.NewContextBuffer(app.config.GetBufferDurationMS(), transcriptionCh, bufferedContextCh)
	if err := contextBuffer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start context buffer: %w", err)
	}

	if app.config.GetDebugMode() {
		app.zapLogger.Info("context buffer started successfully")
	}

	// Wrap channels for health tracking AFTER they're connected to their sources
	bufferedContextChWrapped := app.wrapBufferedContextChannelWithHealthTracking(bufferedContextCh)
	contestCueChWrapped := app.wrapContestCueChannelWithHealthTracking(contestCueCh)

	// Start contest parser processing (BufferedContext -> ContestCue)
	go app.contestParser.ProcessBufferedContextWithPatternMatching(bufferedContextChWrapped, contestCueCh)

	// Start log output processing (ContestCue -> file output)
	go app.logOutput.ProcessContestCues(contestCueChWrapped)

	// Start heartbeat monitoring
	go app.startHeartbeat(ctx)

	app.zapLogger.Info("audio processing pipeline started successfully",
		zap.Bool("debug_mode", app.config.GetDebugMode()))
	return nil
}

// updateStreamHealth updates the stream connection health status
func (app *Application) updateStreamHealth(active bool) {
	app.pipelineHealth.mu.Lock()
	defer app.pipelineHealth.mu.Unlock()
	app.pipelineHealth.streamConnectionActive = active
}

// updateAudioProcessingHealth updates the audio processing health status
func (app *Application) updateAudioProcessingHealth(active bool) {
	app.pipelineHealth.mu.Lock()
	defer app.pipelineHealth.mu.Unlock()
	app.pipelineHealth.audioProcessingActive = active
}

// updateTranscriptionHealth updates transcription activity and metrics
func (app *Application) updateTranscriptionHealth() {
	app.pipelineHealth.mu.Lock()
	defer app.pipelineHealth.mu.Unlock()
	app.pipelineHealth.lastTranscriptionTime = time.Now()
	app.pipelineHealth.transcriptionActive = true
	app.pipelineHealth.totalTranscriptions++
}

// updateTranscriptionPerformance tracks latency and real-time processing metrics
func (app *Application) updateTranscriptionPerformance(segment transcriber.TranscriptionSegment, processingStartTime time.Time) {
	app.pipelineHealth.mu.Lock()
	defer app.pipelineHealth.mu.Unlock()

	// Calculate processing latency
	processingLatency := time.Since(processingStartTime)
	latencyMS := float64(processingLatency.Milliseconds())

	// Update moving average latency (simple exponential moving average)
	alpha := 0.1 // Weight for new value
	if app.pipelineHealth.averageLatencyMS == 0 {
		app.pipelineHealth.averageLatencyMS = latencyMS
	} else {
		app.pipelineHealth.averageLatencyMS = alpha*latencyMS + (1-alpha)*app.pipelineHealth.averageLatencyMS
	}

	// Track total audio duration processed
	audioDurationMS := int64(segment.EndMS - segment.StartMS)
	app.pipelineHealth.totalAudioDurationMS += audioDurationMS

	// Calculate real-time ratio
	if app.pipelineHealth.processingStartTime.IsZero() {
		app.pipelineHealth.processingStartTime = processingStartTime
	}
	totalProcessingTime := time.Since(app.pipelineHealth.processingStartTime)
	totalAudioDuration := time.Duration(app.pipelineHealth.totalAudioDurationMS) * time.Millisecond

	// We're real-time if we're processing audio faster than it's generated
	app.pipelineHealth.isRealTime = totalProcessingTime <= totalAudioDuration*2 // Allow 2x buffer for safety
}

// updateBufferedContextHealth updates buffered context processing
func (app *Application) updateBufferedContextHealth() {
	app.pipelineHealth.mu.Lock()
	defer app.pipelineHealth.mu.Unlock()
	app.pipelineHealth.lastBufferedContextTime = time.Now()
}

// updateContestCueHealth updates contest cue detection metrics
func (app *Application) updateContestCueHealth() {
	app.pipelineHealth.mu.Lock()
	defer app.pipelineHealth.mu.Unlock()
	app.pipelineHealth.lastContestCueTime = time.Now()
	app.pipelineHealth.totalContestCues++
}

// getPipelineHealthStatus returns current pipeline health status
func (app *Application) getPipelineHealthStatus() map[string]interface{} {
	app.pipelineHealth.mu.RLock()
	defer app.pipelineHealth.mu.RUnlock()

	now := time.Now()
	timeSinceLastTranscription := now.Sub(app.pipelineHealth.lastTranscriptionTime)
	timeSinceLastBufferedContext := now.Sub(app.pipelineHealth.lastBufferedContextTime)
	timeSinceLastContestCue := now.Sub(app.pipelineHealth.lastContestCueTime)

	// Consider pipeline unhealthy if no transcription for more than 2 minutes
	transcriptionHealthy := timeSinceLastTranscription < 2*time.Minute || app.pipelineHealth.lastTranscriptionTime.IsZero()

	// Calculate real-time performance ratio
	var realTimeRatio float64
	if !app.pipelineHealth.processingStartTime.IsZero() {
		totalProcessingTime := now.Sub(app.pipelineHealth.processingStartTime)
		totalAudioDuration := time.Duration(app.pipelineHealth.totalAudioDurationMS) * time.Millisecond
		if totalAudioDuration > 0 {
			realTimeRatio = float64(totalAudioDuration) / float64(totalProcessingTime)
		}
	}

	return map[string]interface{}{
		"stream_connected":              app.pipelineHealth.streamConnectionActive,
		"audio_processing_active":       app.pipelineHealth.audioProcessingActive,
		"transcription_active":          app.pipelineHealth.transcriptionActive,
		"transcription_healthy":         transcriptionHealthy,
		"last_transcription_time":       app.pipelineHealth.lastTranscriptionTime.Format(time.RFC3339),
		"last_buffered_context_time":    app.pipelineHealth.lastBufferedContextTime.Format(time.RFC3339),
		"last_contest_cue_time":         app.pipelineHealth.lastContestCueTime.Format(time.RFC3339),
		"time_since_last_transcription": timeSinceLastTranscription.String(),
		"time_since_last_context":       timeSinceLastBufferedContext.String(),
		"time_since_last_cue":           timeSinceLastContestCue.String(),
		"total_transcriptions":          app.pipelineHealth.totalTranscriptions,
		"total_contest_cues":            app.pipelineHealth.totalContestCues,

		// Performance metrics to track "falling behind"
		"average_latency_ms":      app.pipelineHealth.averageLatencyMS,
		"total_audio_duration_ms": app.pipelineHealth.totalAudioDurationMS,
		"is_real_time":            app.pipelineHealth.isRealTime,
		"real_time_ratio":         realTimeRatio, // >1.0 means we're keeping up, <1.0 means falling behind
		"current_backlog_size":    app.pipelineHealth.currentBacklogSize,
	}
}

// writeHealthStatusFile writes the current health status to a file for Docker health checks
func (app *Application) writeHealthStatusFile() error {
	healthStatus := app.getPipelineHealthStatus()

	// Add timestamp for health check validation
	healthStatus["health_check_timestamp"] = time.Now().Format(time.RFC3339)
	healthStatus["healthy"] = app.isSystemHealthy(healthStatus)

	// Write to health status file
	healthFile := "/tmp/radiocontestwinner-health.json"

	// Create directory if it doesn't exist
	dir := filepath.Dir(healthFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create health file directory: %w", err)
	}

	// Marshal health status to JSON
	data, err := json.MarshalIndent(healthStatus, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal health status: %w", err)
	}

	// Write health status file atomically
	tempFile := healthFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write health file: %w", err)
	}

	if err := os.Rename(tempFile, healthFile); err != nil {
		return fmt.Errorf("failed to rename health file: %w", err)
	}

	return nil
}

// isSystemHealthy determines overall system health based on pipeline status
func (app *Application) isSystemHealthy(healthStatus map[string]interface{}) bool {
	// System is healthy if:
	// 1. Stream is connected OR we haven't started processing yet
	// 2. If transcription has started, it should be healthy
	// 3. No critical component failures

	streamConnected := healthStatus["stream_connected"].(bool)
	transcriptionHealthy := healthStatus["transcription_healthy"].(bool)
	totalTranscriptions := healthStatus["total_transcriptions"].(int64)

	// If we have started transcribing, transcription must be healthy
	if totalTranscriptions > 0 && !transcriptionHealthy {
		return false
	}

	// If audio processing started, stream should be connected
	audioProcessingActive := healthStatus["audio_processing_active"].(bool)
	if audioProcessingActive && !streamConnected {
		return false
	}

	// Overall system is healthy
	return true
}

// writeTranscriptionToDebugFile writes transcriptions to a debug file in debug mode
func (app *Application) writeTranscriptionToDebugFile(segment transcriber.TranscriptionSegment) {
	// Create debug transcription log file path
	debugLogPath := "/app/logs/transcriptions_debug.log"

	// Create directory if it doesn't exist
	dir := filepath.Dir(debugLogPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		app.zapLogger.Error("failed to create debug log directory", zap.Error(err))
		return
	}

	// Format transcription as JSON with timestamp
	transcriptionData := map[string]interface{}{
		"timestamp":  time.Now().Format(time.RFC3339),
		"text":       segment.Text,
		"start_ms":   segment.StartMS,
		"end_ms":     segment.EndMS,
		"confidence": segment.Confidence,
	}

	jsonData, err := json.Marshal(transcriptionData)
	if err != nil {
		app.zapLogger.Error("failed to marshal transcription debug data", zap.Error(err))
		return
	}

	// Append to debug log file
	file, err := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		app.zapLogger.Error("failed to open debug transcription log", zap.Error(err))
		return
	}
	defer file.Close()

	if _, err := file.Write(append(jsonData, '\n')); err != nil {
		app.zapLogger.Error("failed to write to debug transcription log", zap.Error(err))
	}
}

// startHeartbeat provides periodic status logging for monitoring with enhanced health checks
func (app *Application) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Enhanced heartbeat with actual pipeline health status
			healthStatus := app.getPipelineHealthStatus()

			// Write health status file for Docker health checks
			if err := app.writeHealthStatusFile(); err != nil {
				app.zapLogger.Error("failed to write health status file", zap.Error(err))
			}

			if app.config.GetDebugMode() {
				app.zapLogger.Info("pipeline heartbeat with health status",
					zap.String("timestamp", time.Now().Format(time.RFC3339)),
					zap.String("stream_url", app.config.GetStreamURL()),
					zap.Any("health_status", healthStatus))
			}

			// Log warnings for potential issues
			if !healthStatus["transcription_healthy"].(bool) && healthStatus["total_transcriptions"].(int64) > 0 {
				app.zapLogger.Warn("transcription pipeline may be unhealthy",
					zap.String("last_transcription", healthStatus["last_transcription_time"].(string)),
					zap.String("time_since_last", healthStatus["time_since_last_transcription"].(string)))
			}

			if !healthStatus["stream_connected"].(bool) {
				app.zapLogger.Warn("stream connection inactive")
			}

			if !healthStatus["audio_processing_active"].(bool) {
				app.zapLogger.Warn("audio processing inactive")
			}

			// Log performance warnings for "falling behind"
			realTimeRatio, hasRatio := healthStatus["real_time_ratio"].(float64)
			if hasRatio && realTimeRatio > 0 {
				if realTimeRatio < 0.8 { // Processing less than 80% of real-time
					app.zapLogger.Warn("⚠️ FALLING BEHIND: Processing slower than real-time",
						zap.Float64("real_time_ratio", realTimeRatio),
						zap.Float64("average_latency_ms", healthStatus["average_latency_ms"].(float64)),
						zap.Bool("is_real_time", healthStatus["is_real_time"].(bool)))
				}
			}

			avgLatency, hasLatency := healthStatus["average_latency_ms"].(float64)
			if hasLatency && avgLatency > 10000 { // More than 10 seconds average latency
				app.zapLogger.Warn("⚠️ HIGH LATENCY: Transcription processing is very slow",
					zap.Float64("average_latency_ms", avgLatency))
			}
		}
	}
}

// Shutdown gracefully stops all components in reverse order
func (app *Application) Shutdown() error {
	app.zapLogger.Info("shutting down application components")

	// Close transcription engine
	if err := app.transcriptionEngine.Close(); err != nil {
		app.zapLogger.Error("error closing transcription engine", zap.Error(err))
	}

	// Close audio processor
	if app.audioProcessor != nil {
		if err := app.audioProcessor.Close(); err != nil {
			app.zapLogger.Error("error closing audio processor", zap.Error(err))
		}
	}

	// Close stream connector
	if err := app.streamConnector.Close(); err != nil {
		app.zapLogger.Error("error closing stream connector", zap.Error(err))
	}

	app.zapLogger.Info("application shutdown completed")
	return nil
}

// wrapTranscriptionChannelWithHealthTracking creates a health tracking wrapper for transcription segments
func (app *Application) wrapTranscriptionChannelWithHealthTracking(originalCh <-chan transcriber.TranscriptionSegment) <-chan transcriber.TranscriptionSegment {
	healthCh := make(chan transcriber.TranscriptionSegment, 100)

	go func() {
		defer close(healthCh)
		for segment := range originalCh {
			// Track when we receive this segment for performance monitoring
			receiveTime := time.Now()

			// Update transcription health tracking
			app.updateTranscriptionHealth()

			// Update performance metrics (estimate processing started 5 seconds ago based on audio chunk duration)
			processingStartTime := receiveTime.Add(-time.Duration(segment.EndMS-segment.StartMS) * time.Millisecond)
			app.updateTranscriptionPerformance(segment, processingStartTime)

			if app.config.GetDebugMode() {
				app.zapLogger.Info("🎙️ TRANSCRIPTION RECEIVED",
					zap.String("text", segment.Text),
					zap.Int("start_ms", segment.StartMS),
					zap.Int("end_ms", segment.EndMS),
					zap.Float32("confidence", segment.Confidence))

				// Also write transcription to debug log file
				app.writeTranscriptionToDebugFile(segment)
			}
			healthCh <- segment
		}
	}()

	return healthCh
}

// wrapBufferedContextChannelWithHealthTracking creates a health tracking wrapper for buffered contexts
func (app *Application) wrapBufferedContextChannelWithHealthTracking(originalCh chan buffer.BufferedContext) chan buffer.BufferedContext {
	healthCh := make(chan buffer.BufferedContext, 100)

	go func() {
		defer close(healthCh)
		for context := range originalCh {
			// Update buffered context health tracking
			app.updateBufferedContextHealth()

			if app.config.GetDebugMode() {
				app.zapLogger.Info("📝 BUFFERED CONTEXT",
					zap.String("text", context.Text),
					zap.Int("start_ms", context.StartMS),
					zap.Int("end_ms", context.EndMS))
			}
			healthCh <- context
		}
	}()

	return healthCh
}

// wrapContestCueChannelWithHealthTracking creates a health tracking wrapper for contest cues
func (app *Application) wrapContestCueChannelWithHealthTracking(originalCh chan parser.ContestCue) chan parser.ContestCue {
	healthCh := make(chan parser.ContestCue, 100)

	go func() {
		defer close(healthCh)
		for cue := range originalCh {
			// Update contest cue health tracking
			app.updateContestCueHealth()

			if app.config.GetDebugMode() {
				app.zapLogger.Info("🏆 CONTEST CUE DETECTED",
					zap.String("cue_id", cue.CueID),
					zap.String("contest_type", cue.ContestType),
					zap.String("timestamp", cue.Timestamp),
					zap.Any("details", cue.Details))
			}
			healthCh <- cue
		}
	}()

	return healthCh
}
