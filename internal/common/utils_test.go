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
	"testing"

	"github.com/shopspring/decimal"
)

// ============================================================================
// Normalization Tests
// ============================================================================

func TestNormalizeSide(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"uppercase buy", "BUY", "BUY"},
		{"lowercase buy", "buy", "BUY"},
		{"mixed case buy", "Buy", "Buy"}, // Returns unchanged
		{"uppercase sell", "SELL", "SELL"},
		{"lowercase sell", "sell", "SELL"},
		{"mixed case sell", "Sell", "Sell"},    // Returns unchanged
		{"invalid side", "invalid", "invalid"}, // Returns unchanged
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSide(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeSide(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeOrderType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"uppercase market", "MARKET", "MARKET"},
		{"lowercase market", "market", "MARKET"},
		{"mixed case market", "Market", "Market"}, // Returns unchanged
		{"uppercase limit", "LIMIT", "LIMIT"},
		{"lowercase limit", "limit", "LIMIT"},
		{"mixed case limit", "Limit", "Limit"}, // Returns unchanged
		{"uppercase twap", "TWAP", "TWAP"},     // Not handled, returns unchanged
		{"lowercase twap", "twap", "twap"},     // Not handled, returns unchanged
		{"invalid type", "invalid", "invalid"}, // Returns unchanged
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeOrderType(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeOrderType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Validation Tests - Order Requests
// ============================================================================

func TestValidateOrderRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     OrderRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid quote order",
			req: OrderRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				Type:       "MARKET",
				QuoteValue: decimal.NewFromFloat(10.0),
				Unit:       "quote",
			},
			wantErr: false,
		},
		{
			name: "valid base order",
			req: OrderRequest{
				Product: "BTC-USD",
				Side:    "SELL",
				Type:    "MARKET",
				BaseQty: decimal.NewFromFloat(0.001),
				Unit:    "base",
			},
			wantErr: false,
		},
		{
			name: "missing product",
			req: OrderRequest{
				Side:       "BUY",
				Type:       "MARKET",
				QuoteValue: decimal.NewFromFloat(10.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "product is required",
		},
		{
			name: "missing side",
			req: OrderRequest{
				Product:    "BTC-USD",
				Type:       "MARKET",
				QuoteValue: decimal.NewFromFloat(10.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "side must be BUY or SELL",
		},
		{
			name: "invalid side",
			req: OrderRequest{
				Product:    "BTC-USD",
				Side:       "HOLD",
				Type:       "MARKET",
				QuoteValue: decimal.NewFromFloat(10.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "side must be BUY or SELL",
		},
		{
			name: "zero quote value",
			req: OrderRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				Type:       "MARKET",
				QuoteValue: decimal.Zero,
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "quote value must be positive",
		},
		{
			name: "negative quote value",
			req: OrderRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				Type:       "MARKET",
				QuoteValue: decimal.NewFromFloat(-10.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "quote value must be positive",
		},
		{
			name: "zero base quantity",
			req: OrderRequest{
				Product: "BTC-USD",
				Side:    "SELL",
				Type:    "MARKET",
				BaseQty: decimal.Zero,
				Unit:    "base",
			},
			wantErr: true,
			errMsg:  "base quantity must be positive",
		},
		{
			name: "negative base quantity",
			req: OrderRequest{
				Product: "BTC-USD",
				Side:    "SELL",
				Type:    "MARKET",
				BaseQty: decimal.NewFromFloat(-0.001),
				Unit:    "base",
			},
			wantErr: true,
			errMsg:  "base quantity must be positive",
		},
		{
			name: "missing quantity specification",
			req: OrderRequest{
				Product: "BTC-USD",
				Side:    "BUY",
				Type:    "MARKET",
				Unit:    "",
			},
			wantErr: true,
			errMsg:  "quantity must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOrderRequest(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateOrderRequest() expected error containing %q, got nil", tt.errMsg)
				} else if err.Error() != tt.errMsg {
					t.Errorf("ValidateOrderRequest() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateOrderRequest() unexpected error: %v", err)
				}
			}
		})
	}
}

// ============================================================================
// Validation Tests - RFQ Requests
// ============================================================================

func TestValidateRfqRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     RfqRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid quote order",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: false,
		},
		{
			name: "valid base order",
			req: RfqRequest{
				Product:    "ETH-USD",
				Side:       "SELL",
				BaseQty:    decimal.NewFromFloat(0.5),
				LimitPrice: decimal.NewFromFloat(2500.0),
				Unit:       "base",
			},
			wantErr: false,
		},
		{
			name: "missing product",
			req: RfqRequest{
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "product is required",
		},
		{
			name: "invalid side",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "HOLD",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "side must be BUY or SELL",
		},
		{
			name: "missing limit price",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "limit price is required and must be positive",
		},
		{
			name: "zero limit price",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.Zero,
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "limit price is required and must be positive",
		},
		{
			name: "negative limit price",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(-43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "limit price is required and must be positive",
		},
		{
			name: "zero quote value",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.Zero,
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "quote value must be positive",
		},
		{
			name: "negative quote value",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(-100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "quote",
			},
			wantErr: true,
			errMsg:  "quote value must be positive",
		},
		{
			name: "zero base quantity",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "SELL",
				BaseQty:    decimal.Zero,
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "base",
			},
			wantErr: true,
			errMsg:  "base quantity must be positive",
		},
		{
			name: "negative base quantity",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "SELL",
				BaseQty:    decimal.NewFromFloat(-0.5),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "base",
			},
			wantErr: true,
			errMsg:  "base quantity must be positive",
		},
		{
			name: "invalid unit",
			req: RfqRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				QuoteValue: decimal.NewFromFloat(100.0),
				LimitPrice: decimal.NewFromFloat(43000.0),
				Unit:       "invalid",
			},
			wantErr: true,
			errMsg:  "unit must be 'base' or 'quote'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRfqRequest(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateRfqRequest() expected error containing %q, got nil", tt.errMsg)
				} else if err.Error() != tt.errMsg {
					t.Errorf("ValidateRfqRequest() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateRfqRequest() unexpected error: %v", err)
				}
			}
		})
	}
}

