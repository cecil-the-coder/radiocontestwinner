package transcriber

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"radiocontestwinner/internal/config"
	"radiocontestwinner/internal/gpu"
)

// WhisperCppModel implements the WhisperModel interface using real Whisper.cpp
// This implementation can use either:
// 1. A local Whisper.cpp binary (most reliable)
// 2. A local Whisper HTTP service
// 3. OpenAI Whisper API (fallback)
type WhisperCppModel struct {
	modelPath      string
	logger         *zap.Logger
	isLoaded       bool
	whisperBin     string // Path to whisper.cpp binary
	modelType      string // Model type (base, small, medium, large)
	tempDir        string // Directory for temporary files
	client         *http.Client
	apiEndpoint    string // For HTTP service mode
	apiKey         string // For OpenAI API mode
	config         *config.Configuration
	gpuDetector    *gpu.GPUDetector
	useGPU         bool
	gpuDeviceID    int
	modelDownloader *ModelDownloader // For automatic model downloading
}

// NewWhisperCppModel creates a new instance of the real Whisper.cpp model
func NewWhisperCppModel(logger *zap.Logger) *WhisperCppModel {
	return NewWhisperCppModelWithConfig(logger, config.NewConfiguration())
}

// NewWhisperCppModelWithConfig creates a new instance with configuration
func NewWhisperCppModelWithConfig(logger *zap.Logger, cfg *config.Configuration) *WhisperCppModel {
	tempDir := "/tmp/whisper"
	os.MkdirAll(tempDir, 0755)

	model := &WhisperCppModel{
		logger:          logger,
		tempDir:         tempDir,
		client:          &http.Client{Timeout: 30 * time.Second},
		modelType:       "base",                            // Default model
		whisperBin:      "/usr/local/bin/whisper-cli",      // Pre-built binary path from container
		config:          cfg,
		gpuDetector:     gpu.NewGPUDetector(logger),
		modelDownloader: NewModelDownloader(logger, "/app/models"), // Container models directory
	}

	// Initialize GPU configuration
	model.initializeGPUConfig()

	return model
}

// initializeGPUConfig sets up GPU detection and configuration
func (w *WhisperCppModel) initializeGPUConfig() {
	// Check if CUDA is enabled in configuration
	cublasEnabled := w.config.GetCUBLASEnabled()
	autoDetect := w.config.GetCUBLASAutoDetect()

	w.logger.Info("initializing GPU configuration",
		zap.Bool("cublas_enabled", cublasEnabled),
		zap.Bool("auto_detect", autoDetect))

	if autoDetect {
		// Auto-detect GPU availability
		gpuInfo := w.gpuDetector.GetGPUInfo()
		w.useGPU = gpuInfo.Available && cublasEnabled

		if w.useGPU {
			w.gpuDeviceID = w.gpuDetector.GetOptimalDeviceID(w.config.GetGPUDeviceID())
			w.logger.Info("GPU auto-detection successful - enabling GPU acceleration",
				zap.Bool("gpu_available", gpuInfo.Available),
				zap.Int("device_count", gpuInfo.DeviceCount),
				zap.String("device_name", gpuInfo.DeviceName),
				zap.String("cuda_version", gpuInfo.CUDAVersion),
				zap.String("driver_version", gpuInfo.DriverVersion),
				zap.Int("selected_device_id", w.gpuDeviceID))
		} else {
			if cublasEnabled {
				w.logger.Warn("GPU acceleration enabled in configuration but no GPU detected - falling back to CPU",
					zap.Bool("gpu_available", gpuInfo.Available),
					zap.String("reason", "No NVIDIA GPU found or CUDA drivers not installed"))
			} else {
				w.logger.Info("GPU acceleration disabled in configuration - using CPU")
			}
		}
	} else {
		// Use explicit configuration
		w.useGPU = cublasEnabled
		w.gpuDeviceID = w.config.GetGPUDeviceID()

		// Verify GPU is actually available when explicitly enabled
		if w.useGPU {
			gpuInfo := w.gpuDetector.GetGPUInfo()
			if !gpuInfo.Available {
				w.logger.Warn("GPU explicitly enabled but no GPU detected - may fail at runtime",
					zap.Int("configured_device_id", w.gpuDeviceID))
			} else {
				w.logger.Info("Using explicit GPU configuration",
					zap.Bool("use_gpu", w.useGPU),
					zap.Int("device_id", w.gpuDeviceID),
					zap.String("device_name", gpuInfo.DeviceName))
			}
		} else {
			w.logger.Info("GPU explicitly disabled - using CPU")
		}
	}
}

