package adapter

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorResponse OpenAI 标准错误响应格式
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

// ConversionError 协议转换错误
type ConversionError struct {
	StatusCode int
	Message    string
	Type       string
	Code       string
}

// Error 实现 error 接口
func (e *ConversionError) Error() string {
	return fmt.Sprintf("conversion error: %s (code: %s)", e.Message, e.Code)
}

// ToErrorResponse 转换为 OpenAI 错误响应
func (e *ConversionError) ToErrorResponse() *ErrorResponse {
	return &ErrorResponse{
		Error: ErrorDetail{
			Message: e.Message,
			Type:    e.Type,
			Code:    e.Code,
		},
	}
}

// ToJSON 转换为 JSON 字节
func (e *ConversionError) ToJSON() []byte {
	resp := e.ToErrorResponse()
	data, _ := json.Marshal(resp)
	return data
}

// NewConversionError 创建转换错误
func NewConversionError(statusCode int, message, errorType, code string) *ConversionError {
	return &ConversionError{
		StatusCode: statusCode,
		Message:    message,
		Type:       errorType,
		Code:       code,
	}
}

// NewInvalidRequestError 创建无效请求错误
func NewInvalidRequestError(message string) *ConversionError {
	return NewConversionError(
		http.StatusBadRequest,
		message,
		"invalid_request_error",
		"invalid_request",
	)
}

// NewAuthenticationError 创建认证错误
func NewAuthenticationError(message string) *ConversionError {
	return NewConversionError(
		http.StatusUnauthorized,
		message,
		"authentication_error",
		"invalid_api_key",
	)
}

// NewRateLimitError 创建限流错误
func NewRateLimitError(message string) *ConversionError {
	return NewConversionError(
		http.StatusTooManyRequests,
		message,
		"rate_limit_error",
		"rate_limit_exceeded",
	)
}

// NewServerError 创建服务器错误
func NewServerError(message string) *ConversionError {
	return NewConversionError(
		http.StatusInternalServerError,
		message,
		"server_error",
		"internal_error",
	)
}

// NewUpstreamError 创建上游服务错误
func NewUpstreamError(statusCode int, message string) *ConversionError {
	errorType := "server_error"
	code := "upstream_error"

	// 根据状态码调整错误类型
	switch statusCode {
	case http.StatusUnauthorized:
		errorType = "authentication_error"
		code = "invalid_api_key"
	case http.StatusForbidden:
		errorType = "authentication_error"
		code = "permission_denied"
	case http.StatusTooManyRequests:
		errorType = "rate_limit_error"
		code = "rate_limit_exceeded"
	case http.StatusBadRequest:
		errorType = "invalid_request_error"
		code = "invalid_request"
	case http.StatusServiceUnavailable:
		errorType = "server_error"
		code = "service_unavailable"
	}

	return NewConversionError(statusCode, message, errorType, code)
}

// NewParseError 创建解析错误
func NewParseError(message string) *ConversionError {
	return NewConversionError(
		http.StatusInternalServerError,
		fmt.Sprintf("failed to parse response: %s", message),
		"server_error",
		"parse_error",
	)
}

// NewMarshalError 创建序列化错误
func NewMarshalError(message string) *ConversionError {
	return NewConversionError(
		http.StatusInternalServerError,
		fmt.Sprintf("failed to marshal request: %s", message),
		"server_error",
		"marshal_error",
	)
}
