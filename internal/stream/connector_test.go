package stream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStreamConnector_Read(t *testing.T) {
	t.Run("should implement io.Reader interface", func(t *testing.T) {
		// Arrange
		testURL := "https://test.example.com/stream.aac"
		connector := NewStreamConnector(testURL)

		// Act & Assert - should implement io.Reader
		var reader io.Reader = connector
		assert.NotNil(t, reader)
	})

	t.Run("should read data from connected stream", func(t *testing.T) {
		// Arrange - create mock HTTP server
		testData := "test audio stream data"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testData))
		}))
		defer server.Close()

		connector := NewStreamConnector(server.URL)
		ctx := context.Background()
		err := connector.Connect(ctx)
		assert.NoError(t, err)

		// Act
		buffer := make([]byte, 1024)
		n, err := connector.Read(buffer)

		// Assert
		// EOF is expected when server closes connection after sending data
		if err != nil && err.Error() != "EOF" {
			t.Errorf("unexpected error: %v", err)
		}
		assert.Greater(t, n, 0, "should read some data")
		assert.Equal(t, testData, string(buffer[:n]))
	})

	t.Run("should return error when not connected", func(t *testing.T) {
		// Arrange
		connector := NewStreamConnector("http://test.example.com")

		// Act
		buffer := make([]byte, 1024)
		n, err := connector.Read(buffer)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, 0, n)
		assert.Contains(t, err.Error(), "not connected to stream")
	})
}

func TestStreamConnector_Connect(t *testing.T) {
	t.Run("should connect to stream URL successfully", func(t *testing.T) {
		// Arrange - create mock HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test stream"))
		}))
		defer server.Close()

		connector := NewStreamConnector(server.URL)
		ctx := context.Background()

		// Act
		err := connector.Connect(ctx)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("should return error for invalid URL", func(t *testing.T) {
		// Arrange
		invalidURL := "invalid-url"
		connector := NewStreamConnector(invalidURL)
		ctx := context.Background()

		// Act
		err := connector.Connect(ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect to stream")
	})

	t.Run("should return error for non-200 status code", func(t *testing.T) {
		// Arrange - create mock HTTP server that returns 404
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		connector := NewStreamConnector(server.URL)
		ctx := context.Background()

		// Act
		err := connector.Connect(ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status 404")
	})
}

func TestStreamConnector_ConnectWithRetry(t *testing.T) {
	t.Run("should retry connection on failure", func(t *testing.T) {
		// Arrange - create mock server that fails first, succeeds second
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}
		}))
		defer server.Close()

		connector := NewStreamConnector(server.URL)
		ctx := context.Background()

		// Act
		err := connector.ConnectWithRetry(ctx)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 2, attempts, "should have made 2 attempts")
	})

	t.Run("should stop after 5 consecutive failures", func(t *testing.T) {
		// Arrange - create mock server that always fails
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		connector := NewStreamConnector(server.URL)
		ctx := context.Background()

		// Act
		err := connector.ConnectWithRetry(ctx)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, 5, attempts, "should have made exactly 5 attempts")
		assert.Contains(t, err.Error(), "maximum retry attempts exceeded")
	})

	t.Run("should reset failure counter on successful connection", func(t *testing.T) {
		// Arrange - create mock server that fails 2 times, succeeds, then fails 2 more times, then succeeds
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			// Fail on attempts 1, 2, 4, 5 - succeed on 3, 6
			if attempts == 3 || attempts == 6 {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer server.Close()

		connector := NewStreamConnector(server.URL)
		ctx := context.Background()

		// Act - First connection should succeed after 3 attempts
		err := connector.ConnectWithRetry(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)

		// Simulate connection drop and reconnect
		connector.Close()

		// Act - Second connection should succeed after 3 more attempts (6 total)
		err = connector.ConnectWithRetry(ctx)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 6, attempts, "should have made 6 total attempts")
	})

	t.Run("should implement exponential backoff between retries", func(t *testing.T) {
		// Arrange - create mock server that always fails
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		connector := NewStreamConnector(server.URL)
		ctx := context.Background()

		// Act & Assert - measure time to ensure backoff is happening
		start := time.Now()
		err := connector.ConnectWithRetry(ctx)
		duration := time.Since(start)

		assert.Error(t, err)
		// With exponential backoff (1s, 2s, 4s, 8s), total should be ~15s minimum
		assert.Greater(t, duration.Seconds(), 10.0, "should take significant time due to exponential backoff")
	})
}