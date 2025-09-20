package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"radiocontestwinner/internal/config"
	"radiocontestwinner/internal/parser"
)

func TestNewLogOutput(t *testing.T) {
	t.Run("should create LogOutput with configuration dependency", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()
		logger := NewLogger()

		// Act
		logOutput, err := NewLogOutput(cfg, logger)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, logOutput)
		assert.Equal(t, "./logs/contest_output.log", logOutput.GetFilePath())
	})

	t.Run("should return error with nil configuration", func(t *testing.T) {
		// Arrange
		logger := NewLogger()

		// Act
		logOutput, err := NewLogOutput(nil, logger)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, logOutput)
		assert.Contains(t, err.Error(), "configuration cannot be nil")
	})

	t.Run("should return error with nil logger", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()

		// Act
		logOutput, err := NewLogOutput(cfg, nil)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, logOutput)
		assert.Contains(t, err.Error(), "logger cannot be nil")
	})

	t.Run("should use custom log file path from configuration", func(t *testing.T) {
		// Arrange - create temporary config file
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		customPath := "/tmp/custom_contest.log"
		configContent := `log:
  file_path: "` + customPath + `"`

		err := os.WriteFile(configFile, []byte(configContent), 0644)
		assert.NoError(t, err)

		cfg, err := config.NewConfigurationFromFile(configFile)
		assert.NoError(t, err)

		logger := NewLogger()

		// Act
		logOutput, err := NewLogOutput(cfg, logger)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, logOutput)
		assert.Equal(t, customPath, logOutput.GetFilePath())
	})
}

func TestLogOutput_FormatContestCueAsJSON(t *testing.T) {
	t.Run("should format ContestCue to required JSON structure", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)

		details := map[string]interface{}{
			"keyword":  "MONEY",
			"number":   "55555",
			"start_ms": 1000,
			"end_ms":   2000,
		}
		contestCue := parser.NewContestCue("MONEY", details)

		// Act
		jsonBytes, err := logOutput.FormatContestCueAsJSON(contestCue)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, jsonBytes)

		// Parse JSON to verify structure
		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		assert.NoError(t, err)

		// Verify required fields
		assert.Equal(t, "MONEY", result["contest_type"])
		assert.Equal(t, "MONEY", result["keyword"])
		assert.Equal(t, "55555", result["shortcode"])
		assert.NotEmpty(t, result["timestamp"])
	})

	t.Run("should extract keyword and shortcode from Details field", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)

		details := map[string]interface{}{
			"keyword":            "CASH",
			"number":             "12345",
			"original_text":      "Text CASH to 12345",
			"reconstructed_text": "Text CASH to 12345",
		}
		contestCue := parser.NewContestCue("CASH", details)

		// Act
		jsonBytes, err := logOutput.FormatContestCueAsJSON(contestCue)

		// Assert
		assert.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		assert.NoError(t, err)

		assert.Equal(t, "CASH", result["contest_type"])
		assert.Equal(t, "CASH", result["keyword"])
		assert.Equal(t, "12345", result["shortcode"])
	})

	t.Run("should return error for nil ContestCue", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)

		// Act
		jsonBytes, err := logOutput.FormatContestCueAsJSON(nil)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, jsonBytes)
		assert.Contains(t, err.Error(), "ContestCue cannot be nil")
	})

	t.Run("should handle missing keyword in Details", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)

		details := map[string]interface{}{
			"number": "55555",
		}
		contestCue := parser.NewContestCue("MONEY", details)

		// Act
		jsonBytes, err := logOutput.FormatContestCueAsJSON(contestCue)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, jsonBytes)
		assert.Contains(t, err.Error(), "keyword not found in Details")
	})

	t.Run("should handle missing number in Details", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)

		details := map[string]interface{}{
			"keyword": "MONEY",
		}
		contestCue := parser.NewContestCue("MONEY", details)

		// Act
		jsonBytes, err := logOutput.FormatContestCueAsJSON(contestCue)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, jsonBytes)
		assert.Contains(t, err.Error(), "number not found in Details")
	})
}

