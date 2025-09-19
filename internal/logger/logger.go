package logger

import (
	"fmt"

	"go.uber.org/zap"
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