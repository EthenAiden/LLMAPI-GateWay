package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// KiroAdapter Kiro 协议适配器
type KiroAdapter struct {
	httpClient   *http.Client
	tokenManager *KiroTokenManager
}

// KiroTokenManager Kiro OAuth Token 管理器
type KiroTokenManager struct {
	mu           sync.RWMutex
	token        string
	tokenExpiry  time.Time
	refreshToken string
}

// KiroRequest Kiro 请求格式
type KiroRequest struct {
	Model    string        `json:"model"`
	Messages []KiroMessage `json:"messages"`
	Config   KiroConfig    `json:"config,omitempty"`
	Stream   bool          `json:"stream,omitempty"`
}

// KiroMessage Kiro 消息格式
type KiroMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// KiroConfig Kiro 配置
type KiroConfig struct {
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// KiroResponse Kiro 响应格式
type KiroResponse struct {
	ID      string        `json:"id"`
	Type    string        `json:"type"`
	Content []KiroContent `json:"content"`
	Usage   KiroUsage     `json:"usage"`
}

// KiroContent Kiro 内容块
type KiroContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// KiroUsage Kiro Token 使用量
type KiroUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// KiroStreamChunk Kiro 流式数据块
type KiroStreamChunk struct {
	Type  string     `json:"type"`
	Delta *KiroDelta `json:"delta,omitempty"`
	Usage *KiroUsage `json:"usage,omitempty"`
}

// KiroDelta Kiro 流式增量
type KiroDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// NewKiroAdapter 创建 Kiro 适配器
func NewKiroAdapter() *KiroAdapter {
	return &KiroAdapter{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		tokenManager: &KiroTokenManager{},
	}
}

// GetValidToken 获取有效的 OAuth Token
func (tm *KiroTokenManager) GetValidToken(ctx context.Context) (string, error) {
	tm.mu.RLock()
	if tm.token != "" && time.Now().Before(tm.tokenExpiry) {
		defer tm.mu.RUnlock()
		return tm.token, nil
	}
	tm.mu.RUnlock()

	// 需要刷新 Token（实际实现中应调用 OAuth 端点）
	// 这里为演示目的返回占位符
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.token = "kiro_token_" + fmt.Sprintf("%d", time.Now().Unix())
	tm.tokenExpiry = time.Now().Add(1 * time.Hour)
	return tm.token, nil
}

// ConvertRequest 将 OpenAI 请求转换为 Kiro 请求
func (a *KiroAdapter) ConvertRequest(ctx context.Context, req *OpenAIRequest) (*UpstreamRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// 获取有效的 Token
	token, err := a.tokenManager.GetValidToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get kiro token: %w", err)
	}

	// 转换消息格式
	messages := make([]KiroMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = KiroMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// 构建 Kiro 请求
	kiroReq := &KiroRequest{
		Model:    req.Model,
		Messages: messages,
		Config: KiroConfig{
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
		},
		Stream: req.Stream,
	}

	// 序列化请求体
	body, err := json.Marshal(kiroReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal kiro request: %w", err)
	}

	return &UpstreamRequest{
		URL:    "https://api.kiro.ai/v1/messages",
		Method: "POST",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + token,
			"X-Kiro-Region": "us-east-1",
		},
		Body: body,
	}, nil
}

// ConvertResponse 将 Kiro 响应转换为 OpenAI 响应
func (a *KiroAdapter) ConvertResponse(ctx context.Context, resp *UpstreamResponse) (*OpenAIResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("response cannot be nil")
	}

	var kiroResp KiroResponse
	if err := json.Unmarshal(resp.Body, &kiroResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kiro response: %w", err)
	}

	// 提取文本内容
	content := ""
	for _, c := range kiroResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	// 构建 OpenAI 响应
	return &OpenAIResponse{
		ID:      kiroResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "kiro",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     kiroResp.Usage.InputTokens,
			CompletionTokens: kiroResp.Usage.OutputTokens,
			TotalTokens:      kiroResp.Usage.InputTokens + kiroResp.Usage.OutputTokens,
		},
	}, nil
}

// ConvertStreamChunk 转换 Kiro 流式数据块
func (a *KiroAdapter) ConvertStreamChunk(ctx context.Context, chunk []byte) (*OpenAIStreamChunk, error) {
	if len(chunk) == 0 {
		return nil, fmt.Errorf("chunk cannot be empty")
	}

	var kiroChunk KiroStreamChunk
	if err := json.Unmarshal(chunk, &kiroChunk); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kiro stream chunk: %w", err)
	}

	// 构建 OpenAI 流式响应
	openaiChunk := &OpenAIStreamChunk{
		ID:      fmt.Sprintf("kiro-%d", time.Now().UnixNano()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "kiro",
		Choices: []StreamDelta{},
	}

	// 处理不同类型的数据块
	if kiroChunk.Delta != nil && kiroChunk.Delta.Type == "text_delta" {
		openaiChunk.Choices = []StreamDelta{
			{
				Index: 0,
				Delta: struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				}{
					Content: kiroChunk.Delta.Text,
				},
			},
		}
	}

	return openaiChunk, nil
}

// GetProviderType 返回提供商类型
func (a *KiroAdapter) GetProviderType() ProviderType {
	return ProviderKiro
}
