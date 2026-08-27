package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Dwisajaa/golang-backend/internal/auth"
	"github.com/Dwisajaa/golang-backend/internal/config"
	"github.com/Dwisajaa/golang-backend/internal/db"
	"github.com/Dwisajaa/golang-backend/internal/httphandler"
	"github.com/Dwisajaa/golang-backend/internal/middleware"
	"github.com/Dwisajaa/golang-backend/internal/repository"
	"github.com/Dwisajaa/golang-backend/internal/router"
	"github.com/Dwisajaa/golang-backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.LoadFromOS()
	if err != nil {
		logger.Error("config_load_failed", "err", err.Error())
		os.Exit(1)
	}
	logger.Info("config_loaded", "app_env", cfg.App.Env, "app_port", cfg.App.Port)

	pool, err := db.Open(cfg.Database)
	if err != nil {
		logger.Error("database_open_failed", "err", err.Error())
		os.Exit(1)
	}
	logger.Info("database_connected",
		"host", cfg.Database.Host,
		"port", cfg.Database.Port,
		"name", cfg.Database.Name,
	) // password deliberately absent

	health := httphandler.NewHealthHandler()
	ready := httphandler.NewReadyHandler(pool)

	userRepo := repository.NewMySQLUserRepository(pool)
	userService := service.NewUserService(userRepo)
	users := httphandler.NewUserHandler(userService)

	tokenStore := repository.NewMySQLTokenStore(pool)
	tokenGen := auth.NewRandomTokenGenerator()
	authService := service.NewAuthService(userRepo, tokenStore, auth.NewBcryptHasher(), tokenGen)
	authHandler := httphandler.NewAuthHandler(authService)

	authMW := middleware.Auth(tokenStore, userRepo, tokenGen)

	engine := router.New(logger, health, ready, users, authHandler, authMW)

	addr := ":" + strconv.Itoa(cfg.App.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("server_start", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server_error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown_error", "err", err)
	}
	if err := pool.Close(); err != nil {
		logger.Error("database_close_error", "err", err)
	}
	logger.Info("server_stopped")
}
