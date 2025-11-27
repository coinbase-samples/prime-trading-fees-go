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

package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/coinbase-samples/prime-trading-fees-go/internal/common"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/config"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/database"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/websocket"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	ordersStreamSymbols string
)

var ordersStreamCmd = &cobra.Command{
	Use:   "orders-stream",
	Short: "Stream live order updates",
	Long:  `Connects to Coinbase Prime WebSocket and monitors order execution updates, storing them in a local database.`,
	Example: `  prime orders-stream --symbols BTC-USD,ETH-USD
  prime orders-stream --symbols BTC-USD`,
	RunE: runOrdersStream,
}

func init() {
	ordersStreamCmd.Flags().StringVar(&ordersStreamSymbols, "symbols", "BTC-USD,ETH-USD", "Comma-separated list of product symbols to subscribe to")
}

func runOrdersStream(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logger
	config.SetupLogger(cfg.Server.LogLevel, cfg.Server.LogJson)
	defer zap.L().Sync()

	// Parse symbols
	productIds := []string{}
	if ordersStreamSymbols != "" {
		productIds = strings.Split(ordersStreamSymbols, ",")
		for i := range productIds {
			productIds[i] = strings.TrimSpace(productIds[i])
		}
	}

	zap.L().Info("Starting orders websocket client",
		zap.Strings("products", productIds),
		zap.String("portfolio", cfg.Prime.Portfolio),
		zap.String("database", cfg.Database.Path))

	// Open database
	db, err := database.NewOrdersDb(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	zap.L().Info("Database opened", zap.String("path", cfg.Database.Path))

	// Create fee strategy
	feeStrategy, err := common.CreateFeeStrategy(cfg.Fees.Percent)
	if err != nil {
		return fmt.Errorf("failed to create fee strategy: %w", err)
	}

	// Create price adjuster
	priceAdjuster := common.NewPriceAdjuster(feeStrategy)

	// Create metadata store (shared in-memory store for order metadata)
	metadataStore := websocket.NewMetadataStore()

	// Create database handler
	handler := websocket.NewDbOrderHandler(db, priceAdjuster, metadataStore)

	// Create orders websocket config
	wsConfig := websocket.OrdersConfig{
		CommonConfig: websocket.CommonConfig{
			Url:              cfg.MarketData.WebSocketUrl,
			AccessKey:        cfg.Prime.AccessKey,
			Passphrase:       cfg.Prime.Passphrase,
			SigningKey:       cfg.Prime.SigningKey,
			ServiceAccountId: cfg.Prime.ServiceAccountId,
			Products:         productIds,
			ReconnectDelay:   cfg.MarketData.ReconnectDelay,
		},
		PortfolioId: cfg.Prime.Portfolio,
	}

	// Create and start websocket client
	wsClient := websocket.NewOrdersClient(wsConfig, handler)
	if err := wsClient.Start(); err != nil {
		return fmt.Errorf("failed to start websocket client: %w", err)
	}

	zap.L().Info("Orders websocket client started. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	zap.L().Info("Shutting down orders websocket client...")
	wsClient.Stop()

	return nil
}
