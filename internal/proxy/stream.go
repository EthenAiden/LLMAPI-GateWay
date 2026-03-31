package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StreamResult holds the token counts collected during a stream
type StreamResult struct {
	PromptTokens     int
	CompletionTokens int
}

// StreamProxy handles SSE stream proxying via io.Pipe + goroutines
type StreamProxy struct {
	connectionMgr *ConnectionManager
	logger        *zap.Logger
}

// NewStreamProxy creates a new stream proxy
func NewStreamProxy(logger *zap.Logger) *StreamProxy {
	return &StreamProxy{
		connectionMgr: NewConnectionManager(logger),
		logger:        logger,
	}
}

// StartCleanup starts the stale-connection cleanup loop; call once at startup
func (sp *StreamProxy) StartCleanup(done <-chan struct{}) {
	sp.connectionMgr.StartCleanup(done)
}

// GetConnectionCount returns the number of active streaming connections
func (sp *StreamProxy) GetConnectionCount() int {
	return sp.connectionMgr.GetConnectionCount()
}

// ProxyStream proxies an already-initiated upstream HTTP response to the client via io.Pipe.
//
// Architecture:
//
//	goroutine A (upstream reader):
//	    reads SSE lines from upstreamResp.Body → writes to pw → counts tokens
//	    on EOF/error: pw.CloseWithError(err) — unblocks goroutine B
//
//	goroutine B (pipe → client writer):  [runs in calling goroutine]
//	    reads from pr → writes SSE lines to w + flushes
//
// Down-client disconnect:
//
//	ctx is cancelled → pw.CloseWithError(ctx.Err()) → goroutine A exits
//	ConnectionManager.Close() closes upstreamResp.Body → goroutine A's scanner exits
//
// Returns token counts so the caller can settle quota.
func (sp *StreamProxy) ProxyStream(
	ctx context.Context,
	w http.ResponseWriter,
	flusher http.Flusher,
	upstreamResp *http.Response,
	sessionID string,
) (StreamResult, error) {
	// Register connection so cleanup goroutine can reclaim it on timeout
	conn := &Connection{
		ID:           sessionID,
		UpstreamResp: upstreamResp,
		ClientWriter: w,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Closed:       false,
	}
	sp.connectionMgr.Register(sessionID, conn)

	pr, pw := io.Pipe()

	var result StreamResult

	// Goroutine A: read upstream SSE, count tokens, write raw lines to pipe
	go func() {
		var writeErr error
		defer func() {
			pw.CloseWithError(writeErr) // unblocks pr.Read in the main goroutine
		}()

		scanner := bufio.NewScanner(upstreamResp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			// Check context before processing each line
			select {
			case <-ctx.Done():
				writeErr = ctx.Err()
				return
			default:
			}

			line := scanner.Bytes()

			if !bytes.HasPrefix(line, []byte("data: ")) {
				// Blank lines or event: lines — forward as-is
				if _, err := pw.Write(append(line, '\n')); err != nil {
					writeErr = err
					return
				}
				continue
			}

			data := bytes.TrimPrefix(line, []byte("data: "))

			if bytes.Equal(data, []byte("[DONE]")) {
				_, writeErr = fmt.Fprintf(pw, "data: [DONE]\n\n")
				return
			}

			// Parse chunk for token counting
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
				} `json:"usage,omitempty"`
			}
			if err := json.Unmarshal(data, &chunk); err == nil {
				if chunk.Usage != nil && chunk.Usage.PromptTokens > 0 {
					// Final chunk with accurate usage — override estimates
					result.PromptTokens = chunk.Usage.PromptTokens
					result.CompletionTokens = chunk.Usage.CompletionTokens
				} else {
					for _, choice := range chunk.Choices {
						if c := choice.Delta.Content; c != "" {
							t := len([]rune(c)) / 4
							if t == 0 {
								t = 1
							}
							result.CompletionTokens += t
						}
					}
				}
			}

			// Forward the SSE line through the pipe
			if _, err := fmt.Fprintf(pw, "data: %s\n\n", data); err != nil {
				writeErr = err
				return
			}

			sp.connectionMgr.UpdateActivity(sessionID)
		}

		if err := scanner.Err(); err != nil {
			writeErr = err
		}
	}()

	// Main goroutine (B): pipe reader → client writer
	// io.Copy exits when pw is closed (by goroutine A or ctx cancel below)
	copyDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 4*1024)
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					// Client disconnected
					copyDone <- werr
					return
				}
				flusher.Flush()
			}
			if err != nil {
				if err == io.EOF {
					copyDone <- nil
				} else {
					copyDone <- err
				}
				return
			}
		}
	}()

	// Watch for context cancellation; interrupt pipe so goroutine A and B both exit
	var streamErr error
	select {
	case <-ctx.Done():
		// Client disconnected or request cancelled — close the pipe to unblock both goroutines
		pw.CloseWithError(ctx.Err())
		pr.CloseWithError(ctx.Err())
		// Close upstream body via ConnectionManager (prevents goroutine leak)
		sp.connectionMgr.Close(sessionID)
		streamErr = ctx.Err()
		// Drain copyDone
		<-copyDone
	case streamErr = <-copyDone:
		// Normal completion or client write error
		sp.connectionMgr.Close(sessionID)
		// Drain any remaining pipe data
		pr.CloseWithError(io.EOF)
	}

	return result, streamErr
}

// TokenCounter tracks token usage (kept for compatibility)
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
