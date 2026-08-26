package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/wallet-ledger/internal/config"
	"github.com/wallet-ledger/internal/database"
	"github.com/wallet-ledger/internal/logger"
	"github.com/wallet-ledger/internal/redis"
	"github.com/wallet-ledger/internal/server"
)

func main() {
	cfg := config.LoadConfig()
	logger.NewLogger(cfg.Env)

	slog.Info("Wallet-Ledger API Starting", "env", cfg.Env)

	// Initialize PostgreSQL
	ctx := context.Background()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize Redis
	rdb, err := redis.Connect(ctx, cfg.RedisURL)
	if err != nil {
		slog.Error("Failed to connect to redis", "error", err)
		// We can fail-open or exit. For critical API we should exit if cache is required.
		os.Exit(1)
	}
	defer rdb.Close()

	// TODO: Initialize Kafka

	srv := server.NewServer(cfg, db, rdb)
	srv.Start()
}
