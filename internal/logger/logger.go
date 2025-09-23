package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"

	"radiocontestwinner/internal/config"
	"radiocontestwinner/internal/parser"
)

// NewLogger creates a new zap logger with default configuration
func NewLogger() *zap.Logger {
	logger, err := zap.NewProduction()
	if err != nil {
		// Fallback to no-op logger if production logger fails
		return zap.NewNop()
	}
	return logger
}

// NewProductionLogger creates a new zap logger configured for production use
func NewProductionLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build production logger: %w", err)
	}
	return logger, nil
}

// NewDevelopmentLogger creates a new zap logger configured for development use
func NewDevelopmentLogger() (*zap.Logger, error) {
	config := zap.NewDevelopmentConfig()
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build development logger: %w", err)
	}
	return logger, nil
}

// LogOutput handles writing contest cues to a configured log file
type LogOutput struct {
	filePath string
	logger   *zap.Logger
	mutex    sync.Mutex // For thread-safe file writing
}

// NewLogOutput creates a new LogOutput with configuration dependency
func NewLogOutput(cfg *config.Configuration, logger *zap.Logger) (*LogOutput, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	filePath := cfg.GetLogFilePath()

	return &LogOutput{
		filePath: filePath,
		logger:   logger,
	}, nil
}

// GetFilePath returns the configured file path
func (lo *LogOutput) GetFilePath() string {
	return lo.filePath
}

// FormatContestCueAsJSON formats a ContestCue into the required JSON structure
func (lo *LogOutput) FormatContestCueAsJSON(cue *parser.ContestCue) ([]byte, error) {
	if cue == nil {
		return nil, fmt.Errorf("ContestCue cannot be nil")
	}

	// Extract keyword from Details
	keyword, exists := cue.Details["keyword"]
	if !exists {
		return nil, fmt.Errorf("keyword not found in Details")
	}

	// Extract number (shortcode) from Details
	number, exists := cue.Details["number"]
	if !exists {
		return nil, fmt.Errorf("number not found in Details")
	}

	// Create the required JSON structure
	output := map[string]interface{}{
		"contest_type": cue.ContestType,
		"keyword":      keyword,
		"shortcode":    number,
		"timestamp":    cue.Timestamp,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ContestCue to JSON: %w", err)
	}

	return jsonBytes, nil
}

// WriteContestCueToFile writes a ContestCue to the configured log file in JSON format
func (lo *LogOutput) WriteContestCueToFile(cue *parser.ContestCue) error {
	if cue == nil {
		return fmt.Errorf("ContestCue cannot be nil")
	}

	// Format ContestCue as JSON
	jsonBytes, err := lo.FormatContestCueAsJSON(cue)
	if err != nil {
		return fmt.Errorf("failed to format ContestCue as JSON: %w", err)
	}

	// Thread-safe file writing
	lo.mutex.Lock()
	defer lo.mutex.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(lo.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open file for appending (create if doesn't exist)
	file, err := os.OpenFile(lo.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", lo.filePath, err)
	}
	defer file.Close()

	// Write JSON with newline
	if _, err := file.Write(append(jsonBytes, '\n')); err != nil {
		return fmt.Errorf("failed to write ContestCue to file %s: %w", lo.filePath, err)
	}

	return nil
}

// ProcessContestCues continuously processes ContestCues from the input channel
func (lo *LogOutput) ProcessContestCues(inputCh <-chan parser.ContestCue) {
	lo.logger.Info("starting contest cue processing pipeline")
	processedCount := 0
	successCount := 0

	for cue := range inputCh {
		processedCount++
		lo.logger.Debug("processing contest cue",
			zap.String("cue_id", cue.CueID),
			zap.String("contest_type", cue.ContestType),
			zap.Int("processed_count", processedCount))

		// Write ContestCue to file
		if err := lo.WriteContestCueToFile(&cue); err != nil {
			lo.logger.Error("failed to write ContestCue to file",
				zap.Error(err),
				zap.String("cue_id", cue.CueID),
				zap.String("contest_type", cue.ContestType),
				zap.String("file_path", lo.filePath))
			// Continue processing despite errors
			continue
		}

		successCount++
		lo.logger.Debug("ContestCue written to file successfully",
			zap.String("cue_id", cue.CueID),
			zap.String("file_path", lo.filePath),
			zap.Int("success_count", successCount))
	}

	lo.logger.Info("contest cue processing pipeline completed",
		zap.Int("total_processed", processedCount),
		zap.Int("successful_writes", successCount),
		zap.String("file_path", lo.filePath))
}
