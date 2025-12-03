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

package common

import (
	"fmt"
	"strings"

	"github.com/coinbase-samples/prime-sdk-go/model"
	"github.com/coinbase-samples/prime-sdk-go/orders"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ============================================================================
// Product Symbol Parsing
// ============================================================================

// GetQuoteCurrency extracts the quote currency from a product symbol
// Example: "BTC-USD" -> "USD", "ETH-BTC" -> "BTC"
func GetQuoteCurrency(productSymbol string) string {
	// Product format is BASE-QUOTE (e.g., BTC-USD, ETH-BTC)
	parts := strings.Split(productSymbol, "-")
	if len(parts) == 2 {
		return parts[1]
	}
	return "" // Unknown format
}

// GetQuotePrecision returns the decimal precision for a given quote currency
func GetQuotePrecision(quoteCurrency string) int32 {
	switch quoteCurrency {
	case "USD", "USDC", "USDT", "EUR", "GBP":
		return PrecisionUSD
	case "BTC":
		return PrecisionBTC
	case "ETH":
		return PrecisionETH
	default:
		return PrecisionDefault
	}
}

// GetProductQuotePrecision extracts quote currency and returns its precision
func GetProductQuotePrecision(productSymbol string) int32 {
	quoteCurrency := GetQuoteCurrency(productSymbol)
	return GetQuotePrecision(quoteCurrency)
}

// ============================================================================
// Normalization Functions
// ============================================================================

// NormalizeSide normalizes order side to uppercase
func NormalizeSide(side string) string {
	if side == "buy" {
		return "BUY"
	}
	if side == "sell" {
		return "SELL"
	}
	return side
}

// NormalizeOrderType normalizes order type to uppercase
func NormalizeOrderType(orderType string) string {
	if orderType == "market" {
		return "MARKET"
	}
	if orderType == "limit" {
		return "LIMIT"
	}
	return orderType
}

// ============================================================================
// Rounding Functions
// ============================================================================

// RoundPrice rounds a price to 2 decimal places for USD
func RoundPrice(d decimal.Decimal) string {
	return d.Round(2).String()
}

// RoundQty rounds a quantity to 8 decimal places for crypto
func RoundQty(d decimal.Decimal) string {
	return d.Round(8).String()
}

// ============================================================================
// Order Preparation Functions
// ============================================================================

// PrepareOrderRequest builds a Prime API order request from user input
// This consolidates the logic shared between preview and execution
func PrepareOrderRequest(
	req OrderRequest,
	portfolioId string,
	priceAdjuster *PriceAdjuster,
	generateClientOrderId bool,
) (*PreparedOrder, error) {
	// Normalize side and type
	normalizedSide := NormalizeSide(req.Side)
	normalizedType := NormalizeOrderType(req.Type)

	// Generate client order Id if needed (for actual orders, not previews)
	clientOrderId := ""
	if generateClientOrderId {
		clientOrderId = uuid.New().String()
	}

	// Build base Prime request
	primeReq := &orders.CreateOrderRequest{
		Order: &model.Order{
			PortfolioId:   portfolioId,
			ProductId:     req.Product,
			Side:          normalizedSide,
			Type:          normalizedType,
			ClientOrderId: clientOrderId,
		},
	}

	// Calculate metadata for quote-denominated orders
	var metadata *OrderMetadata

	if req.Unit == "quote" && !req.QuoteValue.IsZero() {
		// Calculate our markup fee on the requested notional amount
		// For a $10 order with 0.5% markup: fee = $10 * 0.005 = $0.05
		userRequestedAmount := req.QuoteValue

		// Get quote currency precision (2 for USD, 8 for BTC/ETH, etc.)
		precision := GetProductQuotePrecision(req.Product)

		// Round markup to appropriate precision for the quote currency
		// USD: 2 decimals, BTC: 8 decimals, ETH: 8 decimals
		markupAmount := priceAdjuster.FeeStrategy.ComputeFromNotional(req.QuoteValue).Round(precision)

		// Deduct our markup from the amount sent to Prime
		// User wants $10 worth, we send $9.95 to Prime, keep $0.05
		primeOrderAmount := req.QuoteValue.Sub(markupAmount)
		primeReq.Order.QuoteValue = primeOrderAmount.String()

		metadata = &OrderMetadata{
			UserRequestedAmount:   userRequestedAmount,
			MarkupAmount:          markupAmount,
			PrimeOrderQuoteAmount: primeOrderAmount,
		}
	} else if req.Unit == "base" && !req.BaseQty.IsZero() {
		primeReq.Order.BaseQuantity = req.BaseQty.String()
	}

	// Add price for limit orders
	if !req.Price.IsZero() {
		primeReq.Order.LimitPrice = req.Price.String()
	}

	return &PreparedOrder{
		PrimeRequest: primeReq,
		Metadata:     metadata,
		NormalizedReq: NormalizedOrderRequest{
			Product:       req.Product,
			Side:          normalizedSide,
			Type:          normalizedType,
			ClientOrderId: clientOrderId,
		},
	}, nil
}

// ============================================================================
// Validation Functions
// ============================================================================

// ValidateOrderRequest validates common order request parameters
func ValidateOrderRequest(req OrderRequest) error {
	if req.Product == "" {
		return fmt.Errorf("product is required")
	}

	side := req.Side
	if side != "BUY" && side != "SELL" && side != "buy" && side != "sell" {
		return fmt.Errorf("side must be BUY or SELL")
	}

	// Check quantity based on unit type
	if req.Unit == "quote" {
		if req.QuoteValue.IsZero() || req.QuoteValue.IsNegative() {
			return fmt.Errorf("quote value must be positive")
		}
	} else if req.Unit == "base" {
		if req.BaseQty.IsZero() || req.BaseQty.IsNegative() {
			return fmt.Errorf("base quantity must be positive")
		}
	} else {
		return fmt.Errorf("quantity must be specified")
	}

	return nil
}

// ValidateRfqRequest validates an RFQ request
func ValidateRfqRequest(req RfqRequest) error {
	if req.Product == "" {
		return fmt.Errorf("product is required")
	}

	if req.Side != "BUY" && req.Side != "SELL" {
		return fmt.Errorf("side must be BUY or SELL")
	}

	if req.LimitPrice.IsZero() || req.LimitPrice.IsNegative() {
		return fmt.Errorf("limit price is required and must be positive")
	}

	if req.Unit == "quote" {
		if req.QuoteValue.IsZero() || req.QuoteValue.IsNegative() {
			return fmt.Errorf("quote value must be positive")
		}
	} else if req.Unit == "base" {
		if req.BaseQty.IsZero() || req.BaseQty.IsNegative() {
			return fmt.Errorf("base quantity must be positive")
		}
	} else {
		return fmt.Errorf("unit must be 'base' or 'quote'")
	}

	return nil
}
