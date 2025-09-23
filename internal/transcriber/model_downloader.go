package transcriber

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ModelDownloader handles downloading Whisper models from HuggingFace
type ModelDownloader struct {
	logger    *zap.Logger
	modelsDir string
	client    *http.Client
	baseURL   string
}

// NewModelDownloader creates a new model downloader instance
func NewModelDownloader(logger *zap.Logger, modelsDir string) *ModelDownloader {
	return &ModelDownloader{
		logger:    logger,
		modelsDir: modelsDir,
		client: &http.Client{
			Timeout: 10 * time.Minute, // Long timeout for large model downloads
		},
		baseURL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main",
	}
}

// GetAvailableModels returns a list of commonly available Whisper models
func (d *ModelDownloader) GetAvailableModels() []string {
	return []string{
		"tiny.en",
		"tiny",
		"base.en",
		"base",
		"small.en",
		"small",
		"medium.en",
		"medium",
		"large-v1",
		"large-v2",
		"large-v3",
	}
}

// EnsureModelExists checks if a model file exists, and downloads it if it doesn't
func (d *ModelDownloader) EnsureModelExists(modelName, modelPath string) error {
	// Check if model already exists
	if _, err := os.Stat(modelPath); err == nil {
		d.logger.Info("model already exists",
			zap.String("model", modelName),
			zap.String("path", modelPath))
		return nil
	}

	d.logger.Info("model not found locally, attempting download",
		zap.String("model", modelName),
		zap.String("path", modelPath))

	// Ensure models directory exists
	if err := os.MkdirAll(d.modelsDir, 0755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	// Download the model
	return d.downloadModel(modelName, modelPath)
}

// downloadModel downloads a model from HuggingFace
func (d *ModelDownloader) downloadModel(modelName, modelPath string) error {
	// Construct download URL
	url := fmt.Sprintf("%s/ggml-%s.bin", d.baseURL, modelName)

	d.logger.Info("downloading model from HuggingFace",
		zap.String("model", modelName),
		zap.String("url", url),
		zap.String("destination", modelPath))

	// Create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	// Set headers for better download experience
	req.Header.Set("User-Agent", "RadioContestWinner/3.1 (Go HTTP Client)")

	// Execute request
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	// Create temporary file for atomic download
	tempFile := modelPath + ".tmp"
	defer os.Remove(tempFile) // Clean up temp file if something goes wrong

	// Create output file
	out, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	// Copy with progress logging
	written, err := d.copyWithProgress(out, resp.Body, resp.ContentLength, modelName)
	if err != nil {
		return fmt.Errorf("failed to download model data: %w", err)
	}

	// Atomically move temp file to final location
	if err := os.Rename(tempFile, modelPath); err != nil {
		return fmt.Errorf("failed to move downloaded model to final location: %w", err)
	}

	d.logger.Info("model download completed successfully",
		zap.String("model", modelName),
		zap.String("path", modelPath),
		zap.Int64("bytes", written))

	return nil
}

// copyWithProgress copies data from src to dst with progress logging
func (d *ModelDownloader) copyWithProgress(dst io.Writer, src io.Reader, totalSize int64, modelName string) (int64, error) {
	const bufferSize = 32 * 1024 // 32KB buffer
	buffer := make([]byte, bufferSize)

	var written int64
	lastLogTime := time.Now()
	logInterval := 10 * time.Second

	for {
		nr, er := src.Read(buffer)
		if nr > 0 {
			nw, ew := dst.Write(buffer[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, fmt.Errorf("short write")
			}

			// Log progress periodically
			now := time.Now()
			if now.Sub(lastLogTime) >= logInterval {
				if totalSize > 0 {
					percentage := float64(written) / float64(totalSize) * 100
					d.logger.Info("download progress",
						zap.String("model", modelName),
						zap.Int64("downloaded", written),
						zap.Int64("total", totalSize),
						zap.Float64("percentage", percentage))
				} else {
					d.logger.Info("download progress",
						zap.String("model", modelName),
						zap.Int64("downloaded", written))
				}
				lastLogTime = now
			}
		}
		if er != nil {
			if er != io.EOF {
				return written, er
			}
			break
		}
	}
	return written, nil
}

// GetModelPath returns the full path for a given model name
func (d *ModelDownloader) GetModelPath(modelName string) string {
	return filepath.Join(d.modelsDir, fmt.Sprintf("ggml-%s.bin", modelName))
}

// IsValidModelName checks if a model name is in the list of known models
func (d *ModelDownloader) IsValidModelName(modelName string) bool {
	availableModels := d.GetAvailableModels()
	for _, available := range availableModels {
		if strings.EqualFold(available, modelName) {
			return true
		}
	}
	return false
}

// GetModelSize returns approximate size information for common models
func (d *ModelDownloader) GetModelSize(modelName string) string {
	sizes := map[string]string{
		"tiny.en":   "39 MB",
		"tiny":      "39 MB",
		"base.en":   "142 MB",
		"base":      "142 MB",
		"small.en":  "244 MB",
		"small":     "244 MB",
		"medium.en": "769 MB",
		"medium":    "769 MB",
		"large-v1":  "1.5 GB",
		"large-v2":  "1.5 GB",
		"large-v3":  "1.5 GB",
	}

	if size, exists := sizes[modelName]; exists {
		return size
	}
	return "Unknown"
}