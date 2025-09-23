package processor

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"go.uber.org/zap"
)

// AudioProcessor manages FFmpeg process for audio format conversion
type AudioProcessor struct {
	input      io.Reader
	logger     *zap.Logger
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	ffmpegPath string
}

// NewAudioProcessor creates a new AudioProcessor instance
func NewAudioProcessor(input io.Reader, logger *zap.Logger) *AudioProcessor {
	return &AudioProcessor{
		input:      input,
		logger:     logger,
		ffmpegPath: "ffmpeg", // Default FFmpeg binary path
	}
}

// StartFFmpeg initializes and starts the FFmpeg child process
func (a *AudioProcessor) StartFFmpeg(ctx context.Context) error {
	a.logger.Info("starting ffmpeg process for audio conversion")

	// Configure FFmpeg command for AAC to 16kHz PCM conversion
	args := []string{
		"-f", "aac", // Input format: AAC
		"-i", "pipe:0", // Read from stdin
		"-ar", "16000", // Sample rate: 16kHz (required for Whisper)
		"-ac", "1", // Mono channel
		"-f", "s16le", // Output format: 16-bit little-endian PCM
		"-", // Write to stdout
	}

	a.cmd = exec.CommandContext(ctx, a.ffmpegPath, args...)

	// Set up pipes for communication
	stdin, err := a.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	a.stdin = stdin

	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	a.stdout = stdout

	stderr, err := a.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	a.stderr = stderr

	// Start the FFmpeg process
	if err := a.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	a.logger.Info("ffmpeg process started successfully",
		zap.Int("pid", a.cmd.Process.Pid))

	// Start goroutine to handle stderr logging
	go a.handleStderr()

	// Start goroutine to pipe input data to FFmpeg stdin
	go a.pipeInputToStdin()

	return nil
}

// Read implements io.Reader interface, reading converted PCM data from FFmpeg stdout
func (a *AudioProcessor) Read(p []byte) (n int, err error) {
	if a.stdout == nil {
		return 0, fmt.Errorf("ffmpeg process not started")
	}

	return a.stdout.Read(p)
}

// Close properly shuts down the FFmpeg process and cleans up resources
func (a *AudioProcessor) Close() error {
	a.logger.Info("closing audio processor")

	// Close stdin to signal FFmpeg to finish
	if a.stdin != nil {
		a.stdin.Close()
		a.stdin = nil
	}

	// Wait for the process to finish gracefully
	if a.cmd != nil && a.cmd.Process != nil {
		err := a.cmd.Wait()
		if err != nil {
			// Check for expected termination scenarios during cleanup
			if isExpectedProcessTermination(err) {
				a.logger.Debug("process terminated during cleanup", zap.Error(err))
			} else {
				a.logger.Warn("ffmpeg process ended with error", zap.Error(err))
				return fmt.Errorf("ffmpeg process error: %w", err)
			}
		} else {
			a.logger.Info("ffmpeg process ended successfully")
		}
	}

	// Close stdout and stderr after process ends
	if a.stdout != nil {
		a.stdout.Close()
		a.stdout = nil
	}
	if a.stderr != nil {
		a.stderr.Close()
		a.stderr = nil
	}

	return nil
}

// isExpectedProcessTermination checks if the error is an expected termination scenario
func isExpectedProcessTermination(err error) bool {
	errStr := err.Error()
	return errStr == "signal: broken pipe" ||
		errStr == "exit status 1" ||
		errStr == "exit status 187" // FFmpeg input format error
}

// handleStderr captures and logs FFmpeg stderr output for debugging
func (a *AudioProcessor) handleStderr() {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Warn("stderr handler panic recovered", zap.Any("panic", r))
		}
	}()

	// Get a local reference to avoid race conditions
	stderr := a.stderr
	if stderr == nil {
		return
	}

	buf := make([]byte, 1024)
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			output := string(buf[:n])
			// Log FFmpeg errors as warnings, info as debug
			if containsFFmpegError(output) {
				a.logger.Warn("ffmpeg stderr", zap.String("output", output))
			} else {
				a.logger.Debug("ffmpeg stderr", zap.String("output", output))
			}
		}
		if err != nil {
			if err != io.EOF {
				a.logger.Debug("stderr reading completed", zap.Error(err))
			}
			break
		}
	}
}

// containsFFmpegError checks if stderr output contains actual errors vs info
func containsFFmpegError(output string) bool {
	errorIndicators := []string{
		"Error opening",
		"Invalid data",
		"No such file",
		"Permission denied",
		"Connection refused",
	}

	for _, indicator := range errorIndicators {
		if len(output) >= len(indicator) {
			for i := 0; i <= len(output)-len(indicator); i++ {
				if output[i:i+len(indicator)] == indicator {
					return true
				}
			}
		}
	}
	return false
}

// pipeInputToStdin pipes data from the input reader to FFmpeg stdin
func (a *AudioProcessor) pipeInputToStdin() {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Warn("stdin piping panic recovered", zap.Any("panic", r))
		}
	}()

	if a.stdin == nil || a.input == nil {
		return
	}

	defer func() {
		if a.stdin != nil {
			a.stdin.Close()
		}
	}()

	_, err := io.Copy(a.stdin, a.input)
	if err != nil {
		a.logger.Error("error piping input to ffmpeg stdin", zap.Error(err))
	} else {
		a.logger.Debug("successfully piped all input data to ffmpeg")
	}
}
