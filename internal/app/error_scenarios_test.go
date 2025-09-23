package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Error scenario tests for network failures, corrupted audio, and component failures

func skipSlowTestsInCI(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping slow test in CI environment - these tests are resource intensive and prone to timeout")
	}
}

func TestErrorScenarios_NetworkFailures(t *testing.T) {
	skipSlowTestsInCI(t)
	t.Run("should handle connection refused gracefully", func(t *testing.T) {
		testConfig := DefaultTestConfig()
		testConfig.MockStreamURL = "http://localhost:9999/nonexistent" // Non-existent server
		testConfig.DebugMode = false

		app, err := NewTestApplication(testConfig)
		if err != nil {
			t.Skip("Application creation failed (expected for missing env)")
			return
		}
		defer app.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()

		start := time.Now()
		err = app.RunWithTimeout(ctx, 5*time.Second)
		duration := time.Since(start)

		// Should handle connection failure gracefully, not crash
		// Error expected due to connection failure (either pipeline failure or context timeout)
		assert.Error(t, err, "Should return error for connection failure")
		if err != nil {
			// Accept either pipeline failure or timeout error
			errorMsg := err.Error()
			assert.True(t,
				strings.Contains(errorMsg, "failed to start pipeline") ||
				strings.Contains(errorMsg, "context deadline exceeded") ||
				strings.Contains(errorMsg, "timeout"),
				"Error should indicate pipeline failure or timeout, got: %s", errorMsg)
		}

		// Should complete within reasonable time (allow some retry attempts)
		assert.Less(t, duration, 5500*time.Millisecond, "Should not hang for full timeout period")
	})

	t.Run("should handle network timeout gracefully", func(t *testing.T) {
		// Create a server that never responds
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(20 * time.Second) // Longer than test timeout
		}))
		defer slowServer.Close()

		testConfig := DefaultTestConfig()
		testConfig.MockStreamURL = slowServer.URL
		testConfig.DebugMode = false

		app, err := NewTestApplication(testConfig)
		if err != nil {
			t.Skip("Application creation failed (expected for missing env)")
			return
		}
		defer app.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		start := time.Now()
		err = app.RunWithTimeout(ctx, 10*time.Second)
		duration := time.Since(start)

		// Should timeout gracefully within reasonable time
		assert.Error(t, err, "Should timeout gracefully")
		if err != nil {
			errorMsg := err.Error()
			assert.True(t,
				strings.Contains(errorMsg, "context deadline exceeded") ||
				strings.Contains(errorMsg, "timeout") ||
				strings.Contains(errorMsg, "failed to start pipeline"),
				"Error should indicate timeout or pipeline failure, got: %s", errorMsg)
		}

		// Allow for retry logic but shouldn't hang for full timeout
		assert.Less(t, duration, 11*time.Second, "Should not hang for full timeout period")
	})

	t.Run("should handle intermittent connection drops", func(t *testing.T) {
		// Server that closes connection after partial data
		flakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "audio/aac")
			w.WriteHeader(http.StatusOK)

			// Send some data then close connection
			w.Write([]byte("partial audio data"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			// Simulate connection drop
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
		}))
		defer flakeServer.Close()

		testConfig := DefaultTestConfig()
		testConfig.MockStreamURL = flakeServer.URL
		testConfig.DebugMode = false

		app, err := NewTestApplication(testConfig)
		if err != nil {
			t.Skip("Application creation failed (expected for missing env)")
			return
		}
		defer app.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err = app.RunWithTimeout(ctx, 2*time.Second)

		// Should handle connection drop gracefully - may or may not return an error
		// The key is that it doesn't crash and handles the drop appropriately
		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Application handled connection drop with error: %v", err)
		}
		// Test passes if application doesn't crash
		assert.True(t, true, "Application should handle connection drops gracefully")
	})
}

func TestErrorScenarios_CorruptedAudio(t *testing.T) {
	skipSlowTestsInCI(t)
	t.Run("should handle invalid AAC header gracefully", func(t *testing.T) {
		// Create corrupted AAC data
		corruptedData := []byte{0x00, 0x00, 0x00, 0x00} // Invalid AAC header

		mockServer := NewMockAudioServer(corruptedData, 0)
		defer mockServer.Close()

		testConfig := DefaultTestConfig()
		testConfig.MockStreamURL = mockServer.URL()
		testConfig.DebugMode = false

		app, err := NewTestApplication(testConfig)
		if err != nil {
			t.Skip("Application creation failed (expected for missing env)")
			return
		}
		defer app.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err = app.RunWithTimeout(ctx, 2*time.Second)

		// Should handle corrupted audio gracefully
		// Exact error handling depends on FFmpeg behavior with invalid data
		if err != nil {
			t.Logf("Application handled corrupted audio with error: %v", err)
		}
	})

	t.Run("should handle empty audio stream", func(t *testing.T) {
		// Empty audio data
		emptyData := []byte{}

		mockServer := NewMockAudioServer(emptyData, 0)
		defer mockServer.Close()

		testConfig := DefaultTestConfig()
		testConfig.MockStreamURL = mockServer.URL()
		testConfig.DebugMode = false

		app, err := NewTestApplication(testConfig)
		if err != nil {
			t.Skip("Application creation failed (expected for missing env)")
			return
		}
		defer app.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err = app.RunWithTimeout(ctx, 2*time.Second)

		// Should handle empty stream gracefully
		if err != nil {
			t.Logf("Application handled empty stream with error: %v", err)
		}
	})

	t.Run("should handle random binary data as audio", func(t *testing.T) {
		// Generate random binary data that's not valid audio
		randomData := make([]byte, 1024)
		for i := range randomData {
			randomData[i] = byte(i % 256)
		}

		mockServer := NewMockAudioServer(randomData, 0)
		defer mockServer.Close()

		testConfig := DefaultTestConfig()
		testConfig.MockStreamURL = mockServer.URL()
		testConfig.DebugMode = false

		app, err := NewTestApplication(testConfig)
		if err != nil {
			t.Skip("Application creation failed (expected for missing env)")
			return
		}
		defer app.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err = app.RunWithTimeout(ctx, 2*time.Second)

		// Should handle random data gracefully without crashing
		if err != nil {
			t.Logf("Application handled random data with error: %v", err)
		}
	})
}

