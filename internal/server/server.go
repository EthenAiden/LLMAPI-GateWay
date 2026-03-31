package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Server HTTP 服务器
type Server struct {
	engine   *gin.Engine
	httpSrv  *http.Server
	logger   *zap.Logger
	addr     string
	shutdown chan struct{}
}

// NewServer 创建新的 HTTP 服务器
func NewServer(host string, port int, logger *zap.Logger, readTimeout, writeTimeout time.Duration) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// 添加日志中间件
	engine.Use(gin.Recovery())

	addr := fmt.Sprintf("%s:%d", host, port)

	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	return &Server{
		engine:   engine,
		httpSrv:  httpSrv,
		logger:   logger,
		addr:     addr,
		shutdown: make(chan struct{}),
	}
}

// RegisterHealthCheck 注册健康检查端点
func (s *Server) RegisterHealthCheck() {
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	s.engine.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	})

	// Prometheus metrics endpoint
	s.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// RegisterRoute 注册路由
func (s *Server) RegisterRoute(method, path string, handler gin.HandlerFunc) {
	switch method {
	case http.MethodGet:
		s.engine.GET(path, handler)
	case http.MethodPost:
		s.engine.POST(path, handler)
	case http.MethodPut:
		s.engine.PUT(path, handler)
	case http.MethodDelete:
		s.engine.DELETE(path, handler)
	case http.MethodPatch:
		s.engine.PATCH(path, handler)
	default:
		s.logger.Warn("unsupported HTTP method", zap.String("method", method))
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", zap.String("addr", s.addr))

	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("server error", zap.Error(err))
		}
	}()

	return nil
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")

	// 设置关闭超时
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("server shutdown error", zap.Error(err))
		return err
	}

	s.logger.Info("HTTP server stopped")
	return nil
}

// GetEngine 获取 Gin 引擎
func (s *Server) GetEngine() *gin.Engine {
	return s.engine
}
