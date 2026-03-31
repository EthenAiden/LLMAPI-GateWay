package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AntigravityAdapter Antigravity 协议适配器
type AntigravityAdapter struct {
	httpClient *http.Client
}

// AntigravityRequest Antigravity 请求格式
type AntigravityRequest struct {
	Model       string               `json:"model"`
	Messages    []AntigravityMessage `json:"messages"`
	Temperature float64              `json:"temperature,omitempty"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
}

// AntigravityMessage Antigravity 消息格式
type AntigravityMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AntigravityResponse Antigravity 响应格式
type AntigravityResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []AntigravityChoice `json:"choices"`
	Usage   AntigravityUsage    `json:"usage"`
}

// AntigravityChoice Antigravity 选择项
type AntigravityChoice struct {
	Index        int                `json:"index"`
	Message      AntigravityMessage `json:"message"`
	FinishReason string             `json:"finish_reason"`
}

// AntigravityUsage Antigravity Token 使用量
type AntigravityUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// AntigravityStreamChunk Antigravity 流式数据块
type AntigravityStreamChunk struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []AntigravityStreamDelta `json:"choices"`
	Usage   *AntigravityUsage        `json:"usage,omitempty"`
}

// AntigravityStreamDelta Antigravity 流式增量
type AntigravityStreamDelta struct {
	Index int `json:"index"`
	Delta struct {
		Role    string `json:"role,omitempty"`
		Content string `json:"content,omitempty"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// NewAntigravityAdapter 创建 Antigravity 适配器
func NewAntigravityAdapter() *AntigravityAdapter {
	return &AntigravityAdapter{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ConvertRequest 将 OpenAI 请求转换为 Antigravity 请求
func (a *AntigravityAdapter) ConvertRequest(ctx context.Context, req *OpenAIRequest) (*UpstreamRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// 转换消息格式
	messages := make([]AntigravityMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = AntigravityMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// 构建 Antigravity 请求
	antigravityReq := &AntigravityRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}

	// 序列化请求体
	body, err := json.Marshal(antigravityReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal antigravity request: %w", err)
	}

	return &UpstreamRequest{
		URL:    "https://api.antigravity.ai/v1/chat/completions",
		Method: "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	}, nil
}

// ConvertResponse 将 Antigravity 响应转换为 OpenAI 响应
func (a *AntigravityAdapter) ConvertResponse(ctx context.Context, resp *UpstreamResponse) (*OpenAIResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("response cannot be nil")
	}

	var antigravityResp AntigravityResponse
	if err := json.Unmarshal(resp.Body, &antigravityResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal antigravity response: %w", err)
	}

	// 转换选择项
	choices := make([]Choice, len(antigravityResp.Choices))
	for i, choice := range antigravityResp.Choices {
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
		ID:      antigravityResp.ID,
		Object:  antigravityResp.Object,
		Created: antigravityResp.Created,
		Model:   antigravityResp.Model,
		Choices: choices,
		Usage: Usage{
			PromptTokens:     antigravityResp.Usage.PromptTokens,
			CompletionTokens: antigravityResp.Usage.CompletionTokens,
			TotalTokens:      antigravityResp.Usage.TotalTokens,
		},
	}, nil
}

// ConvertStreamChunk 转换 Antigravity 流式数据块
func (a *AntigravityAdapter) ConvertStreamChunk(ctx context.Context, chunk []byte) (*OpenAIStreamChunk, error) {
	if len(chunk) == 0 {
		return nil, fmt.Errorf("chunk cannot be empty")
	}

	var antigravityChunk AntigravityStreamChunk
	if err := json.Unmarshal(chunk, &antigravityChunk); err != nil {
		return nil, fmt.Errorf("failed to unmarshal antigravity stream chunk: %w", err)
	}

	// 转换选择项
	choices := make([]StreamDelta, len(antigravityChunk.Choices))
	for i, choice := range antigravityChunk.Choices {
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
		ID:      antigravityChunk.ID,
		Object:  antigravityChunk.Object,
		Created: antigravityChunk.Created,
		Model:   antigravityChunk.Model,
		Choices: choices,
	}, nil
}

// GetProviderType 返回提供商类型
func (a *AntigravityAdapter) GetProviderType() ProviderType {
	return ProviderAntigravity
}
