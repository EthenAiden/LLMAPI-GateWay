package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// 创建临时配置文件
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  pool_size: 100

postgres:
  host: "localhost"
  port: 5432
  user: "gateway"
  password: "password"
  database: "ai_gateway"
  max_open_conns: 50
  max_idle_conns: 10

kafka:
  brokers:
    - "localhost:9092"
  topic: "gateway-requests"
  compression: "snappy"

circuit_breaker:
  window_size: 10s
  error_threshold: 0.5
  half_open_timeout: 30s

rate_limit:
  user:
    capacity: 10000
    rate: 100
  app:
    capacity: 50000
    rate: 500
  model:
    capacity: 100000
    rate: 1000

logging:
  level: "info"
  format: "json"

monitoring:
  prometheus_port: 9090
`

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	// 测试加载配置
	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// 验证配置值
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected server host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected server port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}

	if cfg.Redis.Addr != "localhost:6379" {
		t.Errorf("expected redis addr localhost:6379, got %s", cfg.Redis.Addr)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("expected redis db 0, got %d", cfg.Redis.DB)
	}

	if cfg.Postgres.Host != "localhost" {
		t.Errorf("expected postgres host localhost, got %s", cfg.Postgres.Host)
	}
	if cfg.Postgres.Port != 5432 {
		t.Errorf("expected postgres port 5432, got %d", cfg.Postgres.Port)
	}

	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "localhost:9092" {
		t.Errorf("expected kafka brokers [localhost:9092], got %v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "gateway-requests" {
		t.Errorf("expected kafka topic gateway-requests, got %s", cfg.Kafka.Topic)
	}

	if cfg.CircuitBreaker.WindowSize != 10*time.Second {
		t.Errorf("expected circuit breaker window size 10s, got %v", cfg.CircuitBreaker.WindowSize)
	}
	if cfg.CircuitBreaker.ErrorThreshold != 0.5 {
		t.Errorf("expected circuit breaker error threshold 0.5, got %f", cfg.CircuitBreaker.ErrorThreshold)
	}

	if cfg.RateLimit.User.Capacity != 10000 {
		t.Errorf("expected user rate limit capacity 10000, got %d", cfg.RateLimit.User.Capacity)
	}
	if cfg.RateLimit.App.Rate != 500 {
		t.Errorf("expected app rate limit rate 500, got %f", cfg.RateLimit.App.Rate)
	}
}

func TestEnvOverrides(t *testing.T) {
	// 创建临时配置文件
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  pool_size: 100

postgres:
  host: "localhost"
  port: 5432
  user: "gateway"
  password: "password"
  database: "ai_gateway"
  max_open_conns: 50
  max_idle_conns: 10

kafka:
  brokers:
    - "localhost:9092"
  topic: "gateway-requests"
  compression: "snappy"

circuit_breaker:
  window_size: 10s
  error_threshold: 0.5
  half_open_timeout: 30s

rate_limit:
  user:
    capacity: 10000
    rate: 100
  app:
    capacity: 50000
    rate: 500
  model:
    capacity: 100000
    rate: 1000

logging:
  level: "info"
  format: "json"

monitoring:
  prometheus_port: 9090
`

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	// 设置环境变量
	os.Setenv("SERVER_HOST", "127.0.0.1")
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("REDIS_ADDR", "redis:6379")
	os.Setenv("POSTGRES_HOST", "postgres")
	os.Setenv("LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("SERVER_HOST")
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("REDIS_ADDR")
		os.Unsetenv("POSTGRES_HOST")
		os.Unsetenv("LOG_LEVEL")
	}()

	// 加载配置
	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// 验证环境变量覆盖
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected server host 127.0.0.1 (from env), got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected server port 9090 (from env), got %d", cfg.Server.Port)
	}
	if cfg.Redis.Addr != "redis:6379" {
		t.Errorf("expected redis addr redis:6379 (from env), got %s", cfg.Redis.Addr)
	}
	if cfg.Postgres.Host != "postgres" {
		t.Errorf("expected postgres host postgres (from env), got %s", cfg.Postgres.Host)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug (from env), got %s", cfg.Logging.Level)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Redis:  RedisConfig{Addr: "localhost:6379"},
				Postgres: PostgresConfig{
					Host:     "localhost",
					Database: "test",
				},
				Kafka: KafkaConfig{
					Brokers: []string{"localhost:9092"},
					Topic:   "test",
				},
				CircuitBreaker: CircuitBreakerConfig{
					ErrorThreshold: 0.5,
				},
				RateLimit: RateLimitConfig{
					User:  RateLimitDimension{Capacity: 100},
					App:   RateLimitDimension{Capacity: 100},
					Model: RateLimitDimension{Capacity: 100},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid server port",
			config: Config{
				Server: ServerConfig{Port: 0},
				Redis:  RedisConfig{Addr: "localhost:6379"},
				Postgres: PostgresConfig{
					Host:     "localhost",
					Database: "test",
				},
				Kafka: KafkaConfig{
					Brokers: []string{"localhost:9092"},
					Topic:   "test",
				},
				CircuitBreaker: CircuitBreakerConfig{
					ErrorThreshold: 0.5,
				},
				RateLimit: RateLimitConfig{
					User:  RateLimitDimension{Capacity: 100},
					App:   RateLimitDimension{Capacity: 100},
					Model: RateLimitDimension{Capacity: 100},
				},
			},
			wantErr: true,
		},
		{
			name: "missing redis addr",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Redis:  RedisConfig{Addr: ""},
				Postgres: PostgresConfig{
					Host:     "localhost",
					Database: "test",
				},
				Kafka: KafkaConfig{
					Brokers: []string{"localhost:9092"},
					Topic:   "test",
				},
				CircuitBreaker: CircuitBreakerConfig{
					ErrorThreshold: 0.5,
				},
				RateLimit: RateLimitConfig{
					User:  RateLimitDimension{Capacity: 100},
					App:   RateLimitDimension{Capacity: 100},
					Model: RateLimitDimension{Capacity: 100},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid circuit breaker threshold",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Redis:  RedisConfig{Addr: "localhost:6379"},
				Postgres: PostgresConfig{
					Host:     "localhost",
					Database: "test",
				},
				Kafka: KafkaConfig{
					Brokers: []string{"localhost:9092"},
					Topic:   "test",
				},
				CircuitBreaker: CircuitBreakerConfig{
					ErrorThreshold: 1.5,
				},
				RateLimit: RateLimitConfig{
					User:  RateLimitDimension{Capacity: 100},
					App:   RateLimitDimension{Capacity: 100},
					Model: RateLimitDimension{Capacity: 100},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	if err == nil {
		t.Error("expected error when loading non-existent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// 写入无效的 YAML
	if _, err := tmpFile.WriteString("invalid: yaml: content: ["); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	_, err = Load(tmpFile.Name())
	if err == nil {
		t.Error("expected error when loading invalid YAML")
	}
}