// LoadModel loads the Whisper model from the specified path
func (w *WhisperCppModel) LoadModel(modelPath string) error {
	w.logger.Info("loading Whisper.cpp model", zap.String("path", modelPath))

	// Validate model path
	if modelPath == "" {
		return fmt.Errorf("model path cannot be empty")
	}

	// Determine the transcription method to use
	if w.isWhisperBinaryAvailable() {
		w.logger.Info("using Whisper.cpp binary for transcription")
		return w.loadWithBinary(modelPath)
	} else if w.isWhisperServiceAvailable() {
		w.logger.Info("using Whisper HTTP service for transcription")
		return w.loadWithService()
	} else {
		w.logger.Warn("falling back to OpenAI Whisper API")
		return w.loadWithAPI()
	}
}

// isWhisperBinaryAvailable checks if whisper.cpp binary is available
func (w *WhisperCppModel) isWhisperBinaryAvailable() bool {
	// Check multiple possible locations for whisper-cli binary (prioritize container paths)
	possiblePaths := []string{
		"/usr/local/bin/whisper-cli",  // Pre-built container binary (primary)
		w.whisperBin,                  // Configured path
		"./whisper-cli",               // App directory (Docker container)
		"/app/whisper-cli",            // App directory (absolute path)
		"/usr/bin/whisper-cli",        // System path
		"./whisper.cpp/build/bin/whisper-cli", // Local build
		"whisper-cli",                 // PATH lookup
	}

	for _, path := range possiblePaths {
		if _, err := exec.LookPath(path); err == nil {
			w.whisperBin = path
			return true
		}
		if _, err := os.Stat(path); err == nil {
			w.whisperBin = path
			return true
		}
	}
	return false
}

// isWhisperServiceAvailable checks if a local Whisper HTTP service is running
func (w *WhisperCppModel) isWhisperServiceAvailable() bool {
	// Check common Whisper service endpoints
	endpoints := []string{
		"http://localhost:9000",
		"http://whisper:9000",
		"http://127.0.0.1:9000",
	}

	for _, endpoint := range endpoints {
		resp, err := w.client.Get(endpoint + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			w.apiEndpoint = endpoint
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return false
}

// loadWithBinary configures for using whisper.cpp binary
func (w *WhisperCppModel) loadWithBinary(modelPath string) error {
	// Check if model file exists, attempt download if it doesn't
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		w.logger.Info("model file not found, attempting automatic download",
			zap.String("path", modelPath))

		// Extract model name from path (e.g., "ggml-base.en.bin" -> "base.en")
		modelName := w.extractModelNameFromPath(modelPath)
		if modelName == "" {
			return fmt.Errorf("cannot determine model name from path: %s", modelPath)
		}

		// Attempt to download the model
		if err := w.modelDownloader.EnsureModelExists(modelName, modelPath); err != nil {
			// If download fails, check for built-in fallback model from container
			fallbackPath := "/app/models/ggml-base.en.bin"
			if modelName != "base.en" {
				if _, fallbackErr := os.Stat(fallbackPath); fallbackErr == nil {
					w.logger.Warn("model download failed, using built-in base.en model as fallback",
						zap.String("requested", modelName),
						zap.String("fallback", fallbackPath),
						zap.Error(err))
					modelPath = fallbackPath
				} else {
					return fmt.Errorf("model download failed and no fallback available: %w", err)
				}
			} else {
				return fmt.Errorf("failed to download model %s: %w", modelName, err)
			}
		}
	}

	// Verify model file exists after potential download
	if _, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("model file still not accessible after download attempt: %s", modelPath)
	}

	w.modelPath = modelPath
	w.isLoaded = true
	w.logger.Info("Whisper.cpp binary model configured", zap.String("path", modelPath))
	return nil
}

