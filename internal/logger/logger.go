package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger 初始化日志记录器
func InitLogger(level string, format string) (*zap.Logger, error) {
	// 解析日志级别
	zapLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// 根据格式选择配置
	var config zap.Config
	if format == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	config.Level = zap.NewAtomicLevelAt(zapLevel)

	// 创建日志记录器
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}

// InitLoggerWithSampling 初始化带采样的日志记录器
func InitLoggerWithSampling(level string, format string, sampleInitial int, sampleThereafter int) (*zap.Logger, error) {
	// 解析日志级别
	zapLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// 根据格式选择配置
	var config zap.Config
	if format == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	config.Level = zap.NewAtomicLevelAt(zapLevel)

	// 设置采样
	config.Sampling = &zap.SamplingConfig{
		Initial:    sampleInitial,
		Thereafter: sampleThereafter,
	}

	// 创建日志记录器
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}
