package adapter

import (
	"context"
	"encoding/json"
	"testing"
)

// TestAdapterFactory tests the adapter factory
func TestAdapterFactory(t *testing.T) {
	factory := NewAdapterFactory()

	tests := []struct {
		name      string
		provider  ProviderType
		wantError bool
	}{
		{
			name:      "get cursor adapter",
			provider:  ProviderCursor,
			wantError: false,
		},
		{
			name:      "get kiro adapter",
			provider:  ProviderKiro,
			wantError: false,
		},
		{
			name:      "get antigravity adapter",
			provider:  ProviderAntigravity,
			wantError: false,
		},
		{
			name:      "get unsupported adapter",
			provider:  "unsupported",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := factory.GetAdapter(tt.provider)
			if (err != nil) != tt.wantError {
				t.Errorf("GetAdapter() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && adapter == nil {
				t.Errorf("GetAdapter() returned nil adapter")
			}
		})
	}
}

// TestCursorAdapterConvertRequest tests Cursor request conversion
func TestCursorAdapterConvertRequest(t *testing.T) {
	adapter := NewCursorAdapter()
	ctx := context.Background()

	req := &OpenAIRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
		Stream:      false,
	}

	upstreamReq, err := adapter.ConvertRequest(ctx, req)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	if upstreamReq.URL != "https://api.cursor.sh/v1/chat/completions" {
		t.Errorf("URL = %s, want https://api.cursor.sh/v1/chat/completions", upstreamReq.URL)
	}

	if upstreamReq.Method != "POST" {
		t.Errorf("Method = %s, want POST", upstreamReq.Method)
	}

	// Verify request body
	var cursorReq CursorRequest
	if err := json.Unmarshal(upstreamReq.Body.([]byte), &cursorReq); err != nil {
		t.Fatalf("Failed to unmarshal request body: %v", err)
	}

	if cursorReq.Model != "gpt-4" {
		t.Errorf("Model = %s, want gpt-4", cursorReq.Model)
	}

	if len(cursorReq.Messages) != 1 {
		t.Errorf("Messages length = %d, want 1", len(cursorReq.Messages))
	}
}

// TestCursorAdapterConvertResponse tests Cursor response conversion
func TestCursorAdapterConvertResponse(t *testing.T) {
	adapter := NewCursorAdapter()
	ctx := context.Background()

	cursorResp := CursorResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []CursorChoice{
			{
				Index: 0,
				Message: CursorMessage{
					Role:    "assistant",
					Content: "Hello!",
				},
				FinishReason: "stop",
			},
		},
		Usage: CursorUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	respBody, _ := json.Marshal(cursorResp)
	upstreamResp := &UpstreamResponse{
		StatusCode: 200,
		Body:       respBody,
	}

	openaiResp, err := adapter.ConvertResponse(ctx, upstreamResp)
	if err != nil {
		t.Fatalf("ConvertResponse() error = %v", err)
	}

	if openaiResp.ID != "chatcmpl-123" {
		t.Errorf("ID = %s, want chatcmpl-123", openaiResp.ID)
	}

	if len(openaiResp.Choices) != 1 {
		t.Errorf("Choices length = %d, want 1", len(openaiResp.Choices))
	}

	if openaiResp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", openaiResp.Usage.TotalTokens)
	}
}

// TestCursorAdapterConvertStreamChunk tests Cursor stream chunk conversion
func TestCursorAdapterConvertStreamChunk(t *testing.T) {
	adapter := NewCursorAdapter()
	ctx := context.Background()

	chunk := CursorStreamChunk{
		ID:      "chatcmpl-123",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []CursorStreamDelta{
			{
				Index: 0,
				Delta: struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				}{
					Content: "Hello",
				},
			},
		},
	}

	chunkBody, _ := json.Marshal(chunk)
	openaiChunk, err := adapter.ConvertStreamChunk(ctx, chunkBody)
	if err != nil {
		t.Fatalf("ConvertStreamChunk() error = %v", err)
	}

	if openaiChunk.ID != "chatcmpl-123" {
		t.Errorf("ID = %s, want chatcmpl-123", openaiChunk.ID)
	}

	if len(openaiChunk.Choices) != 1 {
		t.Errorf("Choices length = %d, want 1", len(openaiChunk.Choices))
	}
}

// TestKiroAdapterConvertRequest tests Kiro request conversion
func TestKiroAdapterConvertRequest(t *testing.T) {
	adapter := NewKiroAdapter()
	ctx := context.Background()

	req := &OpenAIRequest{
		Model: "claude-3",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
		Stream:      false,
	}

	upstreamReq, err := adapter.ConvertRequest(ctx, req)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	if upstreamReq.URL != "https://api.kiro.ai/v1/messages" {
		t.Errorf("URL = %s, want https://api.kiro.ai/v1/messages", upstreamReq.URL)
	}

	if upstreamReq.Method != "POST" {
		t.Errorf("Method = %s, want POST", upstreamReq.Method)
	}

	// Verify headers contain authorization
	if _, ok := upstreamReq.Headers["Authorization"]; !ok {
		t.Errorf("Authorization header missing")
	}
}

