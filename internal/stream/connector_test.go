package stream

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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

	t.Run("should set proper User-Agent and audio streaming headers", func(t *testing.T) {
		// Arrange - create mock HTTP server that captures headers
		var capturedHeaders http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedHeaders = r.Header.Clone()
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

		// Verify User-Agent is set to a realistic browser string
		userAgent := capturedHeaders.Get("User-Agent")
		assert.NotEmpty(t, userAgent)
		assert.NotEqual(t, "Go-http-client/1.1", userAgent, "should not use default Go user agent")
		assert.Contains(t, userAgent, "Mozilla/5.0", "should use realistic browser user agent")
		assert.Contains(t, userAgent, "Chrome", "should identify as Chrome browser")

		// Verify audio streaming headers
		accept := capturedHeaders.Get("Accept")
		assert.Contains(t, accept, "audio/aac", "should accept AAC audio")
		assert.Contains(t, accept, "audio/mpeg", "should accept MPEG audio")
		assert.Contains(t, accept, "audio/*", "should accept general audio types")

		// Verify other streaming headers
		assert.Equal(t, "identity", capturedHeaders.Get("Accept-Encoding"), "should not compress audio streams")
		assert.Equal(t, "no-cache", capturedHeaders.Get("Cache-Control"), "should not cache audio streams")
		assert.Equal(t, "keep-alive", capturedHeaders.Get("Connection"), "should keep connection alive")
		assert.Contains(t, capturedHeaders.Get("Accept-Language"), "en-US", "should include language preference")

		// Verify referer is set
		referer := capturedHeaders.Get("Referer")
		assert.NotEmpty(t, referer, "should set referer header")
		assert.Contains(t, referer, "radio-browser.info", "should use radio streaming referer")
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
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping retry tests in CI environment - these tests involve long backoff delays")
	}
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

func TestStreamConnector_ExtendedStreamingTimeout(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping extended streaming timeout test in CI environment - this test takes 40+ seconds")
	}
	t.Run("should support extended streaming connections beyond 30 seconds", func(t *testing.T) {
		// Arrange - create mock server that streams data slowly over 35 seconds
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)

			// Stream data every second for 35 seconds to exceed current 30s timeout
			for i := 0; i < 35; i++ {
				w.Write([]byte("data chunk "))
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(1 * time.Second)
			}
		}))
		defer server.Close()

		connector := NewStreamConnector(server.URL)
		ctx := context.Background()

		// Act
		err := connector.Connect(ctx)
		assert.NoError(t, err)

		// Read from stream for 35+ seconds
		start := time.Now()
		totalBytes := 0
		buffer := make([]byte, 1024)

		for time.Since(start) < 36*time.Second {
			n, readErr := connector.Read(buffer)
			if readErr != nil && readErr != io.EOF {
				t.Errorf("stream read failed after %v: %v", time.Since(start), readErr)
				break
			}
			totalBytes += n
			if readErr == io.EOF {
				break
			}
		}

		// Assert
		duration := time.Since(start)
		assert.Greater(t, duration.Seconds(), 30.0, "should read for more than 30 seconds without timeout")
		assert.Greater(t, totalBytes, 300, "should have read substantial data")
	})

	t.Run("should have reasonable timeout for initial connection", func(t *testing.T) {
		// Arrange - create server that takes 5 seconds to respond to initial connection
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("connected"))
		}))
		defer server.Close()

		connector := NewStreamConnector(server.URL)
		ctx := context.Background()

		// Act
		start := time.Now()
		err := connector.Connect(ctx)
		duration := time.Since(start)

		// Assert - should succeed within reasonable time
		assert.NoError(t, err)
		assert.Less(t, duration.Seconds(), 10.0, "initial connection should complete within reasonable timeout")
		assert.Greater(t, duration.Seconds(), 4.0, "should wait for server response")
	})

	t.Run("should timeout on initial connection if server is unresponsive", func(t *testing.T) {
		// Arrange - create server that never responds
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(1 * time.Hour) // Never respond
		}))
		defer server.Close()

		// Start server but make it unresponsive by using invalid listener
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		listener.Close() // Close immediately to make it unresponsive

		invalidURL := fmt.Sprintf("http://%s", listener.Addr().String())
		connector := NewStreamConnector(invalidURL)
		ctx := context.Background()

		// Act
		start := time.Now()
		err = connector.Connect(ctx)
		duration := time.Since(start)

		// Assert - should fail within reasonable timeout for connection establishment
		assert.Error(t, err)
		assert.Less(t, duration.Seconds(), 15.0, "should timeout within reasonable time for connection")
	})
}
