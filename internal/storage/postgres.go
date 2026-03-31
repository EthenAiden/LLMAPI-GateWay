package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// PostgresClient PostgreSQL 数据库客户端包装
type PostgresClient struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewPostgresClient 创建新的 PostgreSQL 客户端
func NewPostgresClient(host string, port int, user, password, database string, maxOpenConns, maxIdleConns int, logger *zap.Logger) (*PostgresClient, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, database)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	logger.Info("connected to PostgreSQL", zap.String("host", host), zap.Int("port", port), zap.String("database", database))

	return &PostgresClient{
		db:     db,
		logger: logger,
	}, nil
}

// QueryRow 查询单行
func (pc *PostgresClient) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return pc.db.QueryRowContext(ctx, query, args...)
}

// Query 查询多行
func (pc *PostgresClient) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return pc.db.QueryContext(ctx, query, args...)
}

// Exec 执行语句
func (pc *PostgresClient) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return pc.db.ExecContext(ctx, query, args...)
}

// Begin 开始事务
func (pc *PostgresClient) Begin(ctx context.Context) (*sql.Tx, error) {
	return pc.db.BeginTx(ctx, nil)
}

// Close 关闭连接
func (pc *PostgresClient) Close() error {
	return pc.db.Close()
}

// HealthCheck 健康检查
func (pc *PostgresClient) HealthCheck(ctx context.Context) error {
	return pc.db.PingContext(ctx)
}

// GetDB 获取原始数据库连接
func (pc *PostgresClient) GetDB() *sql.DB {
	return pc.db
}

// CreateTable 创建表
func (pc *PostgresClient) CreateTable(ctx context.Context, tableName string, schema string) error {
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, schema)
	_, err := pc.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}
	return nil
}

// DropTable 删除表
func (pc *PostgresClient) DropTable(ctx context.Context, tableName string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
	_, err := pc.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop table %s: %w", tableName, err)
	}
	return nil
}
