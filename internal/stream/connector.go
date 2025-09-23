package stream

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// StreamConnector handles HTTP stream connections and provides io.Reader interface
type StreamConnector struct {
	url           string
	response      *http.Response
	client        *http.Client
	logger        *zap.Logger
	failureCount  int
	maxRetries    int
	baseBackoffMs int
}

// NewStreamConnector creates a new StreamConnector instance
func NewStreamConnector(url string) *StreamConnector {
	maxRetries := 5
	baseBackoffMs := 1000

	// Allow test environment to override retry parameters
	if envMaxRetries := os.Getenv("STREAM_MAX_RETRIES"); envMaxRetries != "" {
		if retries, err := strconv.Atoi(envMaxRetries); err == nil && retries >= 0 {
			maxRetries = retries
		}
	}
	if envBackoff := os.Getenv("STREAM_BASE_BACKOFF_MS"); envBackoff != "" {
		if backoff, err := strconv.Atoi(envBackoff); err == nil && backoff >= 0 {
			baseBackoffMs = backoff
		}
	}

	return &StreamConnector{
		url:           url,
		client:        createStreamingHTTPClient(),
		logger:        zap.NewNop(), // Default no-op logger
		maxRetries:    maxRetries,
		baseBackoffMs: baseBackoffMs,
	}
}

// NewStreamConnectorWithLogger creates a new StreamConnector instance with custom logger
func NewStreamConnectorWithLogger(url string, logger *zap.Logger) *StreamConnector {
	maxRetries := 5
	baseBackoffMs := 1000

	// Allow test environment to override retry parameters
	if envMaxRetries := os.Getenv("STREAM_MAX_RETRIES"); envMaxRetries != "" {
		if retries, err := strconv.Atoi(envMaxRetries); err == nil && retries >= 0 {
			maxRetries = retries
		}
	}
	if envBackoff := os.Getenv("STREAM_BASE_BACKOFF_MS"); envBackoff != "" {
		if backoff, err := strconv.Atoi(envBackoff); err == nil && backoff >= 0 {
			baseBackoffMs = backoff
		}
	}

	return &StreamConnector{
		url:           url,
		client:        createStreamingHTTPClient(),
		logger:        logger,
		maxRetries:    maxRetries,
		baseBackoffMs: baseBackoffMs,
	}
}

// createStreamingHTTPClient creates an HTTP client optimized for streaming connections
// with separate timeouts for connection establishment vs streaming reads
func createStreamingHTTPClient() *http.Client {
	// Custom transport with connection timeout but no overall request timeout
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // Timeout for initial connection establishment
			KeepAlive: 30 * time.Second, // Keep connections alive for reuse
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second, // Timeout for TLS handshake
		ResponseHeaderTimeout: 30 * time.Second, // Timeout for response headers
		ExpectContinueTimeout: 1 * time.Second,  // Timeout for Expect: 100-continue
		// No IdleConnTimeout - keep connections open for streaming
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	}

	// HTTP client with NO overall timeout to allow indefinite streaming
	return &http.Client{
		Transport: transport,
		// No Timeout field - allows indefinite streaming reads
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

	// Set realistic browser User-Agent to avoid being flagged as a bot
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Set audio streaming specific headers
	req.Header.Set("Accept", "audio/aac,audio/mpeg,audio/*,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "identity") // Don't compress audio streams
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Set referer to look like a browser request (optional - can help with some streams)
	req.Header.Set("Referer", "https://www.radio-browser.info/")

	s.logger.Debug("HTTP headers set for stream request",
		zap.String("user_agent", req.Header.Get("User-Agent")),
		zap.String("accept", req.Header.Get("Accept")))

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
	var lastErr error

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

		lastErr = err
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
			// Include the last connection error to preserve error type information
			if lastErr != nil {
				return fmt.Errorf("failed to connect to stream after retries: connection cancelled: %w (last error: %v)", ctx.Err(), lastErr)
			}
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
