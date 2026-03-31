package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-ide-gateway/internal/config"
	"ai-ide-gateway/internal/logger"
	"ai-ide-gateway/internal/server"
	"ai-ide-gateway/internal/storage"

	"go.uber.org/zap"
)

func main() {
	// 加载配置
	cfg, err := config.Load("configs/config.example.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log, err := logger.InitLogger(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("starting ai-ide-gateway")

	// 初始化 Redis 客户端
	redisClient, err := storage.NewRedisClient(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Redis.PoolSize,
		log,
	)
	if err != nil {
		log.Fatal("failed to initialize Redis client", zap.Error(err))
	}
	defer redisClient.Close()

	// 初始化 Kafka 生产者
	kafkaProducer, err := storage.NewKafkaProducer(
		cfg.Kafka.Brokers,
		cfg.Kafka.Topic,
		cfg.Kafka.Compression,
		log,
	)
	if err != nil {
		log.Fatal("failed to initialize Kafka producer", zap.Error(err))
	}
	defer kafkaProducer.Close()

	// 初始化 PostgreSQL 客户端
	postgresClient, err := storage.NewPostgresClient(
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.Database,
		cfg.Postgres.MaxOpenConns,
		cfg.Postgres.MaxIdleConns,
		log,
	)
	if err != nil {
		log.Fatal("failed to initialize PostgreSQL client", zap.Error(err))
	}
	defer postgresClient.Close()

	// 初始化 HTTP 服务器
	httpServer := server.NewServer(
		cfg.Server.Host,
		cfg.Server.Port,
		log,
		cfg.Server.ReadTimeout,
		cfg.Server.WriteTimeout,
	)

	// 注册健康检查端点
	httpServer.RegisterHealthCheck()

	// 启动 HTTP 服务器
	if err := httpServer.Start(); err != nil {
		log.Fatal("failed to start HTTP server", zap.Error(err))
	}

	log.Info("gateway started successfully",
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port),
	)

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号
	<-sigChan
	log.Info("shutting down gracefully...")

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}

	log.Info("gateway stopped")
}
