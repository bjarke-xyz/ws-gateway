package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bjarke-xyz/ws-gateway/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newLogger(service string) *slog.Logger {
	env := os.Getenv("ENV")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	child := logger.With(slog.Group("service_info", slog.String("env", env), slog.String("service", service)))
	return child
}

func newDatabasePool(ctx context.Context, maxConns int) (*pgxpool.Pool, error) {
	if maxConns == 0 {
		maxConns = 1
	}
	unformattedConnStr := os.Getenv("DATABASE_CONNECTION_POOL_URL")
	err := repository.Migrate("up", unformattedConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	queryChar := "?"
	if strings.Contains(unformattedConnStr, "?") {
		queryChar = "&"
	}
	url := fmt.Sprintf(
		"%s%vpool_max_conns=%d&pool_min_conns=%d",
		unformattedConnStr,
		queryChar,
		maxConns,
		2,
	)
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}

	// Setting the build statement cache to nil helps this work with pgbouncer
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Second
	return pgxpool.NewWithConfig(ctx, config)
}
