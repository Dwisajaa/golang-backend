package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/config"
	"github.com/Dwisajaa/golang-backend/internal/db"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadFromOS()
	if err != nil {
		logger.Error("config_load_failed", "err", err.Error())
		os.Exit(1)
	}

	pool, err := db.Open(cfg.Database)
	if err != nil {
		logger.Error("database_open_failed", "err", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	dir := "migrations"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	m := &db.Migrator{DB: pool, Dir: dir}
	applied, err := m.Migrate(ctx)
	if err != nil {
		logger.Error("migrate_failed", "err", err.Error())
		os.Exit(1)
	}
	if len(applied) == 0 {
		logger.Info("migrate_nothing_to_do")
	} else {
		for _, v := range applied {
			logger.Info("migrate_applied", "migration", v)
		}
	}
}
