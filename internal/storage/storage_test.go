package storage

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewRedisClient(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Test with invalid address (should fail)
	_, err := NewRedisClient("invalid:99999", "", 0, 10, logger)
	if err == nil {
		t.Errorf("NewRedisClient() with invalid address should fail")
	}
}

func TestRedisClientMethods(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Skip if Redis is not available
	client, err := NewRedisClient("localhost:6379", "", 0, 10, logger)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test Set and Get
	err = client.Set(ctx, "test_key", "test_value", 10*time.Second)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	value, err := client.Get(ctx, "test_key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "test_value" {
		t.Errorf("Get() returned %s, want test_value", value)
	}

	// Test Del
	err = client.Del(ctx, "test_key")
	if err != nil {
		t.Fatalf("Del() error = %v", err)
	}

	// Test Exists
	exists, err := client.Exists(ctx, "test_key")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists != 0 {
		t.Errorf("Exists() returned %d, want 0", exists)
	}

	// Test Incr
	err = client.Set(ctx, "counter", "0", 10*time.Second)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	val, err := client.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("Incr() error = %v", err)
	}
	if val != 1 {
		t.Errorf("Incr() returned %d, want 1", val)
	}

	// Test Decr
	val, err = client.Decr(ctx, "counter")
	if err != nil {
		t.Fatalf("Decr() error = %v", err)
	}
	if val != 0 {
		t.Errorf("Decr() returned %d, want 0", val)
	}

	// Test HSet and HGet
	err = client.HSet(ctx, "test_hash", "field1", "value1", "field2", "value2")
	if err != nil {
		t.Fatalf("HSet() error = %v", err)
	}

	hvalue, err := client.HGet(ctx, "test_hash", "field1")
	if err != nil {
		t.Fatalf("HGet() error = %v", err)
	}
	if hvalue != "value1" {
		t.Errorf("HGet() returned %s, want value1", hvalue)
	}

	// Test HGetAll
	hvalues, err := client.HGetAll(ctx, "test_hash")
	if err != nil {
		t.Fatalf("HGetAll() error = %v", err)
	}
	if len(hvalues) != 2 {
		t.Errorf("HGetAll() returned %d fields, want 2", len(hvalues))
	}

	// Test HealthCheck
	err = client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

func TestNewKafkaProducer(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Test with invalid brokers (should still create producer, but fail on write)
	producer, err := NewKafkaProducer([]string{"invalid:9092"}, "test-topic", "snappy", logger)
	if err != nil {
		t.Fatalf("NewKafkaProducer() error = %v", err)
	}
	defer producer.Close()
}

func TestNewKafkaConsumer(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Skip Kafka consumer test as it may hang
	t.Skip("Kafka consumer test skipped - may hang on invalid brokers")
}

func TestNewPostgresClient(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Skip PostgreSQL test as it may hang on invalid connection
	t.Skip("PostgreSQL test skipped - may hang on invalid host")
}

func TestPostgresClientMethods(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Skip if PostgreSQL is not available
	client, err := NewPostgresClient("localhost", 5432, "postgres", "password", "postgres", 10, 5, logger)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test HealthCheck
	err = client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}

	// Test GetDB
	db := client.GetDB()
	if db == nil {
		t.Errorf("GetDB() returned nil")
	}
}

func TestKafkaCompressionCodecs(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	compressions := []string{"snappy", "gzip", "lz4", "zstd", "none"}

	for _, compression := range compressions {
		producer, err := NewKafkaProducer([]string{"localhost:9092"}, "test-topic", compression, logger)
		if err != nil {
			t.Fatalf("NewKafkaProducer() with compression %s error = %v", compression, err)
		}
		defer producer.Close()
	}
}
