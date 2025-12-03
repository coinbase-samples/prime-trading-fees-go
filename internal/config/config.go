/**
 * Copyright 2025-present Coinbase Global, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds the complete application configuration
type Config struct {
	Prime      PrimeConfig
	MarketData MarketDataConfig
	Fees       FeesConfig
	Server     ServerConfig
	Database   DatabaseConfig
}

// PrimeConfig holds Coinbase Prime API credentials
type PrimeConfig struct {
	AccessKey        string
	Passphrase       string
	SigningKey       string
	Portfolio        string
	ServiceAccountId string
}

// String masks sensitive credentials when printing
func (p PrimeConfig) String() string {
	return fmt.Sprintf("PrimeConfig{Portfolio: %s, ServiceAccountId: %s, AccessKey: [REDACTED], Passphrase: [REDACTED], SigningKey: [REDACTED]}",
		p.Portfolio, p.ServiceAccountId)
}

// GoString masks sensitive credentials when using %#v format
func (p PrimeConfig) GoString() string {
	return p.String()
}

// MarketDataConfig holds market data settings
type MarketDataConfig struct {
	WebSocketUrl      string
	Products          []string
	MaxLevels         int
	ReconnectDelay    time.Duration
	InitialWaitTime   time.Duration
	DisplayUpdateRate time.Duration
}

// FeesConfig holds percentage-based fee configuration
type FeesConfig struct {
	Percent string // e.g., "0.002" for 20 bps (0.2%)
}

// ServerConfig holds server settings
type ServerConfig struct {
	LogLevel string
	LogJson  bool
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	Path string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Default configuration
	cfg := &Config{
		Prime: PrimeConfig{},
		MarketData: MarketDataConfig{
			WebSocketUrl:      "wss://ws-feed.prime.coinbase.com",
			Products:          []string{"BTC-USD"},
			MaxLevels:         10,
			ReconnectDelay:    5 * time.Second,
			InitialWaitTime:   2 * time.Second,
			DisplayUpdateRate: 5 * time.Second,
		},
		Fees: FeesConfig{
			Percent: "0.002", // 0.2% (20 bps)
		},
		Server: ServerConfig{
			LogLevel: "info",
			LogJson:  false,
		},
		Database: DatabaseConfig{
			Path: "orders.db",
		},
	}

	// Load from environment variables
	loadFromEnv(cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func loadFromEnv(cfg *Config) {
	// Prime credentials
	if v := os.Getenv("PRIME_ACCESS_KEY"); v != "" {
		cfg.Prime.AccessKey = v
	}
	if v := os.Getenv("PRIME_PASSPHRASE"); v != "" {
		cfg.Prime.Passphrase = v
	}
	if v := os.Getenv("PRIME_SIGNING_KEY"); v != "" {
		cfg.Prime.SigningKey = v
	}
	if v := os.Getenv("PRIME_PORTFOLIO"); v != "" {
		cfg.Prime.Portfolio = v
	}
	if v := os.Getenv("PRIME_SERVICE_ACCOUNT_ID"); v != "" {
		cfg.Prime.ServiceAccountId = v
	}

	// Market data
	if v := os.Getenv("MARKET_DATA_WEBSOCKET_URL"); v != "" {
		cfg.MarketData.WebSocketUrl = v
	}
	if v := os.Getenv("MARKET_DATA_PRODUCTS"); v != "" {
		cfg.MarketData.Products = strings.Split(v, ",")
	}
	if v := os.Getenv("MARKET_DATA_MAX_LEVELS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.MarketData.MaxLevels = i
		}
	}
	if v := os.Getenv("MARKET_DATA_RECONNECT_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MarketData.ReconnectDelay = d
		}
	}
	if v := os.Getenv("MARKET_DATA_INITIAL_WAIT_TIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MarketData.InitialWaitTime = d
		}
	}
	if v := os.Getenv("MARKET_DATA_DISPLAY_UPDATE_RATE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MarketData.DisplayUpdateRate = d
		}
	}

	// Fees (percentage only)
	if v := os.Getenv("FEE_PERCENT"); v != "" {
		cfg.Fees.Percent = v
	}

	// Server
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Server.LogLevel = v
	}
	if v := os.Getenv("LOG_JSON"); v != "" {
		cfg.Server.LogJson = v == "true"
	}

	// Database
	if v := os.Getenv("DATABASE_PATH"); v != "" {
		cfg.Database.Path = v
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate Prime config
	if c.Prime.AccessKey == "" {
		return fmt.Errorf("PRIME_ACCESS_KEY is required")
	}
	if c.Prime.Passphrase == "" {
		return fmt.Errorf("PRIME_PASSPHRASE is required")
	}
	if c.Prime.SigningKey == "" {
		return fmt.Errorf("PRIME_SIGNING_KEY is required")
	}
	if c.Prime.Portfolio == "" {
		return fmt.Errorf("PRIME_PORTFOLIO is required")
	}
	if c.Prime.ServiceAccountId == "" {
		return fmt.Errorf("PRIME_SERVICE_ACCOUNT_ID is required")
	}

	// Validate market data config
	if len(c.MarketData.Products) == 0 {
		return fmt.Errorf("at least one product is required")
	}

	// Validate fee config
	if err := c.Fees.Validate(); err != nil {
		return fmt.Errorf("fee config: %w", err)
	}

	return nil
}

// Validate checks if a fee config is valid
func (f *FeesConfig) Validate() error {
	if f.Percent == "" {
		return fmt.Errorf("FEE_PERCENT is required")
	}
	percent, err := decimal.NewFromString(f.Percent)
	if err != nil {
		return fmt.Errorf("invalid FEE_PERCENT: %w", err)
	}
	if percent.IsNegative() {
		return fmt.Errorf("FEE_PERCENT cannot be negative")
	}
	return nil
}

// SetupLogger initializes the global Zap logger with structured JSON format
func SetupLogger(level string, useJSON bool) {
	// Always use JSON structured logging with production config
	zapConfig := zap.NewProductionConfig()

	// Use ISO8601 timestamps instead of epoch
	zapConfig.EncoderConfig.TimeKey = "ts"
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Enable caller information (file:line)
	zapConfig.EncoderConfig.CallerKey = "caller"
	zapConfig.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// Set other encoder fields
	zapConfig.EncoderConfig.LevelKey = "level"
	zapConfig.EncoderConfig.MessageKey = "msg"
	zapConfig.EncoderConfig.StacktraceKey = "stacktrace"

	// Set log level
	switch level {
	case "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	// Build with caller skip to show correct file:line
	logger, err := zapConfig.Build(zap.AddCallerSkip(0))
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	zap.ReplaceGlobals(logger)
}