func TestLogOutput_WriteContestCueToFile(t *testing.T) {
	t.Run("should write ContestCue JSON to log file", func(t *testing.T) {
		// Arrange
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test_contest.log")

		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)

		// Override file path for testing
		logOutput.filePath = logFile

		details := map[string]interface{}{
			"keyword": "MONEY",
			"number":  "55555",
		}
		contestCue := parser.NewContestCue("MONEY", details)

		// Act
		err = logOutput.WriteContestCueToFile(contestCue)

		// Assert
		assert.NoError(t, err)

		// Verify file was created and contains correct JSON
		assert.FileExists(t, logFile)
		content, err := os.ReadFile(logFile)
		assert.NoError(t, err)
		assert.NotEmpty(t, content)

		// Verify JSON structure
		var result map[string]interface{}
		err = json.Unmarshal(content, &result)
		assert.NoError(t, err)
		assert.Equal(t, "MONEY", result["contest_type"])
		assert.Equal(t, "MONEY", result["keyword"])
		assert.Equal(t, "55555", result["shortcode"])
	})

	t.Run("should append multiple ContestCues to same file", func(t *testing.T) {
		// Arrange
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test_contest.log")

		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)
		logOutput.filePath = logFile

		// Create two different contest cues
		details1 := map[string]interface{}{
			"keyword": "MONEY",
			"number":  "55555",
		}
		contestCue1 := parser.NewContestCue("MONEY", details1)

		details2 := map[string]interface{}{
			"keyword": "CASH",
			"number":  "12345",
		}
		contestCue2 := parser.NewContestCue("CASH", details2)

		// Act
		err = logOutput.WriteContestCueToFile(contestCue1)
		assert.NoError(t, err)
		err = logOutput.WriteContestCueToFile(contestCue2)
		assert.NoError(t, err)

		// Assert
		content, err := os.ReadFile(logFile)
		assert.NoError(t, err)

		// Should contain both JSON objects (each on its own line)
		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		assert.Len(t, lines, 2)

		// Verify first JSON
		var result1 map[string]interface{}
		err = json.Unmarshal([]byte(lines[0]), &result1)
		assert.NoError(t, err)
		assert.Equal(t, "MONEY", result1["contest_type"])

		// Verify second JSON
		var result2 map[string]interface{}
		err = json.Unmarshal([]byte(lines[1]), &result2)
		assert.NoError(t, err)
		assert.Equal(t, "CASH", result2["contest_type"])
	})

	t.Run("should create directory if it doesn't exist", func(t *testing.T) {
		// Arrange
		tmpDir := t.TempDir()
		logDir := filepath.Join(tmpDir, "logs", "contest")
		logFile := filepath.Join(logDir, "test_contest.log")

		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)
		logOutput.filePath = logFile

		details := map[string]interface{}{
			"keyword": "MONEY",
			"number":  "55555",
		}
		contestCue := parser.NewContestCue("MONEY", details)

		// Act
		err = logOutput.WriteContestCueToFile(contestCue)

		// Assert
		assert.NoError(t, err)
		assert.FileExists(t, logFile)
	})

	t.Run("should return error for invalid file path", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)

		// Set invalid file path (path that contains invalid characters for filesystem)
		logOutput.filePath = "/proc/self/mem/invalid/contest.log"

		details := map[string]interface{}{
			"keyword": "MONEY",
			"number":  "55555",
		}
		contestCue := parser.NewContestCue("MONEY", details)

		// Act
		err = logOutput.WriteContestCueToFile(contestCue)

		// Assert
		assert.Error(t, err)
		// The error message will contain either "failed to create directory" or "failed to open file"
		assert.True(t, err != nil && (strings.Contains(err.Error(), "failed to create directory") || strings.Contains(err.Error(), "failed to open file")))
	})

	t.Run("should return error for nil ContestCue", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)

		// Act
		err = logOutput.WriteContestCueToFile(nil)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ContestCue cannot be nil")
	})
}

func TestLogOutput_ProcessContestCues(t *testing.T) {
	t.Run("should process ContestCues from channel and write to file", func(t *testing.T) {
		// Arrange
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test_contest.log")

		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)
		logOutput.filePath = logFile

		// Create channel and ContestCues
		inputCh := make(chan parser.ContestCue, 2)

		details1 := map[string]interface{}{
			"keyword": "MONEY",
			"number":  "55555",
		}
		contestCue1 := parser.NewContestCue("MONEY", details1)

		details2 := map[string]interface{}{
			"keyword": "CASH",
			"number":  "12345",
		}
		contestCue2 := parser.NewContestCue("CASH", details2)

		// Act
		go logOutput.ProcessContestCues(inputCh)

		inputCh <- *contestCue1
		inputCh <- *contestCue2
		close(inputCh)

		// Give some time for processing
		<-time.After(100 * time.Millisecond)

		// Assert
		assert.FileExists(t, logFile)
		content, err := os.ReadFile(logFile)
		assert.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		assert.Len(t, lines, 2)

		// Verify first JSON
		var result1 map[string]interface{}
		err = json.Unmarshal([]byte(lines[0]), &result1)
		assert.NoError(t, err)
		assert.Equal(t, "MONEY", result1["contest_type"])

		// Verify second JSON
		var result2 map[string]interface{}
		err = json.Unmarshal([]byte(lines[1]), &result2)
		assert.NoError(t, err)
		assert.Equal(t, "CASH", result2["contest_type"])
	})

	t.Run("should handle channel closure gracefully", func(t *testing.T) {
		// Arrange
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test_contest.log")

		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)
		logOutput.filePath = logFile

		inputCh := make(chan parser.ContestCue)

		// Act - start processing and immediately close channel
		done := make(chan bool)
		go func() {
			logOutput.ProcessContestCues(inputCh)
			done <- true
		}()

		close(inputCh)

		// Assert - should complete gracefully
		select {
		case <-done:
			// Success - processing completed
		case <-time.After(1 * time.Second):
			t.Fatal("ProcessContestCues did not complete within timeout")
		}
	})

	t.Run("should continue processing after write errors", func(t *testing.T) {
		// Arrange
		cfg := config.NewConfiguration()
		logger := NewLogger()
		logOutput, err := NewLogOutput(cfg, logger)
		assert.NoError(t, err)

		// Set invalid file path to trigger write errors
		logOutput.filePath = "/root/invalid/path/contest.log"

		inputCh := make(chan parser.ContestCue, 2)

		details := map[string]interface{}{
			"keyword": "MONEY",
			"number":  "55555",
		}
		contestCue := parser.NewContestCue("MONEY", details)

		// Act
		done := make(chan bool)
		go func() {
			logOutput.ProcessContestCues(inputCh)
			done <- true
		}()

		inputCh <- *contestCue
		inputCh <- *contestCue  // Second cue should still be processed despite first error
		close(inputCh)

		// Assert - should complete even with write errors
		select {
		case <-done:
			// Success - processing completed despite errors
		case <-time.After(1 * time.Second):
			t.Fatal("ProcessContestCues did not complete within timeout")
		}
	})
}