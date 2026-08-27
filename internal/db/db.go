package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/Dwisajaa/golang-backend/internal/config"
)

// Pool tuning: initial values chosen for a small API behind a single proxy.
// Values are NOT tuned yet — the benchmark phase will produce real numbers.
// Relationship to MySQL max_connections: this app's MaxOpenConns must stay
// well below the server's max_connections divided by 1 (single replica).
const (
	maxOpenConns    = 25
	maxIdleConns    = 25
	connMaxLifetime = 5 * time.Minute
	connMaxIdleTime = 5 * time.Minute
	pingTimeout     = 5 * time.Second
)

// buildDSN renders a go-sql-driver DSN from configuration. The password is
// part of the DSN by design (the driver needs it), but it is never logged.
func buildDSN(cfg config.DatabaseConfig) (string, error) {
	driverCfg := mysql.NewConfig()
	driverCfg.User = cfg.User
	driverCfg.Passwd = cfg.Password
	driverCfg.Net = "tcp"
	driverCfg.Addr = cfg.Host + ":" + strconv.Itoa(cfg.Port)
	driverCfg.DBName = cfg.Name
	driverCfg.ParseTime = true
	// Loc defaults to time.UTC in this driver; the DSN omits `loc=` for the
	// default value. Timestamps parsed from DECIMAL/TIMESTAMP columns are UTC.
	driverCfg.Params = map[string]string{"charset": "utf8mb4"}
	return driverCfg.FormatDSN(), nil
}

// Open builds the DSN, opens the pool, and proves connectivity with a ping.
// A failed ping is fatal: the caller must not start the server without it.
func Open(cfg config.DatabaseConfig) (*sql.DB, error) {
	dsn, err := buildDSN(cfg)
	if err != nil {
		return nil, fmt.Errorf("build dsn: %w", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}
