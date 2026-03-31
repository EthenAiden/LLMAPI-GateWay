package storage

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaProducer Kafka 生产者包装
type KafkaProducer struct {
	writer *kafka.Writer
	logger *zap.Logger
}

// NewKafkaProducer 创建新的 Kafka 生产者
func NewKafkaProducer(brokers []string, topic string, compression string, logger *zap.Logger) (*KafkaProducer, error) {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll,
	}

	logger.Info("created Kafka producer", zap.Strings("brokers", brokers), zap.String("topic", topic), zap.String("compression", compression))

	return &KafkaProducer{
		writer: writer,
		logger: logger,
	}, nil
}

// WriteMessage 写入消息
func (kp *KafkaProducer) WriteMessage(ctx context.Context, key string, value []byte) error {
	msg := kafka.Message{
		Key:   []byte(key),
		Value: value,
	}

	return kp.writer.WriteMessages(ctx, msg)
}

// WriteMessages 写入多条消息
func (kp *KafkaProducer) WriteMessages(ctx context.Context, messages []kafka.Message) error {
	return kp.writer.WriteMessages(ctx, messages...)
}

// Close 关闭生产者
func (kp *KafkaProducer) Close() error {
	return kp.writer.Close()
}

// KafkaConsumer Kafka 消费者包装
type KafkaConsumer struct {
	reader *kafka.Reader
	logger *zap.Logger
}

// NewKafkaConsumer 创建新的 Kafka 消费者
func NewKafkaConsumer(brokers []string, topic string, groupID string, logger *zap.Logger) (*KafkaConsumer, error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		CommitInterval: 1000, // 1 秒提交一次
		StartOffset:    kafka.LastOffset,
	})

	logger.Info("created Kafka consumer", zap.Strings("brokers", brokers), zap.String("topic", topic), zap.String("groupID", groupID))

	return &KafkaConsumer{
		reader: reader,
		logger: logger,
	}, nil
}

// ReadMessage 读取单条消息
func (kc *KafkaConsumer) ReadMessage(ctx context.Context) (kafka.Message, error) {
	return kc.reader.ReadMessage(ctx)
}

// ReadMessages 读取多条消息
func (kc *KafkaConsumer) ReadMessages(ctx context.Context, maxMessages int) ([]kafka.Message, error) {
	messages := make([]kafka.Message, 0, maxMessages)

	for i := 0; i < maxMessages; i++ {
		msg, err := kc.reader.ReadMessage(ctx)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				break
			}
			return nil, fmt.Errorf("failed to read message: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// CommitMessages 提交消息偏移量
func (kc *KafkaConsumer) CommitMessages(ctx context.Context, messages ...kafka.Message) error {
	return kc.reader.CommitMessages(ctx, messages...)
}

// Close 关闭消费者
func (kc *KafkaConsumer) Close() error {
	return kc.reader.Close()
}
