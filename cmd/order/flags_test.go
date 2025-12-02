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
	"testing"

	"github.com/shopspring/decimal"
)

// Helper function to reset flags and set test values
func setupFlags(symbolVal, sideVal, qtyVal, unitVal, typeVal, priceVal, modeVal string) {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	symbol = flag.String("symbol", symbolVal, "")
	side = flag.String("side", sideVal, "")
	qty = flag.String("qty", qtyVal, "")
	unit = flag.String("unit", unitVal, "")
	orderType = flag.String("type", typeVal, "")
	price = flag.String("price", priceVal, "")
	mode = flag.String("mode", modeVal, "")
}

func TestParseAndValidateFlags_ValidMarketBuy(t *testing.T) {
	setupFlags("BTC-USD", "buy", "1000", "", "market", "", "execute")

	result, err := parseAndValidateFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.symbol != "BTC-USD" {
		t.Errorf("expected symbol BTC-USD, got %s", result.symbol)
	}
	if result.side != "BUY" {
		t.Errorf("expected side BUY, got %s", result.side)
	}
	if result.orderType != "MARKET" {
		t.Errorf("expected orderType MARKET, got %s", result.orderType)
	}
	if result.unitType != "quote" {
		t.Errorf("expected unitType quote (smart default for buy), got %s", result.unitType)
	}
	if !result.quantity.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("expected quantity 1000, got %s", result.quantity)
	}
	if result.isPreview {
		t.Error("expected isPreview false for execute mode")
	}
}

func TestParseAndValidateFlags_ValidMarketSell(t *testing.T) {
	setupFlags("BTC-USD", "sell", "0.5", "", "market", "", "execute")

	result, err := parseAndValidateFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.side != "SELL" {
		t.Errorf("expected side SELL, got %s", result.side)
	}
	if result.unitType != "base" {
		t.Errorf("expected unitType base (smart default for sell), got %s", result.unitType)
	}
	if !result.quantity.Equal(decimal.NewFromFloat(0.5)) {
		t.Errorf("expected quantity 0.5, got %s", result.quantity)
	}
}

func TestParseAndValidateFlags_ValidLimitOrder(t *testing.T) {
	setupFlags("ETH-USD", "buy", "1000", "quote", "limit", "3000", "execute")

	result, err := parseAndValidateFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.orderType != "LIMIT" {
		t.Errorf("expected orderType LIMIT, got %s", result.orderType)
	}
	if !result.limitPrice.Equal(decimal.NewFromInt(3000)) {
		t.Errorf("expected limitPrice 3000, got %s", result.limitPrice)
	}
}

func TestParseAndValidateFlags_PreviewMode(t *testing.T) {
	setupFlags("BTC-USD", "buy", "1000", "quote", "market", "", "preview")

	result, err := parseAndValidateFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.isPreview {
		t.Error("expected isPreview true for preview mode")
	}
}

func TestParseAndValidateFlags_CaseInsensitiveInputs(t *testing.T) {
	tests := []struct {
		name         string
		side         string
		unit         string
		orderType    string
		mode         string
		price        string
		expectedSide string
		expectedUnit string
		expectedType string
	}{
		{"uppercase unit and mode", "buy", "QUOTE", "market", "EXECUTE", "", "BUY", "quote", "MARKET"},
		{"lowercase everything", "sell", "base", "limit", "preview", "50000", "SELL", "base", "LIMIT"},
		{"mixed case unit and mode", "buy", "QuOtE", "market", "PrEvIeW", "", "BUY", "quote", "MARKET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupFlags("BTC-USD", tt.side, "100", tt.unit, tt.orderType, tt.price, tt.mode)

			result, err := parseAndValidateFlags()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.side != tt.expectedSide {
				t.Errorf("expected side %s, got %s", tt.expectedSide, result.side)
			}
			if result.unitType != tt.expectedUnit {
				t.Errorf("expected unitType %s, got %s", tt.expectedUnit, result.unitType)
			}
			if result.orderType != tt.expectedType {
				t.Errorf("expected orderType %s, got %s", tt.expectedType, result.orderType)
			}
		})
	}
}

func TestParseAndValidateFlags_ExplicitUnitOverridesDefault(t *testing.T) {
	setupFlags("BTC-USD", "buy", "0.1", "base", "market", "", "execute")

	result, err := parseAndValidateFlags()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.unitType != "base" {
		t.Errorf("expected explicit unit 'base' to override smart default, got %s", result.unitType)
	}
}

func TestParseAndValidateFlags_MissingRequiredSymbol(t *testing.T) {
	setupFlags("", "buy", "1000", "", "market", "", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for missing symbol")
	}
	if err.Error() != "--symbol is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseAndValidateFlags_MissingRequiredSide(t *testing.T) {
	setupFlags("BTC-USD", "", "1000", "", "market", "", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for missing side")
	}
	if err.Error() != "--side is required (buy or sell)" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseAndValidateFlags_MissingRequiredQty(t *testing.T) {
	setupFlags("BTC-USD", "buy", "", "", "market", "", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for missing qty")
	}
	if err.Error() != "--qty is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseAndValidateFlags_InvalidSide(t *testing.T) {
	setupFlags("BTC-USD", "invalid", "1000", "", "market", "", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for invalid side")
	}
}

func TestParseAndValidateFlags_InvalidUnit(t *testing.T) {
	setupFlags("BTC-USD", "buy", "1000", "invalid", "market", "", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for invalid unit")
	}
}

func TestParseAndValidateFlags_InvalidOrderType(t *testing.T) {
	setupFlags("BTC-USD", "buy", "1000", "", "invalid", "", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for invalid order type")
	}
}

func TestParseAndValidateFlags_InvalidMode(t *testing.T) {
	setupFlags("BTC-USD", "buy", "1000", "", "market", "", "invalid")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestParseAndValidateFlags_InvalidQuantity(t *testing.T) {
	setupFlags("BTC-USD", "buy", "not-a-number", "", "market", "", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for invalid quantity")
	}
}

func TestParseAndValidateFlags_InvalidPrice(t *testing.T) {
	setupFlags("BTC-USD", "buy", "1000", "", "limit", "not-a-number", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for invalid price")
	}
}

func TestParseAndValidateFlags_LimitOrderMissingPrice(t *testing.T) {
	setupFlags("BTC-USD", "buy", "1000", "", "limit", "", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for limit order without price")
	}
	if err.Error() != "--price is required for limit orders" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseAndValidateFlags_MarketOrderWithPrice(t *testing.T) {
	setupFlags("BTC-USD", "buy", "1000", "", "market", "50000", "execute")

	_, err := parseAndValidateFlags()
	if err == nil {
		t.Fatal("expected error for market order with price")
	}
	if err.Error() != "--price should not be specified for market orders" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseAndValidateFlags_SmartDefaults(t *testing.T) {
	tests := []struct {
		name         string
		side         string
		expectedUnit string
	}{
		{"buy defaults to quote", "buy", "quote"},
		{"sell defaults to base", "sell", "base"},
		{"BUY uppercase defaults to quote", "BUY", "quote"},
		{"SELL uppercase defaults to base", "SELL", "base"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupFlags("BTC-USD", tt.side, "100", "", "market", "", "execute")

			result, err := parseAndValidateFlags()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.unitType != tt.expectedUnit {
				t.Errorf("expected smart default unit %s for side %s, got %s", tt.expectedUnit, tt.side, result.unitType)
			}
		})
	}
}
