package replay

import (
	"context"
	"encoding/json"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// ReplayCollector collects request metadata for replay
type ReplayCollector struct {
	producer *kafka.Writer
	topic    string
	logger   *zap.Logger
}

// NewReplayCollector creates a new replay collector
func NewReplayCollector(brokers []string, topic string, logger *zap.Logger) *ReplayCollector {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  brokers,
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	})

	return &ReplayCollector{
		producer: writer,
		topic:    topic,
		logger:   logger,
	}
}

// Collect collects request metadata asynchronously
func (rc *ReplayCollector) Collect(ctx context.Context, metadata *RequestMetadata) {
	// Async collection - don't block main request path
	go func() {
		data, err := json.Marshal(metadata)
		if err != nil {
			rc.logger.Error("failed to marshal metadata", zap.Error(err))
			return
		}

		message := kafka.Message{
			Key:   []byte(metadata.RequestID),
			Value: data,
		}

		if err := rc.producer.WriteMessages(ctx, message); err != nil {
			rc.logger.Error("failed to write message to kafka", zap.Error(err))
		}
	}()
}

// Close closes the collector
func (rc *ReplayCollector) Close() error {
	return rc.producer.Close()
}
