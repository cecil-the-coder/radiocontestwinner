package stream

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// StreamConnector handles HTTP stream connections and provides io.Reader interface
type StreamConnector struct {
	url            string
	response       *http.Response
	client         *http.Client
	logger         *zap.Logger
	failureCount   int
	maxRetries     int
	baseBackoffMs  int
}

// NewStreamConnector creates a new StreamConnector instance
func NewStreamConnector(url string) *StreamConnector {
	return &StreamConnector{
		url: url,
		client: &http.Client{
			Timeout: 30 * time.Second, // Reasonable timeout for connection setup
		},
		logger:        zap.NewNop(), // Default no-op logger
		maxRetries:    5,
		baseBackoffMs: 1000, // 1 second base backoff
	}
}

// NewStreamConnectorWithLogger creates a new StreamConnector instance with custom logger
func NewStreamConnectorWithLogger(url string, logger *zap.Logger) *StreamConnector {
	return &StreamConnector{
		url: url,
		client: &http.Client{
			Timeout: 30 * time.Second, // Reasonable timeout for connection setup
		},
		logger:        logger,
		maxRetries:    5,
		baseBackoffMs: 1000, // 1 second base backoff
	}
}

// Connect establishes connection to the stream URL
func (s *StreamConnector) Connect(ctx context.Context) error {
	s.logger.Info("attempting to connect to stream",
		zap.String("url", s.url))

	req, err := http.NewRequestWithContext(ctx, "GET", s.url, nil)
	if err != nil {
		s.logger.Error("failed to create HTTP request",
			zap.String("url", s.url),
			zap.Error(err))
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("failed to connect to stream",
			zap.String("url", s.url),
			zap.Error(err))
		return fmt.Errorf("failed to connect to stream %s: %w", s.url, err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		s.logger.Error("stream connection failed with non-200 status",
			zap.String("url", s.url),
			zap.Int("status_code", resp.StatusCode))
		return fmt.Errorf("failed to connect to stream %s: status %d", s.url, resp.StatusCode)
	}

	s.logger.Info("successfully connected to stream",
		zap.String("url", s.url),
		zap.Int("status_code", resp.StatusCode),
		zap.String("content_type", resp.Header.Get("Content-Type")))

	s.response = resp
	return nil
}

// Read implements io.Reader interface
func (s *StreamConnector) Read(p []byte) (n int, err error) {
	if s.response == nil {
		return 0, fmt.Errorf("not connected to stream")
	}

	return s.response.Body.Read(p)
}

// ConnectWithRetry attempts to connect to the stream with automatic retry logic
func (s *StreamConnector) ConnectWithRetry(ctx context.Context) error {
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		s.logger.Info("attempting connection",
			zap.String("url", s.url),
			zap.Int("attempt", attempt),
			zap.Int("failure_count", s.failureCount))

		err := s.Connect(ctx)
		if err == nil {
			// Successful connection - reset failure counter
			s.failureCount = 0
			s.logger.Info("connection successful, failure counter reset",
				zap.String("url", s.url),
				zap.Int("attempt", attempt))
			return nil
		}

		s.failureCount++
		s.logger.Warn("connection attempt failed",
			zap.String("url", s.url),
			zap.Int("attempt", attempt),
			zap.Int("failure_count", s.failureCount),
			zap.Error(err))

		// If this was the last attempt, don't wait
		if attempt == s.maxRetries {
			break
		}

		// Calculate exponential backoff delay
		backoffMs := s.baseBackoffMs * (1 << (attempt - 1)) // 2^(attempt-1) * baseBackoff
		backoffDuration := time.Duration(backoffMs) * time.Millisecond

		s.logger.Info("waiting before retry",
			zap.String("url", s.url),
			zap.Duration("backoff", backoffDuration),
			zap.Int("next_attempt", attempt+1))

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return fmt.Errorf("connection cancelled: %w", ctx.Err())
		case <-time.After(backoffDuration):
			// Continue to next attempt
		}
	}

	s.logger.Error("maximum retry attempts exceeded",
		zap.String("url", s.url),
		zap.Int("max_retries", s.maxRetries),
		zap.Int("failure_count", s.failureCount))

	return fmt.Errorf("maximum retry attempts exceeded after %d failures", s.maxRetries)
}

// Close closes the current connection
func (s *StreamConnector) Close() error {
	if s.response != nil {
		s.logger.Info("closing stream connection", zap.String("url", s.url))
		err := s.response.Body.Close()
		s.response = nil
		return err
	}
	return nil
}