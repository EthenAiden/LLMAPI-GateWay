package adapter

import "context"

// ProviderType 上游服务提供商类型
type ProviderType string

const (
	ProviderCursor      ProviderType = "cursor"
	ProviderKiro        ProviderType = "kiro"
	ProviderAntigravity ProviderType = "antigravity"
)

// ProtocolAdapter 定义协议适配器接口
type ProtocolAdapter interface {
	// ConvertRequest 将 OpenAI 请求转换为上游协议请求
	ConvertRequest(ctx context.Context, req *OpenAIRequest) (*UpstreamRequest, error)

	// ConvertResponse 将上游响应转换为 OpenAI 响应
	ConvertResponse(ctx context.Context, resp *UpstreamResponse) (*OpenAIResponse, error)

	// ConvertStreamChunk 转换流式响应数据块
	ConvertStreamChunk(ctx context.Context, chunk []byte) (*OpenAIStreamChunk, error)

	// GetProviderType 返回上游服务提供商类型
	GetProviderType() ProviderType
}

// OpenAIRequest OpenAI 请求格式
type OpenAIRequest struct {
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	Temperature   float64        `json:"temperature,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

// StreamOptions controls SSE stream behavior
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// Message 消息格式
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// UpstreamRequest 上游请求
type UpstreamRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    interface{}
}

// UpstreamResponse 上游响应
type UpstreamResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// OpenAIResponse OpenAI 响应格式
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice 选择项
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage Token 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamChunk 流式响应数据块
type OpenAIStreamChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []StreamDelta `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"` // populated in the final chunk when stream_options.include_usage=true
}

// StreamDelta 流式增量
type StreamDelta struct {
	Index int `json:"index"`
	Delta struct {
		Role    string `json:"role,omitempty"`
		Content string `json:"content,omitempty"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
}
