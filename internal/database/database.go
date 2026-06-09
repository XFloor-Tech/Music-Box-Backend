package database

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
)

const healthTimeout = time.Second

// Config contains PostgreSQL connection pool settings.
type Config struct {
	Addr           string
	MaxConnections int32
}

// Service defines the database lifecycle contract used by the server layer.
type Service interface {
	Repository
	Health() map[string]string
	Close() error
}

// Repository defines the low-level query contract for domain repositories.
type Repository interface {
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) pgx.Row
}

type Postgres struct {
	pool *pgxpool.Pool
}

var _ Service = (*Postgres)(nil)

func ConfigFromViper() (Config, error) {
	maxConnections := viper.GetInt("database.max_connections")
	if maxConnections < 1 {
		return Config{}, errors.New("database.max_connections must be greater than 0")
	}

	if maxConnections > 2147483647 {
		return Config{}, errors.New("database.max_connections is too large")
	}

	return Config{
		Addr:           strings.TrimSpace(viper.GetString("database.addr")),
		MaxConnections: int32(maxConnections),
	}, nil
}

func New(ctx context.Context, cfg Config) (*Postgres, error) {
	if cfg.Addr == "" {
		return nil, errors.New("database.addr is required")
	}

	if cfg.MaxConnections < 1 {
		return nil, errors.New("database.max_connections must be greater than 0")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("parse database connection addr: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConnections

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Postgres{pool: pool}, nil
}

func (db *Postgres) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()

	stats := db.pool.Stat()
	health := map[string]string{
		"status":            "up",
		"max_connections":   strconv.Itoa(int(stats.MaxConns())),
		"total_connections": strconv.Itoa(int(stats.TotalConns())),
		"idle_connections":  strconv.Itoa(int(stats.IdleConns())),
	}

	if err := db.pool.Ping(ctx); err != nil {
		health["status"] = "down"
		health["error"] = err.Error()
	}

	return health
}

func (db *Postgres) Close() error {
	db.pool.Close()
	return nil
}

func (db *Postgres) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return db.pool.Exec(ctx, query, args...)
}

func (db *Postgres) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return db.pool.Query(ctx, query, args...)
}

func (db *Postgres) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return db.pool.QueryRow(ctx, query, args...)
}
