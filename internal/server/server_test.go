package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestNewServer(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	server := NewServer("localhost", 8080, logger, 30*time.Second, 30*time.Second)

	if server == nil {
		t.Errorf("NewServer() returned nil")
	}
	if server.addr != "localhost:8080" {
		t.Errorf("NewServer() addr = %s, want localhost:8080", server.addr)
	}
	if server.engine == nil {
		t.Errorf("NewServer() engine is nil")
	}
}

func TestRegisterHealthCheck(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	server := NewServer("localhost", 8080, logger, 30*time.Second, 30*time.Second)
	server.RegisterHealthCheck()

	// Test /health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health returned status %d, want %d", w.Code, http.StatusOK)
	}

	body, _ := io.ReadAll(w.Body)
	if string(body) != `{"status":"ok"}` {
		t.Errorf("GET /health returned body %s, want {\"status\":\"ok\"}", string(body))
	}

	// Test /ready endpoint
	req = httptest.NewRequest("GET", "/ready", nil)
	w = httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /ready returned status %d, want %d", w.Code, http.StatusOK)
	}

	body, _ = io.ReadAll(w.Body)
	if string(body) != `{"status":"ready"}` {
		t.Errorf("GET /ready returned body %s, want {\"status\":\"ready\"}", string(body))
	}
}

func TestRegisterRoute(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	server := NewServer("localhost", 8080, logger, 30*time.Second, 30*time.Second)

	// Register a test route
	server.RegisterRoute("GET", "/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// Test the route
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /test returned status %d, want %d", w.Code, http.StatusOK)
	}

	body, _ := io.ReadAll(w.Body)
	if string(body) != `{"message":"test"}` {
		t.Errorf("GET /test returned body %s, want {\"message\":\"test\"}", string(body))
	}
}

func TestRegisterRouteMultipleMethods(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	server := NewServer("localhost", 8080, logger, 30*time.Second, 30*time.Second)

	// Register routes for different methods
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		server.RegisterRoute(method, "/api/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"method": c.Request.Method})
		})
	}

	// Test each method
	for _, method := range methods {
		req := httptest.NewRequest(method, "/api/test", nil)
		w := httptest.NewRecorder()
		server.engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s /api/test returned status %d, want %d", method, w.Code, http.StatusOK)
		}
	}
}

func TestShutdown(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	server := NewServer("localhost", 0, logger, 30*time.Second, 30*time.Second)

	// Start the server
	err := server.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestGetEngine(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	server := NewServer("localhost", 8080, logger, 30*time.Second, 30*time.Second)
	engine := server.GetEngine()

	if engine == nil {
		t.Errorf("GetEngine() returned nil")
	}
	if engine != server.engine {
		t.Errorf("GetEngine() returned different engine")
	}
}

func TestServerWithDifferentPorts(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ports := []int{8080, 8081, 9090}

	for _, port := range ports {
		server := NewServer("localhost", port, logger, 30*time.Second, 30*time.Second)

		// Just verify the server was created with a non-empty address
		if server.addr == "" {
			t.Errorf("NewServer() addr is empty for port %d", port)
		}
	}
}
