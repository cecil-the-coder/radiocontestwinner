package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// TestConfig represents configuration for testing scenarios
type TestConfig struct {
	MockStreamURL     string
	TestDataPath      string
	SkipTranscription bool
	DebugMode         bool
	BufferDurationMS  int
	AllowlistNumbers  []string
}

// DefaultTestConfig returns a default test configuration
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		MockStreamURL:     "", // Will be set by mock server
		TestDataPath:      "testdata",
		SkipTranscription: false,
		DebugMode:         true,
		BufferDurationMS:  5000,
		AllowlistNumbers:  []string{"12345", "67890", "55555"},
	}
}

// MockAudioServer provides HTTP mock server for audio streaming tests
type MockAudioServer struct {
	server     *httptest.Server
	audioData  []byte
	streamRate time.Duration // Rate at which to stream audio chunks
}

// NewMockAudioServer creates a new mock audio server
func NewMockAudioServer(audioData []byte, streamRate time.Duration) *MockAudioServer {
	mock := &MockAudioServer{
		audioData:  audioData,
		streamRate: streamRate,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.handleAudioStream(w, r)
	})

	mock.server = httptest.NewServer(handler)
	return mock
}

// URL returns the mock server URL
func (m *MockAudioServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *MockAudioServer) Close() {
	m.server.Close()
}

// handleAudioStream simulates streaming audio data
func (m *MockAudioServer) handleAudioStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "audio/aac")
	w.Header().Set("Cache-Control", "no-cache")

	// Stream audio data in chunks to simulate real streaming
	chunkSize := 1024
	for i := 0; i < len(m.audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(m.audioData) {
			end = len(m.audioData)
		}

		chunk := m.audioData[i:end]
		if _, err := w.Write(chunk); err != nil {
			return
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Simulate streaming delay
		if m.streamRate > 0 {
			time.Sleep(m.streamRate)
		}
	}
}

// TestAudioFile represents a test audio file with metadata
type TestAudioFile struct {
	Name          string
	Path          string
	ContentType   string
	Duration      time.Duration
	SampleRate    int
	ExpectedText  string // Expected transcription output
	HasContestCue bool   // Whether this audio should produce contest cues
}

// LoadTestAudioFiles loads all available test audio files
func LoadTestAudioFiles(testDataPath string) ([]*TestAudioFile, error) {
	audioDir := filepath.Join(testDataPath, "audio")

	files := []*TestAudioFile{
		{
			Name:          "contest_sample.aac",
			Path:          filepath.Join(audioDir, "contest_sample.aac"),
			ContentType:   "audio/aac",
			Duration:      5 * time.Second,
			SampleRate:    44100,
			ExpectedText:  "Text CONTEST to 12345",
			HasContestCue: true,
		},
		{
			Name:          "no_contest_sample.aac",
			Path:          filepath.Join(audioDir, "no_contest_sample.aac"),
			ContentType:   "audio/aac",
			Duration:      3 * time.Second,
			SampleRate:    44100,
			ExpectedText:  "This is just regular speech with no contest information",
			HasContestCue: false,
		},
		{
			Name:          "spelled_keyword.aac",
			Path:          filepath.Join(audioDir, "spelled_keyword.aac"),
			ContentType:   "audio/aac",
			Duration:      6 * time.Second,
			SampleRate:    44100,
			ExpectedText:  "Text C-O-N-T-E-S-T to 67890",
			HasContestCue: true,
		},
	}

	// Verify files exist or create placeholder info
	for _, file := range files {
		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			// File doesn't exist yet - this is expected for initial test setup
			continue
		}
	}

	return files, nil
}

// CreateTestAudioData generates synthetic audio data for testing
func CreateTestAudioData(duration time.Duration, sampleRate int) []byte {
	// This creates a minimal AAC-like header for testing
	// In real implementation, this would be actual AAC encoded audio

	// AAC ADTS header (simplified for testing)
	header := []byte{
		0xFF, 0xF1, // Sync word and MPEG layer
		0x50,       // Profile and frequency
		0x80,       // Channel config
		0x03, 0xE0, // Frame length (example)
		0x00, 0x00, // Other header fields
	}

	// Calculate approximate data size for duration
	bytesPerSecond := sampleRate * 2 // Rough estimate for AAC
	dataSize := int(duration.Seconds()) * bytesPerSecond

	// Create synthetic audio data
	data := make([]byte, dataSize)
	for i := range data {
		// Generate simple pattern to simulate audio data
		data[i] = byte(i % 256)
	}

	// Combine header and data
	result := make([]byte, len(header)+len(data))
	copy(result, header)
	copy(result[len(header):], data)

	return result
}

// TestApplication creates an application configured for testing
type TestApplication struct {
	*Application
	TestConfig *TestConfig
	MockServer *MockAudioServer
	TestLogger *zap.Logger
}

// NewTestApplication creates a new application instance for testing
func NewTestApplication(testConfig *TestConfig) (*TestApplication, error) {
	// Set up test environment variables
	originalStreamURL := os.Getenv("STREAM_URL")
	originalDebugMode := os.Getenv("DEBUG_MODE")

	defer func() {
		// Restore original values
		if originalStreamURL == "" {
			os.Unsetenv("STREAM_URL")
		} else {
			os.Setenv("STREAM_URL", originalStreamURL)
		}

		if originalDebugMode == "" {
			os.Unsetenv("DEBUG_MODE")
		} else {
			os.Setenv("DEBUG_MODE", originalDebugMode)
		}
	}()

	// Set test environment
	os.Setenv("STREAM_URL", testConfig.MockStreamURL)
	if testConfig.DebugMode {
		os.Setenv("DEBUG_MODE", "true")
	} else {
		os.Setenv("DEBUG_MODE", "false")
	}

	// Create application
	app, err := NewApplication()
	if err != nil {
		return nil, fmt.Errorf("failed to create test application: %w", err)
	}

	// Create test logger for verification
	testLogger := zap.NewNop() // Use no-op logger for tests to reduce noise

	return &TestApplication{
		Application: app,
		TestConfig:  testConfig,
		TestLogger:  testLogger,
	}, nil
}

// RunWithTimeout runs the test application with a timeout
func (ta *TestApplication) RunWithTimeout(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return ta.Application.Run(ctx)
}

// PipelineTestResult captures results from pipeline testing
type PipelineTestResult struct {
	TranscriptionSegments []interface{} // Store transcription segments
	ContestCues           []interface{} // Store contest cues
	ProcessingLatency     time.Duration
	MemoryUsage           uint64
	CPUUsage              float64
	Errors                []error
}

// MemoryProfiler provides memory usage monitoring for tests
type MemoryProfiler struct {
	initialMemory uint64
	peakMemory    uint64
	samples       []uint64
}

// NewMemoryProfiler creates a new memory profiler
func NewMemoryProfiler() *MemoryProfiler {
	return &MemoryProfiler{
		samples: make([]uint64, 0),
	}
}

// Start begins memory monitoring
func (mp *MemoryProfiler) Start() error {
	// Implementation would use runtime.MemStats to track memory
	// For now, provide placeholder structure
	mp.initialMemory = 1024 * 1024 // 1MB placeholder
	return nil
}

// GetPeakMemory returns the peak memory usage
func (mp *MemoryProfiler) GetPeakMemory() uint64 {
	return mp.peakMemory
}

// Stop ends memory monitoring
func (mp *MemoryProfiler) Stop() {
	// Cleanup monitoring
}