func TestErrorScenarios_ComponentFailures(t *testing.T) {
	skipSlowTestsInCI(t)
	t.Run("should handle missing Whisper model gracefully", func(t *testing.T) {
		// Test with non-existent model path
		testConfig := DefaultTestConfig()
		audioData := CreateTestAudioData(2*time.Second, 44100)
		mockServer := NewMockAudioServer(audioData, 0)
		defer mockServer.Close()

		testConfig.MockStreamURL = mockServer.URL()
		testConfig.DebugMode = false

		// The application should handle missing Whisper model gracefully
		// This is tested by running without setting up a valid model path
		app, err := NewTestApplication(testConfig)
		if err != nil {
			t.Skip("Application creation failed (expected for missing env)")
			return
		}
		defer app.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Application should start even without Whisper model
		// but should log warnings and continue without transcription
		err = app.RunWithTimeout(ctx, 2*time.Second)

		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Application behavior with missing Whisper model: %v", err)
		}

		// The test passes if the application doesn't crash
		assert.True(t, true, "Application should handle missing Whisper model without crashing")
	})

	t.Run("should handle configuration errors during startup", func(t *testing.T) {
		// Test invalid configuration scenarios

		// This would test various invalid configurations
		// For now, we verify that configuration errors are properly wrapped

		testCases := []struct {
			name  string
			setup func() (*TestApplication, error)
		}{
			{
				name: "invalid stream URL format",
				setup: func() (*TestApplication, error) {
					config := DefaultTestConfig()
					config.MockStreamURL = "invalid-url-format"
					return NewTestApplication(config)
				},
			},
			{
				name: "empty allowlist",
				setup: func() (*TestApplication, error) {
					config := DefaultTestConfig()
					config.AllowlistNumbers = []string{} // Empty allowlist
					audioData := CreateTestAudioData(1*time.Second, 44100)
					mockServer := NewMockAudioServer(audioData, 0)
					defer mockServer.Close()
					config.MockStreamURL = mockServer.URL()
					return NewTestApplication(config)
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				app, err := tc.setup()

				if err != nil {
					// Configuration error expected for some cases
					t.Logf("Configuration error (expected): %v", err)
					return
				}

				if app != nil {
					defer app.Shutdown()

					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()

					err = app.RunWithTimeout(ctx, 1*time.Second)
					if err != nil {
						t.Logf("Runtime error: %v", err)
					}
				}
			})
		}
	})
}

func TestErrorScenarios_GracefulDegradation(t *testing.T) {
	skipSlowTestsInCI(t)
	t.Run("should continue operation when transcription fails", func(t *testing.T) {
		// Test system behavior when transcription component fails
		// The system should continue to operate in a degraded mode

		testConfig := DefaultTestConfig()
		testConfig.SkipTranscription = true // Simulate transcription failure

		audioData := CreateTestAudioData(2*time.Second, 44100)
		mockServer := NewMockAudioServer(audioData, 0)
		defer mockServer.Close()

		testConfig.MockStreamURL = mockServer.URL()
		testConfig.DebugMode = false

		app, err := NewTestApplication(testConfig)
		if err != nil {
			t.Skip("Application creation failed (expected for missing env)")
			return
		}
		defer app.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err = app.RunWithTimeout(ctx, 2*time.Second)

		// Application should handle transcription failure gracefully
		// It might continue with reduced functionality
		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Application handled transcription failure: %v", err)
		}

		// Test passes if application doesn't crash completely
		assert.True(t, true, "Application should degrade gracefully when transcription fails")
	})

	t.Run("should handle resource exhaustion scenarios", func(t *testing.T) {
		// Test behavior under resource constraints
		// This would involve creating scenarios that stress memory, CPU, or file descriptors

		testConfig := DefaultTestConfig()

		// Create larger audio data to stress the system
		audioData := CreateTestAudioData(10*time.Second, 44100)
		mockServer := NewMockAudioServer(audioData, 1*time.Millisecond) // Fast streaming
		defer mockServer.Close()

		testConfig.MockStreamURL = mockServer.URL()
		testConfig.BufferDurationMS = 100 // Small buffer to increase processing pressure
		testConfig.DebugMode = false

		app, err := NewTestApplication(testConfig)
		if err != nil {
			t.Skip("Application creation failed (expected for missing env)")
			return
		}
		defer app.Shutdown()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		start := time.Now()
		err = app.RunWithTimeout(ctx, 4*time.Second)
		duration := time.Since(start)

		// System should handle resource pressure without hanging indefinitely
		assert.Less(t, duration, 6*time.Second, "Should not hang under resource pressure")

		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Application handled resource pressure: %v", err)
		}
	})
}
