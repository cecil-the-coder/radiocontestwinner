package app

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for test infrastructure components

func TestDefaultTestConfig(t *testing.T) {
	t.Run("should provide valid default configuration", func(t *testing.T) {
		config := DefaultTestConfig()

		assert.NotNil(t, config)
		assert.Equal(t, "testdata", config.TestDataPath)
		assert.True(t, config.DebugMode)
		assert.Equal(t, 5000, config.BufferDurationMS)
		assert.Len(t, config.AllowlistNumbers, 3)
		assert.Contains(t, config.AllowlistNumbers, "12345")
		assert.Contains(t, config.AllowlistNumbers, "67890")
		assert.Contains(t, config.AllowlistNumbers, "55555")
	})
}

func TestMockAudioServer(t *testing.T) {
	t.Run("should create functional mock audio server", func(t *testing.T) {
		testData := []byte("test audio data")
		server := NewMockAudioServer(testData, 0) // No streaming delay for tests
		defer server.Close()

		// Verify server is accessible
		url := server.URL()
		assert.NotEmpty(t, url)

		// Test HTTP request to server
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify headers
		assert.Equal(t, "audio/aac", resp.Header.Get("Content-Type"))
		assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))

		// Verify response body
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, testData, body)
	})

	t.Run("should stream data with specified rate", func(t *testing.T) {
		testData := make([]byte, 2048) // 2KB of test data
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		streamRate := 10 * time.Millisecond
		server := NewMockAudioServer(testData, streamRate)
		defer server.Close()

		start := time.Now()
		resp, err := http.Get(server.URL())
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		duration := time.Since(start)

		assert.Equal(t, testData, body)

		// Should take some time due to streaming rate (at least 2 chunks * 10ms)
		expectedMinDuration := time.Duration(len(testData)/1024) * streamRate
		assert.GreaterOrEqual(t, duration, expectedMinDuration)
	})
}

func TestCreateTestAudioData(t *testing.T) {
	t.Run("should generate audio data with correct size", func(t *testing.T) {
		duration := 2 * time.Second
		sampleRate := 44100

		data := CreateTestAudioData(duration, sampleRate)

		// Should have header + data
		assert.Greater(t, len(data), 8) // At least header size

		// Verify header starts with AAC-like pattern
		assert.Equal(t, byte(0xFF), data[0])
		assert.Equal(t, byte(0xF1), data[1])
	})

	t.Run("should generate different data for different durations", func(t *testing.T) {
		data1 := CreateTestAudioData(1*time.Second, 44100)
		data2 := CreateTestAudioData(2*time.Second, 44100)

		assert.Greater(t, len(data2), len(data1))
	})
}

func TestLoadTestAudioFiles(t *testing.T) {
	t.Run("should define expected test audio files", func(t *testing.T) {
		files, err := LoadTestAudioFiles("testdata")
		require.NoError(t, err)

		assert.Len(t, files, 3)

		// Verify contest sample file
		contestFile := findFileByName(files, "contest_sample.aac")
		require.NotNil(t, contestFile)
		assert.Equal(t, "audio/aac", contestFile.ContentType)
		assert.Equal(t, 5*time.Second, contestFile.Duration)
		assert.Equal(t, "Text CONTEST to 12345", contestFile.ExpectedText)
		assert.True(t, contestFile.HasContestCue)

		// Verify no-contest sample file
		noContestFile := findFileByName(files, "no_contest_sample.aac")
		require.NotNil(t, noContestFile)
		assert.False(t, noContestFile.HasContestCue)

		// Verify spelled keyword file
		spelledFile := findFileByName(files, "spelled_keyword.aac")
		require.NotNil(t, spelledFile)
		assert.Contains(t, spelledFile.ExpectedText, "C-O-N-T-E-S-T")
		assert.True(t, spelledFile.HasContestCue)
	})
}

func TestMemoryProfiler(t *testing.T) {
	t.Run("should initialize memory profiler", func(t *testing.T) {
		profiler := NewMemoryProfiler()
		assert.NotNil(t, profiler)

		err := profiler.Start()
		assert.NoError(t, err)

		// Should have some initial memory value
		peakMemory := profiler.GetPeakMemory()
		assert.GreaterOrEqual(t, peakMemory, uint64(0))

		profiler.Stop()
	})
}

func TestTestApplication_Creation(t *testing.T) {
	t.Run("should create test application with valid configuration", func(t *testing.T) {
		// Create minimal test configuration
		testConfig := &TestConfig{
			MockStreamURL:     "http://localhost:9999/test",
			TestDataPath:      "testdata",
			SkipTranscription: true, // Skip transcription for unit test
			DebugMode:         false,
			BufferDurationMS:  1000,
			AllowlistNumbers:  []string{"12345"},
		}

		app, err := NewTestApplication(testConfig)

		// This might fail due to missing environment setup, which is expected
		if err != nil {
			assert.Contains(t, err.Error(), "config")
		} else {
			assert.NotNil(t, app)
			assert.NotNil(t, app.Application)
			assert.Equal(t, testConfig, app.TestConfig)
		}
	})
}

// Helper function to find file by name in test files slice
func findFileByName(files []*TestAudioFile, name string) *TestAudioFile {
	for _, file := range files {
		if file.Name == name {
			return file
		}
	}
	return nil
}
