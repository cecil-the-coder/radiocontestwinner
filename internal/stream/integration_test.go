package stream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"radiocontestwinner/internal/config"
	"radiocontestwinner/internal/logger"
)

func TestStreamConnector_Integration(t *testing.T) {
	t.Run("should connect to stream URL from configuration file", func(t *testing.T) {
		// Arrange - create mock HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("integration test stream data"))
		}))
		defer server.Close()

		// Create temporary config file with server URL
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		configContent := "stream:\n  url: \"" + server.URL + "\""

		err := os.WriteFile(configFile, []byte(configContent), 0644)
		assert.NoError(t, err)

		// Load configuration from file
		cfg, err := config.NewConfigurationFromFile(configFile)
		assert.NoError(t, err)

		// Create stream connector with configured URL and logger
		streamURL := cfg.GetStreamURL()
		logger, err := logger.NewDevelopmentLogger()
		assert.NoError(t, err)
		connector := NewStreamConnectorWithLogger(streamURL, logger)
		ctx := context.Background()

		// Act
		err = connector.Connect(ctx)
		assert.NoError(t, err)

		// Assert - read data from connected stream
		buffer := make([]byte, 1024)
		n, err := connector.Read(buffer)

		// EOF is expected when server closes connection
		if err != nil && err.Error() != "EOF" {
			t.Errorf("unexpected error: %v", err)
		}
		assert.Greater(t, n, 0, "should read some data")
		assert.Equal(t, "integration test stream data", string(buffer[:n]))

		// Cleanup
		connector.Close()
	})

	t.Run("should use environment variable configuration for reconnection", func(t *testing.T) {
		// Arrange - create mock server that fails first, then succeeds
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("reconnection success"))
			}
		}))
		defer server.Close()

		// Set environment variable
		os.Setenv("STREAM_URL", server.URL)
		defer os.Unsetenv("STREAM_URL")

		// Load configuration from environment
		cfg, err := config.NewConfigurationFromEnv()
		assert.NoError(t, err)

		// Create stream connector with configured URL and logger
		streamURL := cfg.GetStreamURL()
		logger, err := logger.NewDevelopmentLogger()
		assert.NoError(t, err)
		connector := NewStreamConnectorWithLogger(streamURL, logger)
		ctx := context.Background()

		// Act - use ConnectWithRetry to test reconnection
		err = connector.ConnectWithRetry(ctx)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 2, attempts, "should have made 2 attempts")

		// Read data to verify connection works
		buffer := make([]byte, 1024)
		n, err := connector.Read(buffer)

		if err != nil && err.Error() != "EOF" {
			t.Errorf("unexpected error: %v", err)
		}
		assert.Greater(t, n, 0)
		assert.Equal(t, "reconnection success", string(buffer[:n]))

		// Cleanup
		connector.Close()
	})

	t.Run("should handle stream disconnection and reconnection flow", func(t *testing.T) {
		// Arrange - create mock server that simulates disconnection
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if requestCount <= 2 {
				// First two requests succeed
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("streaming data"))
			} else {
				// Subsequent requests succeed after reconnection
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("reconnected stream"))
			}
		}))
		defer server.Close()

		// Create configuration
		logger, err := logger.NewDevelopmentLogger()
		assert.NoError(t, err)
		connector := NewStreamConnectorWithLogger(server.URL, logger)
		ctx := context.Background()

		// Act - Initial connection
		err = connector.Connect(ctx)
		assert.NoError(t, err)

		// Read some data
		buffer := make([]byte, 1024)
		n, err := connector.Read(buffer)
		if err != nil && err.Error() != "EOF" {
			t.Errorf("unexpected error: %v", err)
		}
		assert.Greater(t, n, 0)

		// Simulate disconnection
		connector.Close()

		// Reconnect
		err = connector.Connect(ctx)
		assert.NoError(t, err)

		// Read data from reconnected stream
		n, err = connector.Read(buffer)
		if err != nil && err.Error() != "EOF" {
			t.Errorf("unexpected error: %v", err)
		}
		assert.Greater(t, n, 0)

		// Cleanup
		connector.Close()

		// Assert that server was called multiple times
		assert.GreaterOrEqual(t, requestCount, 2, "should have made multiple requests")
	})
}