// ============================================================================
// Order Preparation Tests
// ============================================================================

func TestPrepareOrderRequest_QuoteOrders(t *testing.T) {
	// Setup: Create a percent fee strategy with 0.5% (50 bps)
	feeStrategy, err := CreateFeeStrategy("0.005") // 0.5%
	if err != nil {
		t.Fatalf("Failed to create fee strategy: %v", err)
	}

	priceAdjuster := &PriceAdjuster{
		FeeStrategy: feeStrategy,
	}

	tests := []struct {
		name                          string
		req                           OrderRequest
		portfolioId                   string
		generateClientOrderId         bool
		expectMetadata                bool
		expectedUserRequested         string
		expectedMarkup                string
		expectedPrimeOrderQuoteAmount string
	}{
		{
			name: "quote order - $10 with 0.5% fee",
			req: OrderRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				Type:       "MARKET",
				QuoteValue: decimal.NewFromFloat(10.0),
				Unit:       "quote",
			},
			portfolioId:                   "test-portfolio",
			generateClientOrderId:         false,
			expectMetadata:                true,
			expectedUserRequested:         "10",
			expectedMarkup:                "0.05", // 10 * 0.005
			expectedPrimeOrderQuoteAmount: "9.95", // 10 - 0.05
		},
		{
			name: "quote order - $100 with 0.5% fee",
			req: OrderRequest{
				Product:    "ETH-USD",
				Side:       "SELL",
				Type:       "MARKET",
				QuoteValue: decimal.NewFromFloat(100.0),
				Unit:       "quote",
			},
			portfolioId:                   "test-portfolio",
			generateClientOrderId:         true,
			expectMetadata:                true,
			expectedUserRequested:         "100",
			expectedMarkup:                "0.5",  // 100 * 0.005
			expectedPrimeOrderQuoteAmount: "99.5", // 100 - 0.5
		},
		{
			name: "quote order - $1000 with 0.5% fee",
			req: OrderRequest{
				Product:    "BTC-USD",
				Side:       "BUY",
				Type:       "LIMIT",
				QuoteValue: decimal.NewFromFloat(1000.0),
				Price:      decimal.NewFromFloat(50000.0),
				Unit:       "quote",
			},
			portfolioId:                   "test-portfolio",
			generateClientOrderId:         false,
			expectMetadata:                true,
			expectedUserRequested:         "1000",
			expectedMarkup:                "5",   // 1000 * 0.005
			expectedPrimeOrderQuoteAmount: "995", // 1000 - 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepared, err := PrepareOrderRequest(tt.req, tt.portfolioId, priceAdjuster, tt.generateClientOrderId)
			if err != nil {
				t.Fatalf("PrepareOrderRequest() error = %v", err)
			}

			// Check Prime request
			if prepared.PrimeRequest == nil {
				t.Fatal("PrimeRequest is nil")
			}
			if prepared.PrimeRequest.Order.PortfolioId != tt.portfolioId {
				t.Errorf("PortfolioId = %q, want %q", prepared.PrimeRequest.Order.PortfolioId, tt.portfolioId)
			}
			if prepared.PrimeRequest.Order.ProductId != tt.req.Product {
				t.Errorf("ProductId = %q, want %q", prepared.PrimeRequest.Order.ProductId, tt.req.Product)
			}
			if prepared.PrimeRequest.Order.Side != NormalizeSide(tt.req.Side) {
				t.Errorf("Side = %q, want %q", prepared.PrimeRequest.Order.Side, NormalizeSide(tt.req.Side))
			}
			if prepared.PrimeRequest.Order.Type != NormalizeOrderType(tt.req.Type) {
				t.Errorf("Type = %q, want %q", prepared.PrimeRequest.Order.Type, NormalizeOrderType(tt.req.Type))
			}

			// Check client order ID generation
			if tt.generateClientOrderId {
				if prepared.PrimeRequest.Order.ClientOrderId == "" {
					t.Error("ClientOrderId should be generated but is empty")
				}
			} else {
				if prepared.PrimeRequest.Order.ClientOrderId != "" {
					t.Errorf("ClientOrderId should be empty but got %q", prepared.PrimeRequest.Order.ClientOrderId)
				}
			}

			// Check quote value adjustment (Prime gets reduced amount)
			if prepared.PrimeRequest.Order.QuoteValue != tt.expectedPrimeOrderQuoteAmount {
				t.Errorf("QuoteValue = %q, want %q", prepared.PrimeRequest.Order.QuoteValue, tt.expectedPrimeOrderQuoteAmount)
			}

			// Check limit price for limit orders
			if !tt.req.Price.IsZero() {
				if prepared.PrimeRequest.Order.LimitPrice != tt.req.Price.String() {
					t.Errorf("LimitPrice = %q, want %q", prepared.PrimeRequest.Order.LimitPrice, tt.req.Price.String())
				}
			}

			// Check metadata
			if tt.expectMetadata {
				if prepared.Metadata == nil {
					t.Fatal("Metadata is nil")
				}
				if prepared.Metadata.UserRequestedAmount.String() != tt.expectedUserRequested {
					t.Errorf("UserRequestedAmount = %q, want %q", prepared.Metadata.UserRequestedAmount.String(), tt.expectedUserRequested)
				}
				if prepared.Metadata.MarkupAmount.String() != tt.expectedMarkup {
					t.Errorf("MarkupAmount = %q, want %q", prepared.Metadata.MarkupAmount.String(), tt.expectedMarkup)
				}
				if prepared.Metadata.PrimeOrderQuoteAmount.String() != tt.expectedPrimeOrderQuoteAmount {
					t.Errorf("PrimeOrderQuoteAmount = %q, want %q", prepared.Metadata.PrimeOrderQuoteAmount.String(), tt.expectedPrimeOrderQuoteAmount)
				}
			}
		})
	}
}

