package proxy

import (
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Connection represents an active stream connection
type Connection struct {
	ID           string
	UpstreamResp *http.Response
	ClientWriter http.ResponseWriter
	CreatedAt    time.Time
	LastActivity time.Time
	Closed       bool
}

// ConnectionManager manages active connections
type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*Connection
	timeout     time.Duration
	logger      *zap.Logger
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(logger *zap.Logger) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*Connection),
		timeout:     5 * time.Minute,
		logger:      logger,
	}
}

// Register registers a new connection
func (cm *ConnectionManager) Register(sessionID string, conn *Connection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.connections[sessionID] = conn
	cm.logger.Debug("connection registered", zap.String("session", sessionID))
}

// Close closes a connection
func (cm *ConnectionManager) Close(sessionID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, ok := cm.connections[sessionID]
	if !ok {
		return
	}

	if !conn.Closed {
		if conn.UpstreamResp != nil && conn.UpstreamResp.Body != nil {
			conn.UpstreamResp.Body.Close()
		}
		conn.Closed = true
		cm.logger.Debug("connection closed", zap.String("session", sessionID))
	}

	delete(cm.connections, sessionID)
}

// StartCleanup starts the cleanup goroutine
func (cm *ConnectionManager) StartCleanup(ctx <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx:
			return
		case <-ticker.C:
			cm.cleanupStaleConnections()
		}
	}
}

// cleanupStaleConnections removes stale connections
func (cm *ConnectionManager) cleanupStaleConnections() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for sessionID, conn := range cm.connections {
		// Clean up timeout connections
		if now.Sub(conn.LastActivity) > cm.timeout {
			if !conn.Closed {
				if conn.UpstreamResp != nil && conn.UpstreamResp.Body != nil {
					conn.UpstreamResp.Body.Close()
				}
				conn.Closed = true
			}
			delete(cm.connections, sessionID)
			cm.logger.Warn("stale connection cleaned up",
				zap.String("session", sessionID),
				zap.Duration("idle", now.Sub(conn.LastActivity)))
		}
	}
}

// GetConnection gets a connection by session ID
func (cm *ConnectionManager) GetConnection(sessionID string) (*Connection, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conn, ok := cm.connections[sessionID]
	return conn, ok
}

// UpdateActivity updates the last activity time
func (cm *ConnectionManager) UpdateActivity(sessionID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conn, ok := cm.connections[sessionID]; ok {
		conn.LastActivity = time.Now()
	}
}

// GetConnectionCount returns the number of active connections
func (cm *ConnectionManager) GetConnectionCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return len(cm.connections)
}
