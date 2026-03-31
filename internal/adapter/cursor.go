package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CursorAdapter Cursor 协议适配器
type CursorAdapter struct {
	httpClient *http.Client
}

// CursorRequest Cursor 请求格式
type CursorRequest struct {
	Model       string          `json:"model"`
	Messages    []CursorMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// CursorMessage Cursor 消息格式
type CursorMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CursorResponse Cursor 响应格式
type CursorResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []CursorChoice `json:"choices"`
	Usage   CursorUsage    `json:"usage"`
}

// CursorChoice Cursor 选择项
type CursorChoice struct {
	Index        int           `json:"index"`
	Message      CursorMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// CursorUsage Cursor Token 使用量
type CursorUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CursorStreamChunk Cursor 流式数据块
type CursorStreamChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []CursorStreamDelta `json:"choices"`
	Usage   *CursorUsage        `json:"usage,omitempty"`
}

// CursorStreamDelta Cursor 流式增量
type CursorStreamDelta struct {
	Index int `json:"index"`
	Delta struct {
		Role    string `json:"role,omitempty"`
		Content string `json:"content,omitempty"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// NewCursorAdapter 创建 Cursor 适配器
func NewCursorAdapter() *CursorAdapter {
	return &CursorAdapter{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ConvertRequest 将 OpenAI 请求转换为 Cursor 请求
func (a *CursorAdapter) ConvertRequest(ctx context.Context, req *OpenAIRequest) (*UpstreamRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// 转换消息格式
	messages := make([]CursorMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = CursorMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// 构建 Cursor 请求
	cursorReq := &CursorRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}

	// 序列化请求体
	body, err := json.Marshal(cursorReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cursor request: %w", err)
	}

	return &UpstreamRequest{
		URL:    "https://api.cursor.sh/v1/chat/completions",
		Method: "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	}, nil
}

// ConvertResponse 将 Cursor 响应转换为 OpenAI 响应
func (a *CursorAdapter) ConvertResponse(ctx context.Context, resp *UpstreamResponse) (*OpenAIResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("response cannot be nil")
	}

	var cursorResp CursorResponse
	if err := json.Unmarshal(resp.Body, &cursorResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor response: %w", err)
	}

	// 转换选择项
	choices := make([]Choice, len(cursorResp.Choices))
	for i, choice := range cursorResp.Choices {
		choices[i] = Choice{
			Index: choice.Index,
			Message: Message{
				Role:    choice.Message.Role,
				Content: choice.Message.Content,
			},
			FinishReason: choice.FinishReason,
		}
	}

	return &OpenAIResponse{
		ID:      cursorResp.ID,
		Object:  cursorResp.Object,
		Created: cursorResp.Created,
		Model:   cursorResp.Model,
		Choices: choices,
		Usage: Usage{
			PromptTokens:     cursorResp.Usage.PromptTokens,
			CompletionTokens: cursorResp.Usage.CompletionTokens,
			TotalTokens:      cursorResp.Usage.TotalTokens,
		},
	}, nil
}

// ConvertStreamChunk 转换 Cursor 流式数据块
func (a *CursorAdapter) ConvertStreamChunk(ctx context.Context, chunk []byte) (*OpenAIStreamChunk, error) {
	if len(chunk) == 0 {
		return nil, fmt.Errorf("chunk cannot be empty")
	}

	var cursorChunk CursorStreamChunk
	if err := json.Unmarshal(chunk, &cursorChunk); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor stream chunk: %w", err)
	}

	// 转换选择项
	choices := make([]StreamDelta, len(cursorChunk.Choices))
	for i, choice := range cursorChunk.Choices {
		choices[i] = StreamDelta{
			Index: choice.Index,
			Delta: struct {
				Role    string `json:"role,omitempty"`
				Content string `json:"content,omitempty"`
			}{
				Role:    choice.Delta.Role,
				Content: choice.Delta.Content,
			},
			FinishReason: choice.FinishReason,
		}
	}

	return &OpenAIStreamChunk{
		ID:      cursorChunk.ID,
		Object:  cursorChunk.Object,
		Created: cursorChunk.Created,
		Model:   cursorChunk.Model,
		Choices: choices,
	}, nil
}

// GetProviderType 返回提供商类型
func (a *CursorAdapter) GetProviderType() ProviderType {
	return ProviderCursor
}