// TestKiroAdapterConvertResponse tests Kiro response conversion
func TestKiroAdapterConvertResponse(t *testing.T) {
	adapter := NewKiroAdapter()
	ctx := context.Background()

	kiroResp := KiroResponse{
		ID:   "msg-123",
		Type: "message",
		Content: []KiroContent{
			{
				Type: "text",
				Text: "Hello!",
			},
		},
		Usage: KiroUsage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}

	respBody, _ := json.Marshal(kiroResp)
	upstreamResp := &UpstreamResponse{
		StatusCode: 200,
		Body:       respBody,
	}

	openaiResp, err := adapter.ConvertResponse(ctx, upstreamResp)
	if err != nil {
		t.Fatalf("ConvertResponse() error = %v", err)
	}

	if openaiResp.ID != "msg-123" {
		t.Errorf("ID = %s, want msg-123", openaiResp.ID)
	}

	if len(openaiResp.Choices) != 1 {
		t.Errorf("Choices length = %d, want 1", len(openaiResp.Choices))
	}

	if openaiResp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", openaiResp.Usage.PromptTokens)
	}
}

// TestAntigravityAdapterConvertRequest tests Antigravity request conversion
func TestAntigravityAdapterConvertRequest(t *testing.T) {
	adapter := NewAntigravityAdapter()
	ctx := context.Background()

	req := &OpenAIRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
		Stream:      false,
	}

	upstreamReq, err := adapter.ConvertRequest(ctx, req)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	if upstreamReq.URL != "https://api.antigravity.ai/v1/chat/completions" {
		t.Errorf("URL = %s, want https://api.antigravity.ai/v1/chat/completions", upstreamReq.URL)
	}

	if upstreamReq.Method != "POST" {
		t.Errorf("Method = %s, want POST", upstreamReq.Method)
	}
}

// TestAntigravityAdapterConvertResponse tests Antigravity response conversion
func TestAntigravityAdapterConvertResponse(t *testing.T) {
	adapter := NewAntigravityAdapter()
	ctx := context.Background()

	antigravityResp := AntigravityResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []AntigravityChoice{
			{
				Index: 0,
				Message: AntigravityMessage{
					Role:    "assistant",
					Content: "Hello!",
				},
				FinishReason: "stop",
			},
		},
		Usage: AntigravityUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	respBody, _ := json.Marshal(antigravityResp)
	upstreamResp := &UpstreamResponse{
		StatusCode: 200,
		Body:       respBody,
	}

	openaiResp, err := adapter.ConvertResponse(ctx, upstreamResp)
	if err != nil {
		t.Fatalf("ConvertResponse() error = %v", err)
	}

	if openaiResp.ID != "chatcmpl-123" {
		t.Errorf("ID = %s, want chatcmpl-123", openaiResp.ID)
	}

	if openaiResp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", openaiResp.Usage.TotalTokens)
	}
}

// TestErrorHandling tests error handling
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		err            *ConversionError
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "invalid request error",
			err:            NewInvalidRequestError("invalid model"),
			expectedStatus: 400,
			expectedType:   "invalid_request_error",
		},
		{
			name:           "authentication error",
			err:            NewAuthenticationError("invalid api key"),
			expectedStatus: 401,
			expectedType:   "authentication_error",
		},
		{
			name:           "rate limit error",
			err:            NewRateLimitError("rate limit exceeded"),
			expectedStatus: 429,
			expectedType:   "rate_limit_error",
		},
		{
			name:           "server error",
			err:            NewServerError("internal error"),
			expectedStatus: 500,
			expectedType:   "server_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.StatusCode != tt.expectedStatus {
				t.Errorf("StatusCode = %d, want %d", tt.err.StatusCode, tt.expectedStatus)
			}

			if tt.err.Type != tt.expectedType {
				t.Errorf("Type = %s, want %s", tt.err.Type, tt.expectedType)
			}

			// Test JSON conversion
			jsonData := tt.err.ToJSON()
			if len(jsonData) == 0 {
				t.Errorf("ToJSON() returned empty data")
			}

			var resp ErrorResponse
			if err := json.Unmarshal(jsonData, &resp); err != nil {
				t.Errorf("Failed to unmarshal error response: %v", err)
			}

			if resp.Error.Type != tt.expectedType {
				t.Errorf("Error type in JSON = %s, want %s", resp.Error.Type, tt.expectedType)
			}
		})
	}
}

// TestNilInputHandling tests handling of nil inputs
func TestNilInputHandling(t *testing.T) {
	adapter := NewCursorAdapter()
	ctx := context.Background()

	// Test nil request
	_, err := adapter.ConvertRequest(ctx, nil)
	if err == nil {
		t.Errorf("ConvertRequest(nil) should return error")
	}

	// Test nil response
	_, err = adapter.ConvertResponse(ctx, nil)
	if err == nil {
		t.Errorf("ConvertResponse(nil) should return error")
	}

	// Test empty chunk
	_, err = adapter.ConvertStreamChunk(ctx, []byte{})
	if err == nil {
		t.Errorf("ConvertStreamChunk(empty) should return error")
	}
}

// TestProviderType tests provider type identification
func TestProviderType(t *testing.T) {
	tests := []struct {
		name     string
		adapter  ProtocolAdapter
		expected ProviderType
	}{
		{
			name:     "cursor adapter",
			adapter:  NewCursorAdapter(),
			expected: ProviderCursor,
		},
		{
			name:     "kiro adapter",
			adapter:  NewKiroAdapter(),
			expected: ProviderKiro,
		},
		{
			name:     "antigravity adapter",
			adapter:  NewAntigravityAdapter(),
			expected: ProviderAntigravity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.adapter.GetProviderType() != tt.expected {
				t.Errorf("GetProviderType() = %s, want %s", tt.adapter.GetProviderType(), tt.expected)
			}
		})
	}
}
