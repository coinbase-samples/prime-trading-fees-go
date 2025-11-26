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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/coinbase-samples/prime-trading-fees-go/config"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/common"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/database"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/order"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var (
	// Order flags
	symbol    = flag.String("symbol", "", "Product symbol (e.g., BTC-USD)")
	side      = flag.String("side", "", "Order side: buy or sell")
	qty       = flag.String("qty", "", "Order quantity (interpreted based on --unit)")
	unit      = flag.String("unit", "", "Unit for quantity: 'base' (e.g., BTC) or 'quote' (e.g., USD). Defaults: buy=quote, sell=base")
	orderType = flag.String("type", "market", "Order type: market or limit")
	price     = flag.String("price", "", "Limit price (required for limit orders)")
	mode      = flag.String("mode", "execute", "Execution mode: 'preview' (simulate) or 'execute' (place actual order)")
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

// parsedFlags holds the validated and normalized command line flags
type parsedFlags struct {
	symbol     string
	side       string
	orderType  string
	unitType   string
	quantity   decimal.Decimal
	limitPrice decimal.Decimal
	isPreview  bool
}

func run() error {
	// Parse and validate command line flags
	flags, err := parseAndValidateFlags()
	if err != nil {
		return err
	}

	// Load configuration and setup
	cfg, adjuster, err := loadConfigAndSetup()
	if err != nil {
		return err
	}
	defer func(l *zap.Logger) {
		err := l.Sync()
		if err != nil {

		}
	}(zap.L())

	req := buildOrderRequest(flags)

	// Execute based on mode (preview or actual order)
	ctx := context.Background()
	if flags.isPreview {
		return executePreview(ctx, cfg, adjuster, req)
	}
	return executeOrder(ctx, cfg, adjuster, req, flags.unitType, flags.quantity)
}

func outputPreview(resp *common.OrderPreviewResponse) error {
	// Output as formatted JSON
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// parseAndValidateFlags parses and validates all command line flags
func parseAndValidateFlags() (*parsedFlags, error) {
	// Validate required flags
	if *symbol == "" {
		return nil, fmt.Errorf("--symbol is required")
	}
	if *side == "" {
		return nil, fmt.Errorf("--side is required (buy or sell)")
	}
	if *qty == "" {
		return nil, fmt.Errorf("--qty is required")
	}

	// Normalize and validate side
	sideUpper := common.NormalizeSide(*side)
	if sideUpper != "BUY" && sideUpper != "SELL" {
		return nil, fmt.Errorf("--side must be 'buy' or 'sell', got: %s", *side)
	}

	// Determine unit with smart defaults
	unitType := *unit
	if unitType == "" {
		// Smart defaults: buy in quote (USD), sell in base (BTC/ETH)
		if sideUpper == "BUY" {
			unitType = "quote"
		} else {
			unitType = "base"
		}
	}

	// Validate and normalize unit
	if unitType == "base" || unitType == "BASE" {
		unitType = "base"
	} else if unitType == "quote" || unitType == "QUOTE" {
		unitType = "quote"
	} else {
		return nil, fmt.Errorf("--unit must be 'base' or 'quote', got: %s", *unit)
	}

	// Normalize and validate order type
	typeUpper := common.NormalizeOrderType(*orderType)
	if typeUpper != "MARKET" && typeUpper != "LIMIT" {
		return nil, fmt.Errorf("--type must be 'market' or 'limit', got: %s", *orderType)
	}

	// Validate and normalize mode
	isPreview := false
	modeValue := *mode
	if modeValue == "preview" || modeValue == "PREVIEW" {
		isPreview = true
	} else if modeValue == "execute" || modeValue == "EXECUTE" {
		isPreview = false
	} else {
		return nil, fmt.Errorf("--mode must be 'preview' or 'execute', got: %s", *mode)
	}

	// Parse quantity
	quantity, err := decimal.NewFromString(*qty)
	if err != nil {
		return nil, fmt.Errorf("invalid quantity: %w", err)
	}

	// Parse limit price if provided
	var limitPrice decimal.Decimal
	if *price != "" {
		limitPrice, err = decimal.NewFromString(*price)
		if err != nil {
			return nil, fmt.Errorf("invalid price: %w", err)
		}
	}

	// Validate type/price combination
	if typeUpper == "LIMIT" && *price == "" {
		return nil, fmt.Errorf("--price is required for limit orders")
	}
	if typeUpper == "MARKET" && *price != "" {
		return nil, fmt.Errorf("--price should not be specified for market orders")
	}

	return &parsedFlags{
		symbol:     *symbol,
		side:       sideUpper,
		orderType:  typeUpper,
		unitType:   unitType,
		quantity:   quantity,
		limitPrice: limitPrice,
		isPreview:  isPreview,
	}, nil
}

// loadConfigAndSetup loads configuration and sets up dependencies
func loadConfigAndSetup() (*config.Config, *common.PriceAdjuster, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logger
	config.SetupLogger(cfg.Server.LogLevel, cfg.Server.LogJson)

	// Create fee strategy
	feeStrategy, err := common.CreateFeeStrategy(cfg.Fees.Percent)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create fee strategy: %w", err)
	}

	adjuster := common.NewPriceAdjuster(feeStrategy)
	return cfg, adjuster, nil
}

// buildOrderRequest constructs an OrderRequest from parsed flags
func buildOrderRequest(flags *parsedFlags) common.OrderRequest {
	req := common.OrderRequest{
		Product: flags.symbol,
		Side:    flags.side,
		Type:    flags.orderType,
		Price:   flags.limitPrice,
		Unit:    flags.unitType,
	}

	// Set quantity based on unit type
	if flags.unitType == "base" {
		req.BaseQty = flags.quantity
	} else {
		req.QuoteValue = flags.quantity
	}

	return req
}

// executePreview generates and displays an order preview
func executePreview(ctx context.Context, cfg *config.Config, adjuster *common.PriceAdjuster, req common.OrderRequest) error {
	orderService := order.NewOrderServiceWithPrime(cfg, adjuster, nil)

	response, err := orderService.GeneratePreview(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate preview: %w", err)
	}

	return outputPreview(response)
}

// executeOrder places an actual order and stores metadata
func executeOrder(ctx context.Context, cfg *config.Config, adjuster *common.PriceAdjuster, req common.OrderRequest, unitType string, quantity decimal.Decimal) error {
	orderService := order.NewOrderServiceWithPrime(cfg, adjuster, nil)

	response, err := orderService.PlaceOrder(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to place order: %w", err)
	}

	// Store metadata in database for websocket to pick up
	if unitType == "quote" && !quantity.IsZero() {
		if err := storeOrderMetadata(cfg, response, req, adjuster); err != nil {
			zap.L().Warn("Failed to store order metadata", zap.Error(err))
		}
	}

	// Display success message
	fmt.Printf("\n=== Order Submitted ===\n")
	fmt.Printf("Order Id: %s\n", response.OrderId)
	fmt.Printf("Client Order Id: %s\n", response.ClientOrderId)
	fmt.Printf("Product: %s | Side: %s | Type: %s\n\n", response.Product, response.Side, response.Type)
	fmt.Println("Order execution updates will be available via the orders websocket.")
	fmt.Printf("Order state will be tracked in: %s\n", cfg.Database.Path)
	fmt.Println("\nTo monitor orders in real-time, run:")
	fmt.Println("  go run cmd/orders-stream/main.go")

	return nil
}

func storeOrderMetadata(cfg *config.Config, response *common.OrderResponse, req common.OrderRequest, adjuster *common.PriceAdjuster) error {
	// Open database
	db, err := database.NewOrdersDb(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get fee strategy
	feeStrategy := adjuster.FeeStrategy

	userRequestedAmount := req.QuoteValue
	MarkupAmount := feeStrategy.ComputeFromNotional(req.QuoteValue)
	primeOrderAmount := req.QuoteValue.Sub(MarkupAmount)

	// Create preliminary order record
	orderRecord := &database.OrderRecord{
		OrderId:               response.OrderId,
		ClientOrderId:         response.ClientOrderId,
		ProductId:             response.Product,
		Side:                  response.Side,
		OrderType:             response.Type,
		Status:                "PENDING",
		UserRequestedAmount:   userRequestedAmount.String(),
		MarkupAmount:          MarkupAmount.String(),
		PrimeOrderQuoteAmount: primeOrderAmount.String(),
		FirstSeenAt:           time.Now(),
		LastUpdatedAt:         time.Now(),
	}

	// Insert preliminary record
	if err := db.UpsertOrder(orderRecord); err != nil {
		return fmt.Errorf("failed to upsert order metadata: %w", err)
	}

	zap.L().Info("Stored order metadata in database",
		zap.String("order_id", response.OrderId),
		zap.String("user_requested", userRequestedAmount.String()),
		zap.String("our_markup", MarkupAmount.String()),
		zap.String("prime_amount", primeOrderAmount.String()))

	return nil
}