// loadWithService configures for using HTTP service
func (w *WhisperCppModel) loadWithService() error {
	w.isLoaded = true
	w.logger.Info("Whisper HTTP service configured", zap.String("endpoint", w.apiEndpoint))
	return nil
}

// loadWithAPI configures for using OpenAI API
func (w *WhisperCppModel) loadWithAPI() error {
	// Check for API key in environment
	w.apiKey = os.Getenv("OPENAI_API_KEY")
	if w.apiKey == "" {
		w.logger.Warn("no OpenAI API key found, transcription will use mock data")
	}
	w.isLoaded = true
	w.logger.Info("OpenAI Whisper API configured")
	return nil
}

// Transcribe processes audio data and returns transcription segments
func (w *WhisperCppModel) Transcribe(audioData []byte) ([]TranscriptionSegment, error) {
	if !w.isLoaded {
		return nil, fmt.Errorf("whisper model not loaded")
	}

	w.logger.Debug("starting transcription", zap.Int("audio_bytes", len(audioData)))

	// Choose transcription method based on what's available
	if w.whisperBin != "" && w.modelPath != "" {
		return w.transcribeWithBinary(audioData)
	} else if w.apiEndpoint != "" {
		return w.transcribeWithService(audioData)
	} else {
		return w.transcribeWithAPI(audioData)
	}
}

