package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"mockify/backend/config"
	"mockify/backend/router"
	"mockify/backend/prisma"
)

func main() {
	// -------------------------------------------------------------------------
	// Load configuration
	// -------------------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// -------------------------------------------------------------------------
	// Initialize logger (Zap production logger)
	// -------------------------------------------------------------------------
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create zap logger: %v", err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// -------------------------------------------------------------------------
	// Initialize Prisma client (Go Prisma)
	// -------------------------------------------------------------------------
	db, err := prisma.NewClient()
	if err != nil {
		logger.Fatal("failed to create Prisma client", zap.Error(err))
	}
	if err = db.Prisma.Connect(); err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	// Ensure graceful disconnect
	defer func() {
		if err := db.Prisma.Disconnect(); err != nil {
			logger.Error("error disconnecting from database", zap.Error(err))
		}
	}()

	// -------------------------------------------------------------------------
	// Echo instance & middleware
	// -------------------------------------------------------------------------
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Use Zap for request logging
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${time_rfc3339} ${remote_ip} ${method} ${uri} ${status}\n",
		Output: logger.Sugar(),
	}))
	e.Use(middleware.Recover())

	// -------------------------------------------------------------------------
	// Register application routes
	// -------------------------------------------------------------------------
	if err := router.RegisterRoutes(e, db, cfg); err != nil {
		logger.Fatal("failed to register routes", zap.Error(err))
	}

	// -------------------------------------------------------------------------
	// Start HTTP server (non‑blocking) and wait for termination signal
	// -------------------------------------------------------------------------
	go func() {
		addr := cfg.ServerAddress()
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			logger.Fatal("unexpected server error", zap.Error(err))
		}
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutdown signal received, terminating...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server stopped gracefully")
}