package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewLogger(t *testing.T) {
	t.Run("should create a new zap logger instance", func(t *testing.T) {
		// Act
		logger := NewLogger()

		// Assert
		assert.NotNil(t, logger)
		assert.IsType(t, &zap.Logger{}, logger)
	})

	t.Run("should create logger with JSON encoder for production", func(t *testing.T) {
		// Act
		logger, err := NewProductionLogger()

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.IsType(t, &zap.Logger{}, logger)
	})

	t.Run("should create logger with development config for testing", func(t *testing.T) {
		// Act
		logger, err := NewDevelopmentLogger()

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.IsType(t, &zap.Logger{}, logger)
	})
}
