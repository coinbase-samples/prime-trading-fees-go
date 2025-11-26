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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coinbase-samples/prime-trading-fees-go/config"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/fees"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/marketdata"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var (
	symbols = flag.String("symbols", "BTC-USD,ETH-USD", "Comma-separated list of product symbols to stream")
)

func main() {
	flag.Parse()

	// Load .env file
	_ = godotenv.Load()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logger
	config.SetupLogger(cfg.Server.LogLevel, cfg.Server.LogJson)
	defer zap.L().Sync()

	// Parse symbols from command-line flag
	products := []string{}
	if *symbols != "" {
		products = strings.Split(*symbols, ",")
		for i := range products {
			products[i] = strings.TrimSpace(products[i])
		}
	}

	if len(products) == 0 {
		return fmt.Errorf("at least one product symbol is required")
	}

	fmt.Printf("Starting market data stream for %v\n", products)
	fmt.Printf("Display updates every 5 seconds. Press Ctrl+C to stop.\n\n")

	// Initialize components
	store := marketdata.NewOrderBookStore()

	// Create fee strategy
	feeStrategy, err := fees.CreateFeeStrategy(cfg.Fees.Percent)
	if err != nil {
		return fmt.Errorf("failed to create fee strategy: %w", err)
	}

	adjuster := fees.NewPriceAdjuster(feeStrategy)

	// Start market data feed
	wsConfig := marketdata.WebSocketConfig{
		Url:              cfg.MarketData.WebSocketUrl,
		AccessKey:        cfg.Prime.AccessKey,
		Passphrase:       cfg.Prime.Passphrase,
		SigningKey:       cfg.Prime.SigningKey,
		ServiceAccountId: cfg.Prime.ServiceAccountId,
		Portfolio:        cfg.Prime.Portfolio,
		Products:         products,
		MaxLevels:        cfg.MarketData.MaxLevels,
		ReconnectDelay:   cfg.MarketData.ReconnectDelay,
	}
	wsClient := marketdata.NewWebSocketClient(wsConfig, store)

	if err := wsClient.Start(); err != nil {
		return fmt.Errorf("failed to start market data: %w", err)
	}
	defer wsClient.Stop()

	// Wait a moment for initial snapshot
	time.Sleep(cfg.MarketData.InitialWaitTime)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Print updates periodically
	ticker := time.NewTicker(cfg.MarketData.DisplayUpdateRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Clear screen for cleaner display
			fmt.Print("\033[2J\033[H")

			hasData := false
			for _, product := range products {
				book, exists := store.Get(product)
				if !exists {
					continue
				}

				snapshot := book.Snapshot()
				if len(snapshot.Bids) == 0 || len(snapshot.Asks) == 0 {
					continue
				}

				hasData = true
				displayOrderBook(product, snapshot, adjuster)
			}

			if !hasData {
				fmt.Printf("Waiting for market data...\n")
				fmt.Printf("Last update check: %s\n", time.Now().Format("15:04:05"))
			}

		case <-sigChan:
			fmt.Printf("\nShutting down...\n")
			return nil
		}
	}
}

func displayOrderBook(product string, snapshot marketdata.OrderBookSnapshot, adjuster *fees.PriceAdjuster) {
	// Display header
	fmt.Printf("\n═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  %s Order Book @ %s\n", product, snapshot.UpdateTime.Format("15:04:05"))
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")

	// Determine how many levels to show (max 10)
	maxLevels := 10
	bidLevels := len(snapshot.Bids)
	askLevels := len(snapshot.Asks)
	if bidLevels > maxLevels {
		bidLevels = maxLevels
	}
	if askLevels > maxLevels {
		askLevels = maxLevels
	}

	// Show asks in reverse order (highest to lowest)
	fmt.Printf("  %-15s %-15s %-15s\n", "ASK SIZE", "ASK PRICE", "ADJ PRICE")
	fmt.Printf("  %-15s %-15s %-15s\n", "--------", "---------", "---------")
	for i := askLevels - 1; i >= 0; i-- {
		ask := snapshot.Asks[i]
		adjAsk := adjuster.AdjustAskPrice(ask.Price, decimal.NewFromInt(1))
		fmt.Printf("  %-15s %-15s %-15s\n",
			ask.Size.StringFixed(4),
			ask.Price.StringFixed(2),
			adjAsk.StringFixed(2))
	}

	// Show spread
	if len(snapshot.Bids) > 0 && len(snapshot.Asks) > 0 {
		bestBid := snapshot.Bids[0]
		bestAsk := snapshot.Asks[0]
		spread := bestAsk.Price.Sub(bestBid.Price)

		fmt.Printf("\n  %-15s %-15s\n", "", "SPREAD")
		fmt.Printf("  %-15s %-15s\n", "", "------")
		fmt.Printf("  %-15s %s\n\n", "", spread.StringFixed(2))
	}

	// Show bids
	fmt.Printf("  %-15s %-15s %-15s\n", "BID SIZE", "BID PRICE", "ADJ PRICE")
	fmt.Printf("  %-15s %-15s %-15s\n", "--------", "---------", "---------")
	for i := 0; i < bidLevels; i++ {
		bid := snapshot.Bids[i]
		adjBid := adjuster.AdjustBidPrice(bid.Price, decimal.NewFromInt(1))
		fmt.Printf("  %-15s %-15s %-15s\n",
			bid.Size.StringFixed(4),
			bid.Price.StringFixed(2),
			adjBid.StringFixed(2))
	}

	fmt.Printf("\n")
}
