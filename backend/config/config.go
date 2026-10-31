package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	// Adjust the import path to match the generated Prisma client location.
	db "mockify/backend/db"
)

// Config holds all runtime configuration values.
type Config struct {
	// Environment name (development, staging, production)
	Env string
	// Port on which the HTTP server should listen.
	Port string
	// Full PostgreSQL connection URL.
	DBUrl string
	// Secret used to sign and verify JWT tokens.
	JWTSecret string
	// Minimum log level.
	LogLevel zerolog.Level
	// Configured logger instance.
	Logger zerolog.Logger
}

// loadEnv loads a .env file if present. Errors are ignored because the
// application can also run without it (e.g., in containers where env vars are injected).
func loadEnv() {
	_ = godotenv.Load()
}

// getEnv returns the value of an environment variable or a fallback if not set.
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// getEnvInt parses an integer environment variable or returns the fallback.
func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

// parseLogLevel converts a string representation of a log level to zerolog.Level.
// If parsing fails, it defaults to InfoLevel.
func parseLogLevel(level string) zerolog.Level {
	if lvl, err := zerolog.ParseLevel(strings.ToLower(level)); err == nil {
		return lvl
	}
	return zerolog.InfoLevel
}

// New creates a Config instance, validates required fields and configures the global logger.
func New() (*Config, error) {
	loadEnv()

	cfg := &Config{
		Env:       getEnv("APP_ENV", "development"),
		Port:      getEnv("PORT", "8080"),
		DBUrl:     getEnv("DATABASE_URL", ""),
		JWTSecret: getEnv("JWT_SECRET", ""),
		LogLevel:  parseLogLevel(getEnv("LOG_LEVEL", "info")),
	}

	// Validate mandatory configuration.
	if cfg.DBUrl == "" {
		return nil, fmt.Errorf("environment variable DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("environment variable JWT_SECRET is required")
	}

	// Configure zerolog with a human‑readable console writer.
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}
	logger := zerolog.New(consoleWriter).
		Level(cfg.LogLevel).
		With().
		Timestamp().
		Str("env", cfg.Env).
		Logger()

	cfg.Logger = logger
	// Override the package‑level logger for convenience.
	log.Logger = logger

	return cfg, nil
}

// NewDBClient creates and connects a Prisma client based on the supplied Config.
// The caller is responsible for calling client.Prisma.Disconnect() when the
// application shuts down.
func NewDBClient(cfg *Config) (*db.PrismaClient, error) {
	// The generated Prisma client reads the datasource URL from the
	// DATABASE_URL environment variable, so we ensure it is set.
	// This approach works both for the generated client and for any
	// custom options the client may expose.
	_ = os.Setenv("DATABASE_URL", cfg.DBUrl)

	client := db.NewClient()

	// Connect to the database.
	if err := client.Prisma.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Verify the connection is alive.
	if err := client.Prisma.Ping(); err != nil {
		_ = client.Prisma.Disconnect()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return client, nil
}