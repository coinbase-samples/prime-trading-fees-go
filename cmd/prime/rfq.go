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
	"strings"

	"github.com/coinbase-samples/prime-sdk-go/client"
	"github.com/coinbase-samples/prime-sdk-go/credentials"
	"github.com/coinbase-samples/prime-sdk-go/orders"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/common"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/config"
	"github.com/coinbase-samples/prime-trading-fees-go/internal/rfq"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	rfqSymbol     string
	rfqSide       string
	rfqQty        string
	rfqUnit       string
	rfqPrice      string
	rfqAutoAccept bool
)

var rfqCmd = &cobra.Command{
	Use:   "rfq",
	Short: "Request for Quote (RFQ) workflow",
	Long:  `Create and optionally accept a Request for Quote (RFQ) on Coinbase Prime. RFQs require a limit price.`,
	Example: `  # Create RFQ for buying $10,000 of BTC at $50,000 limit
  prime rfq --symbol BTC-USD --side buy --qty 10000 --price 50000

  # Create and auto-accept RFQ
  prime rfq --symbol BTC-USD --side buy --qty 10000 --price 50000 --auto-accept`,
	RunE: runRfq,
}

func init() {
	rfqCmd.Flags().StringVar(&rfqSymbol, "symbol", "", "Product symbol (e.g., BTC-USD) [required]")
	rfqCmd.Flags().StringVar(&rfqSide, "side", "", "Order side: buy or sell [required]")
	rfqCmd.Flags().StringVar(&rfqQty, "qty", "", "Order quantity (interpreted based on --unit) [required]")
	rfqCmd.Flags().StringVar(&rfqUnit, "unit", "", "Unit for quantity: 'base' (e.g., BTC) or 'quote' (e.g., USD). Defaults: buy=quote, sell=base")
	rfqCmd.Flags().StringVar(&rfqPrice, "price", "", "Limit price [required for RFQ]")
	rfqCmd.Flags().BoolVar(&rfqAutoAccept, "auto-accept", false, "Automatically accept the quote (default: false, just show quote)")

	rfqCmd.MarkFlagRequired("symbol")
	rfqCmd.MarkFlagRequired("side")
	rfqCmd.MarkFlagRequired("qty")
	rfqCmd.MarkFlagRequired("price")
}

type parsedRfqFlags struct {
	symbol     string
	side       string
	unitType   string
	quantity   decimal.Decimal
	limitPrice decimal.Decimal
	autoAccept bool
}

func runRfq(cmd *cobra.Command, args []string) error {
	// Parse and validate flags
	flags, err := parseAndValidateRfqFlags(rfqSymbol, rfqSide, rfqQty, rfqUnit, rfqPrice, rfqAutoAccept)
	if err != nil {
		return err
	}

	// Load configuration
	cfg, adjuster, primeClient, err := loadRfqConfigAndSetup()
	if err != nil {
		return err
	}
	defer zap.L().Sync()

	// Build RFQ request
	req := buildRfqRequest(flags)

	// Create RFQ service
	rfqService := rfq.NewRfqService(cfg, adjuster, primeClient)

	ctx := context.Background()

	// Create quote
	quoteResp, err := rfqService.CreateQuote(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create quote: %w", err)
	}

	// Display quote
	if err := outputQuote(quoteResp); err != nil {
		return err
	}

	// Auto-accept if flag is set
	if flags.autoAccept {
		fmt.Println("\n--- Auto-accepting quote ---")
		acceptResp, err := rfqService.AcceptQuote(ctx, common.AcceptRfqRequest{
			QuoteId: quoteResp.QuoteId,
			Product: quoteResp.Product,
			Side:    quoteResp.Side,
		})
		if err != nil {
			return fmt.Errorf("failed to accept quote: %w", err)
		}

		if err := outputAcceptResponse(acceptResp); err != nil {
			return err
		}
	} else {
		fmt.Printf("Note: Quote expires at %s\n", quoteResp.ExpirationTime)
	}

	return nil
}

