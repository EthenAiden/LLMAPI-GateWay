package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StreamProxy handles SSE stream proxying
type StreamProxy struct {
	httpClient    *http.Client
	connectionMgr *ConnectionManager
	logger        *zap.Logger
}

// NewStreamProxy creates a new stream proxy
func NewStreamProxy(logger *zap.Logger) *StreamProxy {
	return &StreamProxy{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // Long timeout for streaming
		},
		connectionMgr: NewConnectionManager(logger),
		logger:        logger,
	}
}

// ProxyStream proxies a streaming request
func (sp *StreamProxy) ProxyStream(ctx context.Context, w http.ResponseWriter, upstreamReq *http.Request, sessionID string) error {
	// Set SSE response headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// Send upstream request
	upstreamResp, err := sp.httpClient.Do(upstreamReq)
	if err != nil {
		sp.logger.Error("upstream request failed", zap.Error(err))
		return err
	}
	defer upstreamResp.Body.Close()

	// Create pipe for streaming
	pr, pw := io.Pipe()
	defer pr.Close()

	// Register connection
	conn := &Connection{
		ID:           sessionID,
		UpstreamResp: upstreamResp,
		ClientWriter: w,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Closed:       false,
	}
	sp.connectionMgr.Register(sessionID, conn)
	defer sp.connectionMgr.Close(sessionID)

	// Channel for errors
	errChan := make(chan error, 1)

	// Goroutine to read upstream response and parse tokens
	go func() {
		defer pw.Close()

		scanner := bufio.NewScanner(upstreamResp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1MB buffer

		for scanner.Scan() {
			line := scanner.Bytes()

			// Parse SSE data
			if bytes.HasPrefix(line, []byte("data: ")) {
				// Write to pipe
				if _, err := pw.Write(line); err != nil {
					errChan <- err
					return
				}
				if _, err := pw.Write([]byte("\n")); err != nil {
					errChan <- err
					return
				}

				// Flush to client
				flusher.Flush()
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- err
		}
	}()

	// Copy from pipe to client
	go func() {
		_, err := io.Copy(w, pr)
		if err != nil && err != io.EOF {
			sp.logger.Error("stream copy failed", zap.Error(err))
		}
	}()

	// Wait for completion or error
	select {
	case <-ctx.Done():
		sp.logger.Info("context cancelled")
		return ctx.Err()
	case err := <-errChan:
		sp.logger.Error("stream error", zap.Error(err))
		return err
	}
}

// TokenCounter tracks token usage
type TokenCounter struct {
	mu               sync.Mutex
	PromptTokens     int
	CompletionTokens int
}

// AddTokens adds tokens to the counter
func (tc *TokenCounter) AddTokens(prompt, completion int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.PromptTokens += prompt
	tc.CompletionTokens += completion
}

// GetTokens returns current token counts
func (tc *TokenCounter) GetTokens() (int, int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.PromptTokens, tc.CompletionTokens
}
