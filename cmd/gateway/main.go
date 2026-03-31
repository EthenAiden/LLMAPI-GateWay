package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-ide-gateway/internal/adapter"
	"ai-ide-gateway/internal/auth"
	"ai-ide-gateway/internal/breaker"
	"ai-ide-gateway/internal/config"
	"ai-ide-gateway/internal/handler"
	"ai-ide-gateway/internal/logger"
	"ai-ide-gateway/internal/monitor"
	"ai-ide-gateway/internal/proxy"
	"ai-ide-gateway/internal/quota"
	"ai-ide-gateway/internal/replay"
	"ai-ide-gateway/internal/router"
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

	// 初始化依赖组件
	// 1. 认证器
	authenticator := auth.NewAuthenticator(redisClient.GetClient(), log)

	// 2. 路由管理器
	routeManager := router.NewRouteManager(redisClient.GetClient(), log)
	if err := routeManager.LoadRules(context.Background()); err != nil {
		log.Warn("failed to load route rules", zap.Error(err))
	}
	if err := routeManager.SubscribeUpdates(context.Background()); err != nil {
		log.Warn("failed to subscribe route updates", zap.Error(err))
	}

	// 3. 熔断器
	circuitBreaker := breaker.NewCircuitBreaker(breaker.CircuitBreakerConfig{
		WindowSize:      cfg.CircuitBreaker.WindowSize,
		ErrorThreshold:  cfg.CircuitBreaker.ErrorThreshold,
		HalfOpenTimeout: cfg.CircuitBreaker.HalfOpenTimeout,
		MaxRequests:     10,
	}, log)

	// 4. 故障切换执行器
	failoverExecutor := router.NewFailoverExecutor(routeManager, circuitBreaker, log)

	// 启动健康探活（在后台 goroutine 中运行）
	healthProbe := router.NewHealthProbe(routeManager, cfg.CircuitBreaker.HalfOpenTimeout, log)
	go healthProbe.Start(context.Background())

	// 5. 协议适配器工厂
	adapterFactory := adapter.NewAdapterFactory()

	// 6. 限流器
	rateLimiter := breaker.NewRateLimiter(redisClient.GetClient(), log, breaker.RateLimiterConfig{
		UserCapacity:  cfg.RateLimit.User.Capacity,
		UserRate:      cfg.RateLimit.User.Rate,
		AppCapacity:   cfg.RateLimit.App.Capacity,
		AppRate:       cfg.RateLimit.App.Rate,
		ModelCapacity: cfg.RateLimit.Model.Capacity,
		ModelRate:     cfg.RateLimit.Model.Rate,
	})

	// 7. 配额管理器
	quotaManager := quota.NewQuotaManager(redisClient.GetClient(), log)

	// 8. 回放收集器
	replayCollector := replay.NewReplayCollector(cfg.Kafka.Brokers, cfg.Kafka.Topic, log)
	defer replayCollector.Close()

	// 9. 监控指标
	metrics := monitor.NewMetrics()

	// 10. SSE 流式代理（io.Pipe + goroutine，管理上游连接生命周期）
	streamProxy := proxy.NewStreamProxy(log)
	cleanupDone := make(chan struct{})
	go streamProxy.StartCleanup(cleanupDone)
	defer close(cleanupDone)

	// 11. ChatHandler
	chatHandler := handler.NewChatHandler(
		authenticator,
		routeManager,
		failoverExecutor,
		adapterFactory,
		rateLimiter,
		quotaManager,
		replayCollector,
		metrics,
		streamProxy,
		log,
	)

	// 注册 API 路由
	httpServer.RegisterRoute(http.MethodPost, "/v1/chat/completions", chatHandler.Handle)

	// 12. 流量回放服务
	replayService := replay.NewReplayService(cfg.Kafka.Brokers, cfg.Kafka.Topic, log)
	replayHandler := handler.NewReplayHandler(replayService, log)
	httpServer.RegisterRoute(http.MethodGet, "/v1/replay/history", replayHandler.ListHistory)
	httpServer.RegisterRoute(http.MethodPost, "/v1/replay/run", replayHandler.StartReplay)

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