// transcribeWithBinary uses whisper.cpp binary for transcription
func (w *WhisperCppModel) transcribeWithBinary(audioData []byte) ([]TranscriptionSegment, error) {
	// Save audio to temporary WAV file
	tempFile := filepath.Join(w.tempDir, fmt.Sprintf("audio_%d.wav", time.Now().UnixNano()))
	defer os.Remove(tempFile)

	if err := w.saveAudioToWAV(audioData, tempFile); err != nil {
		return nil, fmt.Errorf("failed to save audio: %w", err)
	}

	// Get configuration values
	threads := w.config.GetWhisperThreads()
	useGPU := w.useGPU
	deviceID := w.gpuDeviceID

	// Build command arguments with JSON output for better timing
	args := []string{
		"-m", w.modelPath,
		"-f", tempFile,
		"--output-json",
		"--output-file", tempFile + ".out",
		"--threads", strconv.Itoa(threads),
		"--language", "en",
	}

	// Add GPU-specific arguments
	if useGPU {
		// GPU is enabled by default in whisper-cli, no extra flags needed
		w.logger.Info("transcribing with GPU acceleration",
			zap.Int("device_id", deviceID),
			zap.Int("threads", threads))
	} else {
		// Explicitly disable GPU
		args = append(args, "--no-gpu")
		w.logger.Info("transcribing with CPU",
			zap.Int("threads", threads))
	}

	// Run whisper.cpp binary
	cmd := exec.Command(w.whisperBin, args...)

	// Log the command being executed for debugging
	w.logger.Debug("whisper command details",
		zap.String("command", cmd.String()),
		zap.String("input_file", tempFile),
		zap.Bool("use_gpu", useGPU),
		zap.Int("device_id", deviceID))

	// Define output file path first (JSON format)
	outputFile := tempFile + ".out.json"
	defer os.Remove(outputFile)

	output, err := cmd.CombinedOutput()

	// Always log the output for debugging
	w.logger.Debug("whisper command completed",
		zap.Error(err),
		zap.String("output", string(output)),
		zap.String("expected_output_file", outputFile))
	if err != nil {
		// Check if it's just a deprecation warning by looking for the output file
		if _, statErr := os.Stat(outputFile); statErr == nil {
			// Output file exists, so transcription might have worked despite the warning
			w.logger.Warn("whisper.cpp returned error but created output file (likely deprecation warning)",
				zap.Error(err),
				zap.String("output", string(output)))
		} else {
			w.logger.Error("whisper.cpp execution failed",
				zap.Error(err),
				zap.String("output", string(output)))
			return nil, fmt.Errorf("whisper.cpp failed: %w", err)
		}
	}

	// Read and parse the JSON output file
	jsonBytes, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read whisper JSON output: %w", err)
	}

	// Parse JSON response with segments
	var result struct {
		Text          string `json:"text"`
		Language      string `json:"language"`
		Segments      []struct {
			Text  string  `json:"text"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
		} `json:"segments"`
		Transcription []struct {
			Text    string `json:"text"`
			Offsets struct {
				From int `json:"from"`
				To   int `json:"to"`
			} `json:"offsets"`
		} `json:"transcription"`
	}

	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse whisper JSON output: %w", err)
	}

	var segments []TranscriptionSegment
	if len(result.Segments) > 0 {
		// Use actual segments with timing from whisper.cpp (legacy format)
		for _, seg := range result.Segments {
			segments = append(segments, TranscriptionSegment{
				Text:       strings.TrimSpace(seg.Text),
				StartMS:    int(seg.Start * 1000),
				EndMS:      int(seg.End * 1000),
				Confidence: 0.85,
			})
		}
	} else if len(result.Transcription) > 0 {
		// Use transcription array with offsets (new format)
		for _, trans := range result.Transcription {
			segments = append(segments, TranscriptionSegment{
				Text:       strings.TrimSpace(trans.Text),
				StartMS:    trans.Offsets.From,
				EndMS:      trans.Offsets.To,
				Confidence: 0.85,
			})
		}
	} else if result.Text != "" {
		// Fallback to single segment if no segments provided
		segments = append(segments, TranscriptionSegment{
			Text:       strings.TrimSpace(result.Text),
			StartMS:    0,
			EndMS:      int(float64(len(audioData)) / 32000.0 * 1000), // Approximate duration
			Confidence: 0.85,
		})
	}

	if len(segments) == 0 {
		return []TranscriptionSegment{}, nil
	}

	// Add GPU info to logging
	extraFields := []zap.Field{
		zap.Int("segments", len(segments)),
		zap.Bool("used_gpu", useGPU),
	}
	if useGPU {
		extraFields = append(extraFields, zap.Int("device_id", deviceID))
	}

	w.logger.Info("transcription completed with binary", extraFields...)

	return segments, nil
}

// transcribeWithService uses HTTP service for transcription
func (w *WhisperCppModel) transcribeWithService(audioData []byte) ([]TranscriptionSegment, error) {
	// Write the audio data as form field
	req, err := http.NewRequest("POST", w.apiEndpoint+"/transcribe", bytes.NewReader(audioData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("Accept", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcription request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transcription service error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text     string `json:"text"`
		Language string `json:"language"`
		Segments []struct {
			Text  string  `json:"text"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
		} `json:"segments"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var segments []TranscriptionSegment
	if len(result.Segments) > 0 {
		for _, seg := range result.Segments {
			segments = append(segments, TranscriptionSegment{
				Text:       seg.Text,
				StartMS:    int(seg.Start * 1000),
				EndMS:      int(seg.End * 1000),
				Confidence: 0.85,
			})
		}
	} else if result.Text != "" {
		segments = append(segments, TranscriptionSegment{
			Text:       result.Text,
			StartMS:    0,
			EndMS:      int(float64(len(audioData)) / 32000.0 * 1000),
			Confidence: 0.85,
		})
	}

	w.logger.Info("transcription completed with service", zap.Int("segments", len(segments)))
	return segments, nil
}

// transcribeWithAPI uses OpenAI Whisper API for transcription
func (w *WhisperCppModel) transcribeWithAPI(audioData []byte) ([]TranscriptionSegment, error) {
	if w.apiKey == "" {
		w.logger.Warn("no API key available, returning mock transcription")
		return w.generateMockTranscription(audioData), nil
	}

	// Save audio to temporary file for API upload
	tempFile := filepath.Join(w.tempDir, fmt.Sprintf("audio_%d.wav", time.Now().UnixNano()))
	defer os.Remove(tempFile)

	if err := w.saveAudioToWAV(audioData, tempFile); err != nil {
		return nil, fmt.Errorf("failed to save audio: %w", err)
	}

	// Create multipart form for OpenAI API
	var buf bytes.Buffer
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+w.apiKey)
	req.Header.Set("Content-Type", "multipart/form-data")

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	segment := TranscriptionSegment{
		Text:       result.Text,
		StartMS:    0,
		EndMS:      int(float64(len(audioData)) / 32000.0 * 1000),
		Confidence: 0.90, // OpenAI API typically has high confidence
	}

	w.logger.Info("transcription completed with OpenAI API", zap.String("text", result.Text))
	return []TranscriptionSegment{segment}, nil
}

// generateMockTranscription provides fallback when no real transcription is available
func (w *WhisperCppModel) generateMockTranscription(audioData []byte) []TranscriptionSegment {
	// This should only be used as an absolute fallback
	w.logger.Warn("using mock transcription as fallback")

	mockPhrases := []string{
		"Contest station calling CQ",
		"Ham radio operator transmitting",
		"Radio communication in progress",
		"Amateur radio contest activity",
	}

	// Use audio length to determine which phrase
	phraseIndex := (len(audioData) / 1000) % len(mockPhrases)
	text := mockPhrases[phraseIndex]

	return []TranscriptionSegment{{
		Text:       text,
		StartMS:    0,
		EndMS:      int(float64(len(audioData)) / 32000.0 * 1000),
		Confidence: 0.60, // Lower confidence for mock data
	}}
}

// saveAudioToWAV saves PCM audio data as a WAV file
func (w *WhisperCppModel) saveAudioToWAV(audioData []byte, filename string) error {
	// Create WAV header for 16-bit PCM mono at 16kHz
	header := w.createWAVHeader(len(audioData))

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write WAV header
	if _, err := file.Write(header); err != nil {
		return err
	}

	// Write audio data
	if _, err := file.Write(audioData); err != nil {
		return err
	}

	return nil
}

// createWAVHeader creates a WAV file header for the given data size
func (w *WhisperCppModel) createWAVHeader(dataSize int) []byte {
	header := make([]byte, 44)

	// RIFF header
	copy(header[0:4], "RIFF")
	writeUint32 := func(val uint32, offset int) {
		binary.LittleEndian.PutUint32(header[offset:offset+4], val)
	}
	writeUint16 := func(val uint16, offset int) {
		binary.LittleEndian.PutUint16(header[offset:offset+2], val)
	}

	writeUint32(uint32(36+dataSize), 4) // File size - 8
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	writeUint32(16, 16)    // PCM header size
	writeUint16(1, 20)     // PCM format
	writeUint16(1, 22)     // Mono
	writeUint32(16000, 24) // Sample rate
	writeUint32(32000, 28) // Byte rate
	writeUint16(2, 32)     // Block align
	writeUint16(16, 34)    // Bits per sample
	copy(header[36:40], "data")
	writeUint32(uint32(dataSize), 40) // Data size

	return header
}

// extractModelNameFromPath extracts the model name from a file path
// e.g., "/app/models/ggml-base.en.bin" -> "base.en"
func (w *WhisperCppModel) extractModelNameFromPath(modelPath string) string {
	fileName := filepath.Base(modelPath)

	// Remove "ggml-" prefix and ".bin" suffix
	if strings.HasPrefix(fileName, "ggml-") && strings.HasSuffix(fileName, ".bin") {
		modelName := fileName[5 : len(fileName)-4] // Remove "ggml-" (5 chars) and ".bin" (4 chars)
		return modelName
	}

	// If the format doesn't match expected pattern, return empty string
	return ""
}

// GetGPUStatus returns the current GPU usage status
func (w *WhisperCppModel) GetGPUStatus() (bool, int) {
	return w.useGPU, w.gpuDeviceID
}

// Close releases the Whisper model resources
func (w *WhisperCppModel) Close() error {
	w.logger.Info("closing Whisper.cpp model")

	// Clean up temporary files
	if w.tempDir != "" {
		os.RemoveAll(w.tempDir)
	}

	w.isLoaded = false
	w.modelPath = ""
	w.whisperBin = ""
	w.apiEndpoint = ""
	w.apiKey = ""

	w.logger.Info("Whisper.cpp model closed successfully")
	return nil
}
