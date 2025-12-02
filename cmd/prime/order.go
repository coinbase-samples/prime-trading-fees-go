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
	"fmt"
	"time"

	"github.com/coinbase-samples/prime-trading-fees-go/internal/common"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/config"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/database"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/order"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	orderSymbol string
	orderSide   string
	orderQty    string
	orderUnit   string
	orderType   string
	orderPrice  string
	orderMode   string
)

var orderCmd = &cobra.Command{
	Use:   "order",
	Short: "Place a market or limit order",
	Long:  `Place a market or limit order on Coinbase Prime. Supports preview mode to simulate orders before execution.`,
	Example: `  # Preview a market buy order for $1000 of BTC
  prime order --symbol BTC-USD --side buy --qty 1000 --mode preview

  # Execute a market sell order for 0.5 BTC
  prime order --symbol BTC-USD --side sell --qty 0.5 --unit base --mode execute

  # Execute a limit buy at $50,000
  prime order --symbol BTC-USD --side buy --qty 1000 --type limit --price 50000 --mode execute`,
	RunE: runOrder,
}

func init() {
	orderCmd.Flags().StringVar(&orderSymbol, "symbol", "", "Product symbol (e.g., BTC-USD) [required]")
	orderCmd.Flags().StringVar(&orderSide, "side", "", "Order side: buy or sell [required]")
	orderCmd.Flags().StringVar(&orderQty, "qty", "", "Order quantity (interpreted based on --unit) [required]")
	orderCmd.Flags().StringVar(&orderUnit, "unit", "", "Unit for quantity: 'base' (e.g., BTC) or 'quote' (e.g., USD). Defaults: buy=quote, sell=base")
	orderCmd.Flags().StringVar(&orderType, "type", "market", "Order type: market or limit")
	orderCmd.Flags().StringVar(&orderPrice, "price", "", "Limit price (required for limit orders)")
	orderCmd.Flags().StringVar(&orderMode, "mode", "execute", "Execution mode: 'preview' (simulate) or 'execute' (place actual order)")

	orderCmd.MarkFlagRequired("symbol")
	orderCmd.MarkFlagRequired("side")
	orderCmd.MarkFlagRequired("qty")
}

// parsedOrderFlags holds the validated and normalized command line flags
type parsedOrderFlags struct {
	symbol     string
	side       string
	orderType  string
	unitType   string
	quantity   decimal.Decimal
	limitPrice decimal.Decimal
	isPreview  bool
}

func runOrder(cmd *cobra.Command, args []string) error {
	// Parse and validate command line flags
	flags, err := parseAndValidateOrderFlags()
	if err != nil {
		return err
	}

	// Load configuration and setup
	cfg, adjuster, err := loadOrderConfigAndSetup()
	if err != nil {
		return err
	}
	defer zap.L().Sync()

	req := buildOrderRequest(flags)

	// Execute based on mode (preview or actual order)
	ctx := context.Background()
	if flags.isPreview {
		return executePreview(ctx, cfg, adjuster, req)
	}
	return executeOrder(ctx, cfg, adjuster, req, flags.unitType, flags.quantity)
}

func parseAndValidateOrderFlags() (*parsedOrderFlags, error) {
	// Validate required flags (already handled by MarkFlagRequired, but double-check)
	if orderSymbol == "" {
		return nil, fmt.Errorf("--symbol is required")
	}
	if orderSide == "" {
		return nil, fmt.Errorf("--side is required (buy or sell)")
	}
	if orderQty == "" {
		return nil, fmt.Errorf("--qty is required")
	}

	// Normalize and validate side
	sideUpper := common.NormalizeSide(orderSide)
	if sideUpper != "BUY" && sideUpper != "SELL" {
		return nil, fmt.Errorf("--side must be 'buy' or 'sell', got: %s", orderSide)
	}

	// Determine unit with smart defaults
	unitType := orderUnit
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
		return nil, fmt.Errorf("--unit must be 'base' or 'quote', got: %s", orderUnit)
	}

	// Normalize and validate order type
	typeUpper := common.NormalizeOrderType(orderType)
	if typeUpper != "MARKET" && typeUpper != "LIMIT" {
		return nil, fmt.Errorf("--type must be 'market' or 'limit', got: %s", orderType)
	}

	// Validate and normalize mode
	isPreview := false
	modeValue := orderMode
	if modeValue == "preview" || modeValue == "PREVIEW" {
		isPreview = true
	} else if modeValue == "execute" || modeValue == "EXECUTE" {
		isPreview = false
	} else {
		return nil, fmt.Errorf("--mode must be 'preview' or 'execute', got: %s", orderMode)
	}

	// Parse quantity
	quantity, err := decimal.NewFromString(orderQty)
	if err != nil {
		return nil, fmt.Errorf("invalid quantity: %w", err)
	}

	// Parse limit price if provided
	var limitPrice decimal.Decimal
	if orderPrice != "" {
		limitPrice, err = decimal.NewFromString(orderPrice)
		if err != nil {
			return nil, fmt.Errorf("invalid price: %w", err)
		}
	}

	// Validate type/price combination
	if typeUpper == "LIMIT" && orderPrice == "" {
		return nil, fmt.Errorf("--price is required for limit orders")
	}
	if typeUpper == "MARKET" && orderPrice != "" {
		return nil, fmt.Errorf("--price should not be specified for market orders")
	}

	return &parsedOrderFlags{
		symbol:     orderSymbol,
		side:       sideUpper,
		orderType:  typeUpper,
		unitType:   unitType,
		quantity:   quantity,
		limitPrice: limitPrice,
		isPreview:  isPreview,
	}, nil
}

func loadOrderConfigAndSetup() (*config.Config, *common.PriceAdjuster, error) {
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

func buildOrderRequest(flags *parsedOrderFlags) common.OrderRequest {
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

func executePreview(ctx context.Context, cfg *config.Config, adjuster *common.PriceAdjuster, req common.OrderRequest) error {
	orderService := order.NewOrderServiceWithPrime(cfg, adjuster, nil)

	response, err := orderService.GeneratePreview(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate preview: %w", err)
	}

	return outputPreview(response)
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
	fmt.Println("  prime orders-stream")

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
