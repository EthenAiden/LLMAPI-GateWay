package config

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

// Config 主配置结构体
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Redis          RedisConfig          `yaml:"redis"`
	Postgres       PostgresConfig       `yaml:"postgres"`
	Kafka          KafkaConfig          `yaml:"kafka"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	Logging        LoggingConfig        `yaml:"logging"`
	Monitoring     MonitoringConfig     `yaml:"monitoring"`
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// PostgresConfig PostgreSQL 数据库配置
type PostgresConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// KafkaConfig Kafka 配置
type KafkaConfig struct {
	Brokers     []string `yaml:"brokers"`
	Topic       string   `yaml:"topic"`
	Compression string   `yaml:"compression"`
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	WindowSize      time.Duration `yaml:"window_size"`
	ErrorThreshold  float64       `yaml:"error_threshold"`
	HalfOpenTimeout time.Duration `yaml:"half_open_timeout"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	User  RateLimitDimension `yaml:"user"`
	App   RateLimitDimension `yaml:"app"`
	Model RateLimitDimension `yaml:"model"`
}

// RateLimitDimension 限流维度配置
type RateLimitDimension struct {
	Capacity int     `yaml:"capacity"`
	Rate     float64 `yaml:"rate"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	PrometheusPort int `yaml:"prometheus_port"`
}

// Load 从 YAML 文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 应用环境变量覆盖
	applyEnvOverrides(&cfg)

	// 验证配置
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides 应用环境变量覆盖配置
func applyEnvOverrides(cfg *Config) {
	// Server
	if v := os.Getenv("SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Server.Port)
	}

	// Redis
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Redis.DB)
	}

	// Postgres
	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		cfg.Postgres.Host = v
	}
	if v := os.Getenv("POSTGRES_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Postgres.Port)
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		cfg.Postgres.User = v
	}
	if v := os.Getenv("POSTGRES_PASSWORD"); v != "" {
		cfg.Postgres.Password = v
	}
	if v := os.Getenv("POSTGRES_DATABASE"); v != "" {
		cfg.Postgres.Database = v
	}

	// Kafka
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		// 简单实现：假设用逗号分隔
		cfg.Kafka.Brokers = []string{v}
	}
	if v := os.Getenv("KAFKA_TOPIC"); v != "" {
		cfg.Kafka.Topic = v
	}

	// Logging
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
}

// validate 验证配置的有效性
func validate(cfg *Config) error {
	// Server 验证
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	// Redis 验证
	if cfg.Redis.Addr == "" {
		return fmt.Errorf("redis address is required")
	}

	// Postgres 验证
	if cfg.Postgres.Host == "" {
		return fmt.Errorf("postgres host is required")
	}
	if cfg.Postgres.Database == "" {
		return fmt.Errorf("postgres database is required")
	}

	// Kafka 验证
	if len(cfg.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka brokers are required")
	}
	if cfg.Kafka.Topic == "" {
		return fmt.Errorf("kafka topic is required")
	}

	// CircuitBreaker 验证
	if cfg.CircuitBreaker.ErrorThreshold <= 0 || cfg.CircuitBreaker.ErrorThreshold > 1 {
		return fmt.Errorf("circuit breaker error threshold must be between 0 and 1")
	}

	// RateLimit 验证
	if cfg.RateLimit.User.Capacity <= 0 {
		return fmt.Errorf("user rate limit capacity must be positive")
	}
	if cfg.RateLimit.App.Capacity <= 0 {
		return fmt.Errorf("app rate limit capacity must be positive")
	}
	if cfg.RateLimit.Model.Capacity <= 0 {
		return fmt.Errorf("model rate limit capacity must be positive")
	}

	return nil
}