func TestPrepareOrderRequest_BaseOrders(t *testing.T) {
	// Base orders don't use the fee strategy, but we still need to provide one
	feeStrategy, _ := CreateFeeStrategy("0.005")
	priceAdjuster := &PriceAdjuster{
		FeeStrategy: feeStrategy,
	}

	tests := []struct {
		name               string
		req                OrderRequest
		expectedBaseQty    string
		shouldHaveMetadata bool
	}{
		{
			name: "base order - 0.001 BTC",
			req: OrderRequest{
				Product: "BTC-USD",
				Side:    "BUY",
				Type:    "MARKET",
				BaseQty: decimal.NewFromFloat(0.001),
				Unit:    "base",
			},
			expectedBaseQty:    "0.001",
			shouldHaveMetadata: false,
		},
		{
			name: "base order - 0.5 ETH",
			req: OrderRequest{
				Product: "ETH-USD",
				Side:    "SELL",
				Type:    "MARKET",
				BaseQty: decimal.NewFromFloat(0.5),
				Unit:    "base",
			},
			expectedBaseQty:    "0.5",
			shouldHaveMetadata: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepared, err := PrepareOrderRequest(tt.req, "test-portfolio", priceAdjuster, false)
			if err != nil {
				t.Fatalf("PrepareOrderRequest() error = %v", err)
			}

			// Check base quantity is passed through unchanged
			if prepared.PrimeRequest.Order.BaseQuantity != tt.expectedBaseQty {
				t.Errorf("BaseQuantity = %q, want %q", prepared.PrimeRequest.Order.BaseQuantity, tt.expectedBaseQty)
			}

			// Check no metadata for base orders
			if tt.shouldHaveMetadata {
				if prepared.Metadata == nil {
					t.Error("Expected metadata but got nil")
				}
			} else {
				if prepared.Metadata != nil {
					t.Error("Expected no metadata but got non-nil")
				}
			}
		})
	}
}

// ============================================================================
// Rounding Tests
// ============================================================================

func TestRoundPrice(t *testing.T) {
	tests := []struct {
		name     string
		input    decimal.Decimal
		expected string
	}{
		{"already 2 decimals", decimal.NewFromFloat(100.50), "100.5"},
		{"needs rounding up", decimal.NewFromFloat(100.556), "100.56"},
		{"needs rounding down", decimal.NewFromFloat(100.554), "100.55"},
		{"zero", decimal.Zero, "0"},
		{"whole number", decimal.NewFromInt(100), "100"},
		{"many decimals", decimal.NewFromFloat(99.999999), "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundPrice(tt.input)
			if result != tt.expected {
				t.Errorf("RoundPrice(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRoundQty(t *testing.T) {
	tests := []struct {
		name     string
		input    decimal.Decimal
		expected string
	}{
		{"8 decimals", decimal.NewFromFloat(0.12345678), "0.12345678"},
		{"needs rounding", decimal.NewFromFloat(0.123456789), "0.12345679"},
		{"zero", decimal.Zero, "0"},
		{"whole number", decimal.NewFromInt(1), "1"},
		{"many decimals", decimal.NewFromFloat(0.999999999), "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundQty(tt.input)
			if result != tt.expected {
				t.Errorf("RoundQty(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