func parseAndValidateRfqFlags(symbol, side, qty, unit, price string, autoAccept bool) (*parsedRfqFlags, error) {
	// Validate required flags
	if symbol == "" {
		return nil, fmt.Errorf("--symbol is required")
	}
	if side == "" {
		return nil, fmt.Errorf("--side is required (buy or sell)")
	}
	if qty == "" {
		return nil, fmt.Errorf("--qty is required")
	}
	if price == "" {
		return nil, fmt.Errorf("--price is required for RFQ (limit price)")
	}

	// Normalize and validate side
	sideUpper := common.NormalizeSide(side)
	if sideUpper != "BUY" && sideUpper != "SELL" {
		return nil, fmt.Errorf("--side must be 'buy' or 'sell', got: %s", side)
	}

	// Determine unit with smart defaults
	unitType := unit
	if unitType == "" {
		// Smart defaults: buy in quote (USD), sell in base (BTC/ETH)
		if sideUpper == "BUY" {
			unitType = "quote"
		} else {
			unitType = "base"
		}
	}

	// Validate and normalize unit
	if strings.EqualFold(unitType, "base") {
		unitType = "base"
	} else if strings.EqualFold(unitType, "quote") {
		unitType = "quote"
	} else {
		return nil, fmt.Errorf("--unit must be 'base' or 'quote', got: %s", unit)
	}

	// Parse quantity
	quantity, err := decimal.NewFromString(qty)
	if err != nil {
		return nil, fmt.Errorf("invalid quantity: %w", err)
	}

	// Parse limit price (required for RFQ)
	limitPrice, err := decimal.NewFromString(price)
	if err != nil {
		return nil, fmt.Errorf("invalid price: %w", err)
	}
	if limitPrice.IsZero() || limitPrice.IsNegative() {
		return nil, fmt.Errorf("--price must be positive")
	}

	return &parsedRfqFlags{
		symbol:     symbol,
		side:       sideUpper,
		unitType:   unitType,
		quantity:   quantity,
		limitPrice: limitPrice,
		autoAccept: autoAccept,
	}, nil
}

func loadRfqConfigAndSetup() (*config.Config, *common.PriceAdjuster, orders.OrdersService, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logger
	config.SetupLogger(cfg.Server.LogLevel, cfg.Server.LogJson)

	// Create fee strategy
	feeStrategy, err := common.CreateFeeStrategy(cfg.Fees.Percent)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create fee strategy: %w", err)
	}

	adjuster := common.NewPriceAdjuster(feeStrategy)

	// Create Prime client
	creds := &credentials.Credentials{
		AccessKey:    cfg.Prime.AccessKey,
		Passphrase:   cfg.Prime.Passphrase,
		SigningKey:   cfg.Prime.SigningKey,
		PortfolioId:  cfg.Prime.Portfolio,
		SvcAccountId: cfg.Prime.ServiceAccountId,
	}

	httpClient, _ := client.DefaultHttpClient()
	restClient := client.NewRestClient(creds, httpClient)
	primeClient := orders.NewOrdersService(restClient)

	return cfg, adjuster, primeClient, nil
}

func buildRfqRequest(flags *parsedRfqFlags) common.RfqRequest {
	req := common.RfqRequest{
		Product:    flags.symbol,
		Side:       flags.side,
		LimitPrice: flags.limitPrice,
		Unit:       flags.unitType,
	}

	// Set quantity based on unit type
	if flags.unitType == "base" {
		req.BaseQty = flags.quantity
	} else {
		req.QuoteValue = flags.quantity
	}

	return req
}

func outputQuote(resp *common.RfqResponse) error {
	// Output as formatted JSON
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println("\n=== RFQ Quote ===")
	fmt.Println(string(data))
	return nil
}

func outputAcceptResponse(resp *common.AcceptRfqResponse) error {
	fmt.Println("\n=== Quote Accepted ===")
	fmt.Printf("Order ID: %s\n", resp.OrderId)
	fmt.Printf("Quote ID: %s\n", resp.QuoteId)
	fmt.Printf("Client Order ID: %s\n", resp.ClientOrderId)
	fmt.Printf("Product: %s\n", resp.Product)
	fmt.Printf("Side: %s\n", resp.Side)
	return nil
}
